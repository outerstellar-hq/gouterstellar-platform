package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSummaryJSONMatchesJavaContract(t *testing.T) {
	payload, err := json.Marshal(UserSummary{
		ID: "user-id", Username: "alice", Email: "alice@example.com",
		Role: "ADMIN", Enabled: true, FailedLoginAttempts: 2,
	})

	require.NoError(t, err)
	assert.JSONEq(t, `{
		"id":"user-id",
		"username":"alice",
		"email":"alice@example.com",
		"role":"ADMIN",
		"enabled":true,
		"failedLoginAttempts":2,
		"lockedUntil":null
	}`, string(payload))
}
