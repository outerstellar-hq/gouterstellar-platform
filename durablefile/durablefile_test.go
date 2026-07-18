package durablefile_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/outerstellar-hq/gouterstellar-platform/durablefile"
)

func TestWriteCreatesParentsAndReplacesCompleteFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "private", "state.json")
	if err := durablefile.Write(path, []byte("old content with a long tail"), 0o640, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := durablefile.Write(path, []byte("new"), 0o600, 0o700); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("content = %q, want %q", content, "new")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("mode = %v, want %v", got, os.FileMode(0o600))
		}
	}
}

func TestWriteReaderPreservesDestinationAndCleansTemporaryFileOnReadError(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "state.json")
	if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}

	readError := errors.New("source failed")
	err := durablefile.WriteReader(path, &failingReader{err: readError}, 0o600, 0o700)
	if !errors.Is(err, readError) {
		t.Fatalf("error = %v, want wrapped %v", err, readError)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "previous" {
		t.Fatalf("content = %q, want previous content", content)
	}
	temporaryFiles, err := filepath.Glob(filepath.Join(directory, ".durable-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporaryFiles) != 0 {
		t.Fatalf("temporary files remain: %v", temporaryFiles)
	}
}

func TestConcurrentWritesExposeOneCompleteValue(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state")
	values := []string{
		strings.Repeat("a", 32*1024),
		strings.Repeat("b", 32*1024),
		strings.Repeat("c", 32*1024),
	}

	var writers sync.WaitGroup
	for _, value := range values {
		value := value
		writers.Add(1)
		go func() {
			defer writers.Done()
			if err := durablefile.Write(path, []byte(value), 0o600, 0o700); err != nil {
				t.Errorf("write: %v", err)
			}
		}()
	}
	writers.Wait()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range values {
		if string(content) == value {
			return
		}
	}
	t.Fatalf("result is not one complete input: length %d", len(content))
}

func TestReplaceFlushesAndMovesExistingFile(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	source := filepath.Join(directory, "source.tmp")
	destination := filepath.Join(directory, "nested", "destination")
	if err := os.WriteFile(source, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := durablefile.Replace(source, destination, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "replacement" {
		t.Fatalf("content = %q, want replacement", content)
	}
}

func TestWriteRejectsModeTypeBits(t *testing.T) {
	t.Parallel()

	err := durablefile.Write(filepath.Join(t.TempDir(), "state"), nil, os.ModeDir|0o700, 0o700)
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
}

type failingReader struct {
	sent bool
	err  error
}

func (reader *failingReader) Read(destination []byte) (int, error) {
	if reader.sent {
		return 0, reader.err
	}
	reader.sent = true
	return copy(destination, "partial"), nil
}
