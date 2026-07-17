package filter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/web"
)

func TestLoggingEchoesOrGeneratesRequestID(t *testing.T) {
	handler := Logging()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, w.Header().Get(RequestIDHeader), web.RequestIDFromContext(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))

	provided := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(RequestIDHeader, "test-correlation-id-42")
	handler.ServeHTTP(provided, request)
	assert.Equal(t, "test-correlation-id-42", provided.Header().Get(RequestIDHeader))

	generated := httptest.NewRecorder()
	handler.ServeHTTP(generated, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.NotEmpty(t, generated.Header().Get(RequestIDHeader))
}

func TestObservabilityResponseCapturePreservesWebSocketUpgrade(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	websocketHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade WebSocket: %v", err)
			return
		}
		_ = connection.Close()
	})
	server := httptest.NewServer(Metrics(prometheus.NewRegistry())(Logging()(websocketHandler)))
	t.Cleanup(server.Close)

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	require.NoError(t, connection.Close())
}
