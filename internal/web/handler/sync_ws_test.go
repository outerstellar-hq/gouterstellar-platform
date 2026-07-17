package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence/db"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/service"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
	extplatform "github.com/outerstellar-hq/gouterstellar-platform/platform"
)

type websocketSessionRepo struct {
	persistence.SessionRepository
	session db.PltSession
	err     error
}

func (r *websocketSessionRepo) FindByTokenHash(context.Context, string) (db.PltSession, error) {
	return r.session, r.err
}

type websocketUserRepo struct {
	persistence.UserRepository
	user db.PltUser
	err  error
}

func (r *websocketUserRepo) FindByID(context.Context, uuid.UUID) (db.PltUser, error) {
	return r.user, r.err
}

func TestSyncWebSocketRouteAuthenticatesInsideProtocol(t *testing.T) {
	ctx := extplatform.NewContributionContext("platform-core")
	require.NoError(t, NewSyncWebSocket(service.NewWsEventPublisher(), nil, nil, false).ContributeRoutes(ctx))
	require.Len(t, ctx.Routes.All(), 1)
	assert.Equal(t, extplatform.GroupPublicUI, ctx.Routes.All()[0].Group)
}

func TestSyncWebSocketRejectsMissingAndInvalidSessionsWith4401(t *testing.T) {
	tests := []struct {
		name     string
		cookie   string
		sessions persistence.SessionRepository
	}{
		{name: "missing cookie"},
		{name: "invalid cookie", cookie: "invalid", sessions: &websocketSessionRepo{err: pgx.ErrNoRows}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewSyncWebSocket(service.NewWsEventPublisher(), test.sessions, &websocketUserRepo{}, false)
			connection, ready := dialSyncWebSocket(t, handler, test.cookie)
			<-ready

			_, _, err := connection.ReadMessage()
			var closeError *websocket.CloseError
			require.ErrorAs(t, err, &closeError)
			assert.Equal(t, 4401, closeError.Code)
			assert.Equal(t, "Authentication required", closeError.Text)
		})
	}
}

func TestSyncWebSocketBroadcastsJavaRefreshProtocolToValidSession(t *testing.T) {
	userID := uuid.New()
	publisher := service.NewWsEventPublisher()
	handler := NewSyncWebSocket(
		publisher,
		&websocketSessionRepo{session: db.PltSession{
			UserID:    userID,
			ExpiresAt: pgtype.Timestamp{Time: time.Now().Add(time.Hour), Valid: true},
		}},
		&websocketUserRepo{user: db.PltUser{ID: userID, Enabled: true}},
		false,
	)
	connection, ready := dialSyncWebSocket(t, handler, "valid-session")
	<-ready

	publisher.PublishRefresh(service.MessageListPanel)

	require.NoError(t, connection.SetReadDeadline(time.Now().Add(time.Second)))
	messageType, message, err := connection.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Contains(t, string(message), `hx-on::load="htmx.trigger(document.body, 'refresh')"`)
	assert.Contains(t, string(message), `data-refresh-target="message-list-panel"`)
}

func dialSyncWebSocket(t *testing.T, handler *SyncWebSocket, token string) (*websocket.Conn, <-chan struct{}) {
	t.Helper()
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.Handle(w, r)
		close(ready)
	}))
	t.Cleanup(server.Close)

	header := http.Header{}
	if token != "" {
		header.Set("Cookie", web.SessionCookieName+"="+token)
	}
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), header)
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	return connection, ready
}
