package service

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

type WsClient struct {
	UserID string
	Conn   *websocket.Conn
}

type WsEventPublisher struct {
	mu      sync.RWMutex
	clients map[*WsClient]struct{}
}

func NewWsEventPublisher() *WsEventPublisher {
	return &WsEventPublisher{clients: make(map[*WsClient]struct{})}
}

func (p *WsEventPublisher) Register(client *WsClient) {
	p.mu.Lock()
	p.clients[client] = struct{}{}
	p.mu.Unlock()
}

func (p *WsEventPublisher) Unregister(client *WsClient) {
	p.mu.Lock()
	delete(p.clients, client)
	p.mu.Unlock()
	_ = client.Conn.Close()
}

func (p *WsEventPublisher) PublishRefresh(targetID string) {
	msg := "refresh:" + targetID
	p.mu.RLock()
	defer p.mu.RUnlock()
	for client := range p.clients {
		err := client.Conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			slog.Warn("WS write failed", "userID", client.UserID, "error", err)
		}
	}
}

var _ EventPublisher = (*WsEventPublisher)(nil)
