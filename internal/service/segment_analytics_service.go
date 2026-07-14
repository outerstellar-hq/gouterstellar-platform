package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// SegmentAnalyticsService implements AnalyticsService by batching events to the
// Segment HTTP API. It is non-blocking: Track enqueues the event onto a buffered
// channel and a dedicated worker goroutine performs the HTTP POST, so request
// handlers never wait on the network. Events dropped due to a full queue or a
// failed POST are logged but never propagated to the caller.
type SegmentAnalyticsService struct {
	writeKey  string
	endpoint  string
	client    *http.Client
	events    chan segmentEvent
	done      chan struct{}
}

type segmentEvent struct {
	event      string
	properties map[string]interface{}
	at         time.Time
}

const (
	// segmentQueueSize bounds the number of in-flight events held between Track
	// and the worker. When full, further events are dropped to protect the
	// request path from backpressure.
	segmentQueueSize = 256
	// segmentUserIdentify lets anonymous events flow without a known user id.
	segmentAnonymousID = "outerstellar-platform"
)

// NewSegmentAnalyticsService constructs a Segment-backed analytics service for
// the given write key. The background worker is started immediately; call
// Close to stop it cleanly during shutdown.
func NewSegmentAnalyticsService(writeKey string) *SegmentAnalyticsService {
	s := &SegmentAnalyticsService{
		writeKey: writeKey,
		endpoint: "https://api.segment.io/v1/batch",
		client:   &http.Client{Timeout: 10 * time.Second},
		events:   make(chan segmentEvent, segmentQueueSize),
		done:     make(chan struct{}),
	}
	go s.worker()
	return s
}

// Track enqueues an event for asynchronous delivery to Segment. It never blocks
// the caller for longer than it takes to push onto the buffered channel; if the
// channel is full the event is logged and dropped.
func (s *SegmentAnalyticsService) Track(ctx context.Context, event string, properties map[string]interface{}) {
	props := make(map[string]interface{}, len(properties))
	for k, v := range properties {
		props[k] = v
	}
	select {
	case s.events <- segmentEvent{event: event, properties: props, at: time.Now().UTC()}:
	default:
		slog.Warn("Segment analytics queue full, dropping event", "event", event)
	}
}

// Close stops the background worker, letting queued events drain. It is safe to
// call multiple times.
func (s *SegmentAnalyticsService) Close() {
	close(s.events)
	<-s.done
}

func (s *SegmentAnalyticsService) worker() {
	defer close(s.done)
	for ev := range s.events {
		if err := s.send(ev); err != nil {
			slog.Warn("Segment analytics send failed", "event", ev.event, "error", err)
		}
	}
}

func (s *SegmentAnalyticsService) send(ev segmentEvent) error {
	track := map[string]interface{}{
		"type":       "track",
		"event":      ev.event,
		"anonymousId": segmentAnonymousID,
		"timestamp":  ev.at.Format(time.RFC3339Nano),
	}
	if len(ev.properties) > 0 {
		track["properties"] = ev.properties
	}
	// Surface the authenticated user id (if the handler populated it under the
	// conventional "user_id" property) as the Segment userId for identity.
	if uid, ok := ev.properties["user_id"]; ok {
		track["userId"] = uid
	}

	payload := map[string]interface{}{
		"batch": []map[string]interface{}{track},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.writeKey, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil
	}
	return nil
}
