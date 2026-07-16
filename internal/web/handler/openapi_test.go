package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpecUsesJavaCompatibleCanonicalContracts(t *testing.T) {
	paths, ok := buildOpenAPISpec()["paths"].(map[string]interface{})
	require.True(t, ok)

	expected := map[string]string{
		"/auth/password":            "put",
		"/auth/reset-request":       "post",
		"/auth/reset-confirm":       "post",
		"/admin/users":              "get",
		"/admin/users/{id}/enabled": "put",
		"/admin/users/{id}/role":    "put",
		"/devices/register":         "delete",
		"/notifications/read-all":   "put",
		"/notifications/{id}/read":  "put",
	}
	for path, method := range expected {
		operations, exists := paths[path].(map[string]interface{})
		require.True(t, exists, path)
		assert.Contains(t, operations, method, path)
	}
}
