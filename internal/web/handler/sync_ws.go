package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/rygel/gouterstellar-platform/internal/persistence"
	"github.com/rygel/gouterstellar-platform/internal/service"
	"github.com/rygel/gouterstellar-platform/internal/web"
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

func (h *SyncWebSocket) RegisterRoutes(r chi.Router) {
	r.Get("/ws/sync", h.Handle)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *SyncWebSocket) Handle(w http.ResponseWriter, r *http.Request) {
	token := web.GetSessionToken(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	session, err := h.sessionRepo.FindByTokenHash(r.Context(), tokenHash)
	if err != nil || session.ExpiresAt.Time.Before(time.Now()) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	pltUser, err := h.userRepo.FindByID(r.Context(), session.UserID)
	if err != nil || !pltUser.Enabled {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WS upgrade failed", "error", err)
		return
	}

	client := &service.WsClient{
		UserID: session.UserID.String(),
		Conn:   conn,
	}
	h.publisher.Register(client)

	go func() {
		defer h.publisher.Unregister(client)
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
}
