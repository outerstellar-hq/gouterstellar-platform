package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
)

func TestSessionRealmPassesRawTokenToSharedLookup(t *testing.T) {
	const rawToken = "oss_raw_session_token"
	var received string
	realm := NewSessionRealm(func(_ context.Context, token string) model.SessionLookup {
		received = token
		return model.SessionExpired{}
	})

	result := realm.Authenticate(context.Background(), rawToken)

	assert.Equal(t, rawToken, received)
	assert.IsType(t, ExpiredResult{}, result)
}
