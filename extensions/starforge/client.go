package starforge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrUnavailable       = errors.New("starforge service unavailable")
	ErrMalformedResponse = errors.New("starforge returned a malformed response")
	ErrInvalidLabel      = errors.New("worker label is invalid")
)

const maxUpstreamResponseBytes = 1 << 20

type Heartbeat struct {
	ServerID        string     `json:"serverId"`
	SessionID       string     `json:"sessionId"`
	ConnectedAt     *time.Time `json:"connectedAt"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
}

type Worker struct {
	UUID          string     `json:"uuid"`
	DisplayName   string     `json:"displayName"`
	OperatorLabel string     `json:"operatorLabel"`
	State         string     `json:"state"`
	OS            string     `json:"os"`
	Architecture  string     `json:"architecture"`
	AgentVersion  string     `json:"agentVersion"`
	LastSeenAt    *time.Time `json:"lastSeenAt"`
	Heartbeat     Heartbeat  `json:"heartbeat"`
}

// upstreamWorker is the wire contract exposed by StarlineZero's control plane.
// Keep this separate from Worker: the latter is the stable view model returned
// by this extension's BFF, while the control-plane API uses snake_case fields
// and reports session data as flat nullable columns.
type upstreamWorker struct {
	ID               string     `json:"id"`
	DisplayName      string     `json:"display_name"`
	OperatorLabel    string     `json:"operator_label"`
	State            string     `json:"state"`
	OS               string     `json:"os"`
	Architecture     string     `json:"architecture"`
	AgentVersion     string     `json:"agent_version"`
	LastSeenAt       *time.Time `json:"last_seen_at"`
	ConnectionID     string     `json:"connection_id"`
	ServerInstanceID string     `json:"server_instance_id"`
	ConnectedAt      *time.Time `json:"connected_at"`
	LastHeartbeatAt  *time.Time `json:"last_heartbeat_at"`
}

type Client interface {
	ListWorkers(context.Context) ([]Worker, error)
	UpdateWorkerLabel(context.Context, string, string) error
}

// HTTPClient talks only to the Starforge control-plane API. Its credential is
// kept in a private field and is never returned by errors or response models.
type HTTPClient struct {
	baseURL    *url.URL
	credential string
	httpClient *http.Client
}

func NewHTTPClient(rawBaseURL, credential string, client *http.Client) (*HTTPClient, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	clientCopy := *client
	if clientCopy.CheckRedirect == nil {
		clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}

	rawBaseURL = strings.TrimSpace(rawBaseURL)
	if rawBaseURL == "" {
		return &HTTPClient{credential: credential, httpClient: &clientCopy}, nil
	}
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" || baseURL.User != nil {
		return nil, fmt.Errorf("invalid Starforge base URL")
	}
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	return &HTTPClient{baseURL: baseURL, credential: credential, httpClient: &clientCopy}, nil
}

func (c *HTTPClient) ListWorkers(ctx context.Context) ([]Worker, error) {
	request, err := c.request(ctx, http.MethodGet, "/api/workers", nil)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request) // #nosec G704 -- destination is validated startup configuration; redirects are disabled
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrUnavailable
	}

	var payload struct {
		Workers []upstreamWorker `json:"workers"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxUpstreamResponseBytes+1))
	if err := decoder.Decode(&payload); err != nil {
		return nil, ErrMalformedResponse
	}
	if err := ensureJSONEOF(decoder); err != nil || payload.Workers == nil {
		return nil, ErrMalformedResponse
	}
	workers := make([]Worker, 0, len(payload.Workers))
	for _, worker := range payload.Workers {
		if _, err := uuid.Parse(worker.ID); err != nil || strings.TrimSpace(worker.State) == "" {
			return nil, ErrMalformedResponse
		}
		workers = append(workers, Worker{
			UUID:          worker.ID,
			DisplayName:   worker.DisplayName,
			OperatorLabel: worker.OperatorLabel,
			State:         worker.State,
			OS:            worker.OS,
			Architecture:  worker.Architecture,
			AgentVersion:  worker.AgentVersion,
			LastSeenAt:    worker.LastSeenAt,
			Heartbeat: Heartbeat{
				ServerID:        worker.ServerInstanceID,
				SessionID:       worker.ConnectionID,
				ConnectedAt:     worker.ConnectedAt,
				LastHeartbeatAt: worker.LastHeartbeatAt,
			},
		})
	}
	return workers, nil
}

func (c *HTTPClient) UpdateWorkerLabel(ctx context.Context, workerID, label string) error {
	if _, err := uuid.Parse(workerID); err != nil {
		return fmt.Errorf("worker UUID: %w", ErrInvalidLabel)
	}
	label, err := ValidateLabel(label)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"label": label})
	if err != nil {
		return ErrInvalidLabel
	}
	request, err := c.request(ctx, http.MethodPut, "/api/workers/"+url.PathEscape(workerID)+"/label", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request) // #nosec G704 -- destination is validated startup configuration; redirects are disabled
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUnavailable
	}
	return nil
}

func (c *HTTPClient) request(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	if c == nil || c.baseURL == nil || strings.TrimSpace(c.credential) == "" {
		return nil, ErrUnavailable
	}
	target := *c.baseURL
	target.Path += endpoint
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body) // #nosec G704 -- only fixed API paths are joined to validated startup configuration
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+c.credential)
	request.Header.Set("Accept", "application/json")
	return request, nil
}

func ValidateLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if !utf8.ValidString(label) || utf8.RuneCountInString(label) > 120 {
		return "", ErrInvalidLabel
	}
	for _, character := range label {
		if unicode.IsControl(character) {
			return "", ErrInvalidLabel
		}
	}
	return label, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrMalformedResponse
	}
	return nil
}
