package durablefile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReaderReportsCommittedSyncFailure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state")
	syncFailure := errors.New("sync failed")
	err := writeReader(path, strings.NewReader("new state"), 0o600, 0o700, func(string) error {
		return syncFailure
	})

	var committedError *CommittedError
	if !errors.As(err, &committedError) {
		t.Fatalf("error = %v, want *CommittedError", err)
	}
	if !errors.Is(err, syncFailure) {
		t.Fatalf("error = %v, want wrapped %v", err, syncFailure)
	}
	if got := committedError.Error(); !strings.Contains(got, "replacement") || !strings.Contains(got, syncFailure.Error()) {
		t.Fatalf("committed error text = %q", got)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "new state" {
		t.Fatalf("content = %q, want new state", content)
	}
}

func TestReplaceReportsCommittedSyncFailure(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	destination := filepath.Join(directory, "nested", "destination")
	if err := os.WriteFile(source, []byte("new state"), 0o600); err != nil {
		t.Fatal(err)
	}
	syncFailure := errors.New("sync failed")
	err := replace(source, destination, 0o700, func(string) error {
		return syncFailure
	})

	var committedError *CommittedError
	if !errors.As(err, &committedError) {
		t.Fatalf("error = %v, want *CommittedError", err)
	}
	if !errors.Is(err, syncFailure) {
		t.Fatalf("error = %v, want wrapped %v", err, syncFailure)
	}
	content, readErr := os.ReadFile(destination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "new state" {
		t.Fatalf("content = %q, want new state", content)
	}
}
