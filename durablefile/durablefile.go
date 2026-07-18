// Package durablefile writes complete files without exposing partial content.
package durablefile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/natefinch/atomic"
)

var replaceMutex sync.Mutex

// Write writes data to path using a same-directory temporary file. It creates
// missing parent directories with directoryMode and makes the replacement
// visible only after the temporary file has been flushed and closed.
func Write(path string, data []byte, fileMode, directoryMode os.FileMode) error {
	return WriteReader(path, bytes.NewReader(data), fileMode, directoryMode)
}

// WriteReader writes all content from reader to path without exposing a
// partial file. A read, flush, close, or replacement failure leaves the
// previous destination unchanged.
//
// If syncing the parent directory fails after replacement, the returned error
// states that the new file is visible but its survival across an immediate
// system crash is not guaranteed.
func WriteReader(path string, reader io.Reader, fileMode, directoryMode os.FileMode) (err error) {
	if err := validate(path, fileMode, directoryMode); err != nil {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return fmt.Errorf("create parent directory for %q: %w", path, err)
	}

	temporary, err := os.CreateTemp(directory, ".durable-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file for %q: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if temporary != nil {
			_ = temporary.Close()
		}
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(fileMode); err != nil {
		return fmt.Errorf("set temporary file mode for %q: %w", path, err)
	}
	if _, err := io.Copy(temporary, reader); err != nil {
		return fmt.Errorf("write temporary file for %q: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("flush temporary file for %q: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file for %q: %w", path, err)
	}
	temporary = nil

	if err := replaceSynced(temporaryPath, path); err != nil {
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("replacement of %q succeeded but syncing its directory failed: %w", path, err)
	}
	return nil
}

// Replace flushes a closed source file and atomically replaces destination
// with it. Source and destination must be on the same filesystem. Missing
// destination directories are created with directoryMode.
func Replace(source, destination string, directoryMode os.FileMode) error {
	if source == "" {
		return fmt.Errorf("source path is required")
	}
	if err := validatePathAndDirectoryMode(destination, directoryMode); err != nil {
		return err
	}

	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, directoryMode); err != nil {
		return fmt.Errorf("create parent directory for %q: %w", destination, err)
	}

	file, err := os.OpenFile(source, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", source, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("flush source file %q: %w", source, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close source file %q: %w", source, err)
	}

	if err := replaceSynced(source, destination); err != nil {
		return err
	}
	if err := syncDirectory(directory); err != nil {
		return fmt.Errorf("replacement of %q succeeded but syncing its directory failed: %w", destination, err)
	}
	return nil
}

func replaceSynced(source, destination string) error {
	replaceMutex.Lock()
	defer replaceMutex.Unlock()

	if err := atomic.ReplaceFile(source, destination); err != nil {
		return fmt.Errorf("replace %q with %q: %w", destination, source, err)
	}
	return nil
}

func validate(path string, fileMode, directoryMode os.FileMode) error {
	if err := validatePathAndDirectoryMode(path, directoryMode); err != nil {
		return err
	}
	if fileMode != fileMode.Perm() {
		return fmt.Errorf("file mode %v contains non-permission bits", fileMode)
	}
	return nil
}

func validatePathAndDirectoryMode(path string, directoryMode os.FileMode) error {
	if path == "" || filepath.Base(path) == "." {
		return fmt.Errorf("destination file path is required")
	}
	if directoryMode != directoryMode.Perm() {
		return fmt.Errorf("directory mode %v contains non-permission bits", directoryMode)
	}
	return nil
}
