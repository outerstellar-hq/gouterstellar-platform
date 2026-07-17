package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

type SyncWebSocket struct {
	publisher   *service.WsEventPublisher
	sessionRepo persistence.SessionRepository
	userRepo    persistence.UserRepository
	sessionSec  bool
}

func NewSyncWebSocket(
	publisher *service.WsEventPublisher,
	sessionRepo persistence.SessionRepository,
	userRepo persistence.UserRepository,
	sessionSec bool,
) *SyncWebSocket {
	return &SyncWebSocket{
		publisher:   publisher,
		sessionRepo: sessionRepo,
		userRepo:    userRepo,
		sessionSec:  sessionSec,
	}
}

// ContributeRoutes registers the WebSocket sync route. Authentication happens
// after the upgrade so rejected peers receive the Java-compatible 4401 close.
func (h *SyncWebSocket) ContributeRoutes(ctx *extplatform.ContributionContext) error {
	ctx.Routes.Public(http.MethodGet, "/ws/sync", "WebSocket sync", http.HandlerFunc(h.Handle))
	return nil
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (h *SyncWebSocket) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WS upgrade failed", "error", err)
		return
	}

	if !h.authenticated(r) {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(4401, "Authentication required"),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
		return
	}

	client := &service.WsClient{Conn: conn}
	h.publisher.Register(client)

	// done coordinates shutdown of the read loop and the ping writer: when
	// the read loop exits (client disconnect or read error) it closes done,
	// which stops the ping writer too.
	done := make(chan struct{})

	go func() {
		defer h.publisher.Unregister(client)
		defer close(done)
		conn.SetReadLimit(512)
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()

	// Ping writer: sends a WebSocket ping every 30s to keep the connection
	// alive through proxies/load balancers and to detect dead peers. Stops
	// when the read loop exits (done closed) or a write fails.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()
}

func (h *SyncWebSocket) authenticated(r *http.Request) bool {
	token := web.GetSessionToken(r)
	if token == "" {
		return false
	}

	hash := sha256.Sum256([]byte(token))
	session, err := h.sessionRepo.FindByTokenHash(r.Context(), hex.EncodeToString(hash[:]))
	if err != nil || session.ExpiresAt.Time.Before(time.Now()) {
		return false
	}
	user, err := h.userRepo.FindByID(r.Context(), session.UserID)
	return err == nil && user.Enabled
}
