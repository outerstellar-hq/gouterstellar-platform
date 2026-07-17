package starforge

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientListsWorkersAndKeepsCredentialServerSide(t *testing.T) {
	t.Parallel()

	credential := "server-secret-credential"
	workerID := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/workers", r.URL.Path)
		assert.Equal(t, "Bearer "+credential, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"workers":[{"id":"` + workerID + `","display_name":"Mac mini","operator_label":"Nova","state":"online","os":"darwin","architecture":"arm64","agent_version":"1.2.3","last_seen_at":"2026-07-16T12:00:00Z","connection_id":"session-1","server_instance_id":"control-1","connected_at":"2026-07-16T11:00:00Z","last_heartbeat_at":"2026-07-16T12:00:00Z"}]}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, credential, server.Client())
	require.NoError(t, err)

	workers, err := client.ListWorkers(context.Background())
	require.NoError(t, err)
	require.Len(t, workers, 1)
	assert.Equal(t, "Nova", workers[0].OperatorLabel)
	assert.Equal(t, "session-1", workers[0].Heartbeat.SessionID)
	assert.Equal(t, "control-1", workers[0].Heartbeat.ServerID)
	require.NotNil(t, workers[0].LastSeenAt)
	assert.Equal(t, "2026-07-16T12:00:00Z", workers[0].LastSeenAt.Format(time.RFC3339))
}

func TestHTTPClientRejectsMalformedWorkerResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"workers":[{"id":"not-a-uuid","state":"online"}]}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "credential", server.Client())
	require.NoError(t, err)

	_, err = client.ListWorkers(context.Background())
	assert.ErrorIs(t, err, ErrMalformedResponse)
}

func TestHTTPClientListsSleepCatalogInDeterministicOrder(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/sleep/catalog", r.URL.Path)
		_, _ = w.Write([]byte(`{"stories":[
			{"id":"story-b","title":"Second story","order":2,"episodes":[
				{"id":"episode-b2","title":"Later","order":2,"status":"queued","stages":{"upload":{"status":"queued"},"text":{"status":"complete"}},"artifacts":[{"label":"Expired preview","url":"https://starline.invalid/old.mp4","state":"expired","expires_at":"2026-07-16T12:00:00Z"}]},
				{"id":"episode-b1","title":"Earlier","order":1,"status":"complete","publication_metadata":{"title":"Copy title","description":"Copy body"},"stages":{"voice":{"status":"running"},"text":{"status":"complete"}},"artifacts":[{"label":"Durable master","url":"https://starline.invalid/master.mp4","state":"durable"}]}
			]},
			{"id":"story-a","title":"First story","order":1,"episodes":[]}
		]}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "credential", server.Client())
	require.NoError(t, err)

	catalog, err := client.SleepCatalog(context.Background())
	require.NoError(t, err)
	require.Len(t, catalog.Stories, 2)
	assert.Equal(t, "First story", catalog.Stories[0].Title)
	assert.Equal(t, "Second story", catalog.Stories[1].Title)
	require.Len(t, catalog.Stories[1].Episodes, 2)
	assert.Equal(t, "Earlier", catalog.Stories[1].Episodes[0].Title)
	assert.Equal(t, "Later", catalog.Stories[1].Episodes[1].Title)
	assert.Equal(t, fixedSleepStages, stageNames(catalog.Stories[1].Episodes[0].Stages))
	assert.Equal(t, "missing", catalog.Stories[1].Episodes[0].Stages[2].Status)
	assert.Equal(t, "Copy title", catalog.Stories[1].Episodes[0].PublicationMetadata.Title)
	assert.Equal(t, "durable", catalog.Stories[1].Episodes[0].Artifacts[0].State)
}

func TestHTTPClientRejectsMalformedSleepCatalog(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"stories":[{"id":"story","title":"Story","episodes":[{"id":"episode","title":"Episode","artifacts":[{"label":"unsafe","url":"javascript:alert(1)","state":"preview"}]}]}]}`))
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "credential", server.Client())
	require.NoError(t, err)

	_, err = client.SleepCatalog(context.Background())
	assert.ErrorIs(t, err, ErrMalformedResponse)
}

func TestHTTPClientErrorsNeverExposeCredentialOrResponseBody(t *testing.T) {
	t.Parallel()

	credential := "credential-that-must-stay-private"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream accidentally echoed "+credential, http.StatusInternalServerError)
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, credential, server.Client())
	require.NoError(t, err)

	_, err = client.ListWorkers(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.NotContains(t, err.Error(), credential)
	assert.NotContains(t, err.Error(), "accidentally echoed")
}

func TestUpdateWorkerLabelSupportsChangeAndClear(t *testing.T) {
	t.Parallel()

	workerID := uuid.NewString()
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/workers/"+workerID+"/label", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewHTTPClient(server.URL, "credential", server.Client())
	require.NoError(t, err)

	require.NoError(t, client.UpdateWorkerLabel(context.Background(), workerID, " Nova · Mac mini "))
	require.NoError(t, client.UpdateWorkerLabel(context.Background(), workerID, ""))
	assert.JSONEq(t, `{"label":"Nova · Mac mini"}`, bodies[0])
	assert.JSONEq(t, `{"label":""}`, bodies[1])
}

func TestValidateLabelRejectsOversizedAndControlCharacters(t *testing.T) {
	t.Parallel()

	_, err := ValidateLabel(strings.Repeat("a", 121))
	assert.ErrorIs(t, err, ErrInvalidLabel)
	_, err = ValidateLabel("line\nbreak")
	assert.ErrorIs(t, err, ErrInvalidLabel)
	label, err := ValidateLabel("  Nova  ")
	require.NoError(t, err)
	assert.Equal(t, "Nova", label)
}

func TestUnconfiguredHTTPClientReportsUnavailable(t *testing.T) {
	t.Parallel()

	client, err := NewHTTPClient("", "", nil)
	require.NoError(t, err)
	_, err = client.ListWorkers(context.Background())
	assert.True(t, errors.Is(err, ErrUnavailable))
}

func stageNames(stages []SleepStage) []string {
	names := make([]string, 0, len(stages))
	for _, stage := range stages {
		names = append(names, stage.Name)
	}
	return names
}
