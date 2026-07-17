package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestPublishRefreshBroadcastsBrowserWebSocketProtocol(t *testing.T) {
	publisher := NewWsEventPublisher()
	firstClient, firstServer := websocketPair(t)
	secondClient, secondServer := websocketPair(t)
	publisher.Register(&WsClient{Conn: firstServer})
	publisher.Register(&WsClient{Conn: secondServer})

	publisher.PublishRefresh(MessageListPanel)

	expected := `<div id="ws-updates" ws-subscribe aria-live="polite" hx-swap-oob="true">
    <div data-refresh-target="message-list-panel"></div>
</div>`
	for _, client := range []*websocket.Conn{firstClient, secondClient} {
		require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
		messageType, message, err := client.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, messageType)
		require.Equal(t, expected, string(message))
	}
}

func websocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConnection := make(chan *websocket.Conn, 1)
	serverError := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			serverError <- err
			return
		}
		serverConnection <- connection
	}))
	t.Cleanup(server.Close)

	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	select {
	case connection := <-serverConnection:
		t.Cleanup(func() { _ = connection.Close() })
		return client, connection
	case err := <-serverError:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server WebSocket")
	}
	return nil, nil
}
