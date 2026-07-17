package filter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/security"
)

type expiredRealm struct{}

func (expiredRealm) Name() string { return "session" }

func (expiredRealm) Authenticate(context.Context, string) security.AuthResult {
	return security.ExpiredResult{}
}

func TestBearerAuthSignalsExpiredSession(t *testing.T) {
	metrics := NewAuthMetrics(prometheus.NewRegistry())
	handler := BearerAuth(metrics, expiredRealm{})(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	request.Header.Set("Authorization", "Bearer oss_expired")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, "true", recorder.Header().Get(SessionExpiredHeader))
	assert.Contains(t, recorder.Body.String(), "expired")
}
