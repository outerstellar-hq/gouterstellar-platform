package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		wantErr  bool
	}{
		{
			name: "valid manifest",
			manifest: Manifest{
				ID:    "reports",
				Label: "Reports",
				Mode:  ExtensionHost,
				Ownership: RouteOwnership{
					UI: []string{"/reports"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty ID rejected",
			manifest: Manifest{
				ID:    "",
				Label: "No ID",
				Mode:  FullPlatform,
			},
			wantErr: true,
		},
		{
			name: "invalid mode rejected",
			manifest: Manifest{
				ID:   "x",
				Mode: PlatformMode("bogus"),
			},
			wantErr: true,
		},
		{
			name: "full platform with no ownership rejected",
			manifest: Manifest{
				ID:   "x",
				Mode: FullPlatform,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
