package reports

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

type stubMessageCounter struct{ count int64 }

func (s stubMessageCounter) CountMessages(ctx context.Context) (int64, error) {
	return s.count, nil
}

func TestReportsContract(t *testing.T) {
	ext := New(stubMessageCounter{count: 42})

	diag, err := extplatform.CheckExtension(ext, extplatform.TestHostContext())
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"GET /reports", "GET /api/v1/reports/summary"},
		diag.RoutePatterns(),
	)
	assert.Contains(t, diag.NavigationLabels(), "Reports")
}

func TestReportsManifest(t *testing.T) {
	ext := New(stubMessageCounter{})
	m := ext.Manifest()

	assert.Equal(t, "reports", m.ID)
	assert.Equal(t, extplatform.ExtensionHost, m.Mode)
	assert.NotEmpty(t, m.Ownership.UI)
	assert.NotEmpty(t, m.Ownership.API)
	require.Len(t, m.Migrations, 1)
	assert.Equal(t, "schema_migrations_reports", m.Migrations[0].Table)
}
