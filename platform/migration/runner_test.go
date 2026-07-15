package migration

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		filename string
		wantVer  int64
		wantErr  bool
	}{
		{"V001__initial_schema.sql", 1, false},
		{"V002__user_profiles.sql", 2, false},
		{"V010__add_indexes.sql", 10, false},
		{"V100__big_migration.sql", 100, false},
		{"V1__old_format.sql", 1, false},
		{"not_a_migration.sql", 0, true},
		{"V.sql", 0, true},
		{"README.md", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ver, err := parseVersion(tt.filename)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantVer, ver)
			}
		})
	}
}

func TestSortSetsOrder(t *testing.T) {
	sets := []setEntry{
		{extensionID: "zebra"},
		{extensionID: "reports"},
		{extensionID: "platform-core"},
	}
	sort.Sort(byExtensionID(sets))

	assert.Equal(t, "platform-core", sets[0].extensionID)
	assert.Equal(t, "reports", sets[1].extensionID)
	assert.Equal(t, "zebra", sets[2].extensionID)
}

func TestPendingMigrationsFiltersApplied(t *testing.T) {
	files := []migrationFile{
		{Version: 1, Filename: "V001__a.sql"},
		{Version: 2, Filename: "V002__b.sql"},
		{Version: 3, Filename: "V003__c.sql"},
	}
	applied := map[int64]bool{1: true, 2: true}

	pending := pendingFiles(files, applied)
	require.Len(t, pending, 1)
	assert.Equal(t, int64(3), pending[0].Version)
}
