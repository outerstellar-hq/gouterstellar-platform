package filter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

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
