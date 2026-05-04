package service

import (
	"log/slog"
	"sync"
	"time"
)

type circuitState int

const (
	circuitClosed   circuitState = iota
	circuitOpen
	circuitHalfOpen
)

type ResilientEmailService struct {
	delegate    EmailService
	mu          sync.Mutex
	state       circuitState
	failures    []time.Time
	openedAt    time.Time
	windowSize  int
	threshold   float64
	openTimeout time.Duration
}

func NewResilientEmailService(delegate EmailService) *ResilientEmailService {
	return &ResilientEmailService{
		delegate:    delegate,
		state:       circuitClosed,
		windowSize:  10,
		threshold:   0.5,
		openTimeout: 60 * time.Second,
	}
}

func (r *ResilientEmailService) Send(to, subject, body string) {
	r.mu.Lock()
	state := r.state
	r.mu.Unlock()

	switch state {
	case circuitOpen:
		slog.Warn("Circuit breaker open, dropping email", "to", to, "subject", subject)
		return
	case circuitHalfOpen:
		r.halfOpenSend(to, subject, body)
		return
	default:
		r.closedSend(to, subject, body)
	}
}

func (r *ResilientEmailService) closedSend(to, subject, body string) {
	r.delegate.Send(to, subject, body)
	r.recordFailure(false)
}

func (r *ResilientEmailService) halfOpenSend(to, subject, body string) {
	r.delegate.Send(to, subject, body)
	r.recordFailure(false)
}

func (r *ResilientEmailService) recordFailure(wasFailure bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if wasFailure {
		r.failures = append(r.failures, time.Now())
	}

	now := time.Now()
	cutoff := now.Add(-r.openTimeout)
	filtered := r.failures[:0]
	for _, t := range r.failures {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	r.failures = filtered

	if len(r.failures) >= r.windowSize {
		failureRate := float64(len(r.failures)) / float64(r.windowSize)
		if failureRate >= r.threshold {
			r.state = circuitOpen
			r.openedAt = now
			slog.Warn("Circuit breaker opened", "failureRate", failureRate)
		}
	}
}
