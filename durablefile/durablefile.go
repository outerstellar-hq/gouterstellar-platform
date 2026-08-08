// Package durablefile writes complete files without exposing partial content.
package durablefile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/natefinch/atomic"
)

type destinationLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*destinationLock
}

type destinationLock struct {
	mu   sync.Mutex
	refs int
}

var destinationLocks = destinationLockRegistry{locks: make(map[string]*destinationLock)}

// CommittedError reports a durability failure after the destination has
// already been replaced. The new destination is visible, but its survival
// across an immediate system crash is not guaranteed.
//
// Callers can distinguish this outcome with errors.As. Unwrap exposes the
// underlying sync failure to errors.Is and errors.As.
type CommittedError struct {
	err error
}

func (err *CommittedError) Error() string {
	return err.err.Error()
}

func (err *CommittedError) Unwrap() error {
	return err.err
}

type directorySync func(string) error

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
// If syncing a directory fails after replacement, the returned error is a
// *CommittedError: the new file is visible but its survival across an
// immediate system crash is not guaranteed.
func WriteReader(path string, reader io.Reader, fileMode, directoryMode os.FileMode) (err error) {
	return writeReader(path, reader, fileMode, directoryMode, syncDirectory)
}

func writeReader(path string, reader io.Reader, fileMode, directoryMode os.FileMode, syncDir directorySync) (err error) {
	if err := validate(path, fileMode, directoryMode); err != nil {
		return err
	}
	if reader == nil {
		return fmt.Errorf("source reader is required")
	}

	directory := filepath.Dir(path)
	createdDirectories, err := createParentDirectories(directory, directoryMode)
	if err != nil {
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

	return replacementTransaction{
		source:             temporaryPath,
		destination:        path,
		sourceDirectory:    directory,
		destinationDir:     directory,
		createdDirectories: createdDirectories,
		syncDir:            syncDir,
	}.commit()
}

// Replace flushes a closed source file and atomically replaces destination
// with it. Source and destination must be on the same filesystem. Missing
// destination directories are created with directoryMode. A directory sync
// failure after replacement is returned as a *CommittedError.
func Replace(source, destination string, directoryMode os.FileMode) error {
	return replace(source, destination, directoryMode, syncDirectory)
}

func replace(source, destination string, directoryMode os.FileMode, syncDir directorySync) error {
	if source == "" {
		return fmt.Errorf("source path is required")
	}
	if err := validatePathAndDirectoryMode(destination, directoryMode); err != nil {
		return err
	}

	directory := filepath.Dir(destination)
	createdDirectories, err := createParentDirectories(directory, directoryMode)
	if err != nil {
		return fmt.Errorf("create parent directory for %q: %w", destination, err)
	}

	// #nosec G304 -- Replace is a filesystem primitive; the caller explicitly owns source and destination.
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

	return replacementTransaction{
		source:             source,
		destination:        destination,
		sourceDirectory:    filepath.Dir(source),
		destinationDir:     directory,
		createdDirectories: createdDirectories,
		syncDir:            syncDir,
	}.commit()
}

type replacementTransaction struct {
	source             string
	destination        string
	sourceDirectory    string
	destinationDir     string
	createdDirectories []string
	syncDir            directorySync
}

func (transaction replacementTransaction) commit() error {
	release := lockDestination(transaction.destination)
	defer release()

	if err := replaceAtomically(transaction.source, transaction.destination); err != nil {
		return err
	}
	if err := transaction.syncDir(transaction.destinationDir); err != nil {
		return committed(fmt.Errorf("replacement of %q succeeded but syncing its directory failed: %w", transaction.destination, err))
	}
	if !sameDirectory(transaction.sourceDirectory, transaction.destinationDir) {
		if err := transaction.syncDir(transaction.sourceDirectory); err != nil {
			return committed(fmt.Errorf("replacement of %q succeeded but syncing source directory %q failed: %w", transaction.destination, transaction.sourceDirectory, err))
		}
	}
	if err := syncCreatedDirectoryParents(transaction.createdDirectories, transaction.syncDir); err != nil {
		return committed(fmt.Errorf("replacement of %q succeeded but syncing a created directory parent failed: %w", transaction.destination, err))
	}
	return nil
}

func createParentDirectories(directory string, mode os.FileMode) ([]string, error) {
	var missing []string
	for current := filepath.Clean(directory); ; current = filepath.Dir(current) {
		_, err := os.Stat(current)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		missing = append(missing, current)
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	if err := os.MkdirAll(directory, mode); err != nil {
		return nil, err
	}
	return missing, nil
}

func syncCreatedDirectoryParents(created []string, syncDir directorySync) error {
	synced := make(map[string]struct{}, len(created))
	for _, directory := range created {
		parent := filepath.Dir(directory)
		if _, ok := synced[parent]; ok {
			continue
		}
		if err := syncDir(parent); err != nil {
			return err
		}
		synced[parent] = struct{}{}
	}
	return nil
}

func committed(err error) error {
	return &CommittedError{err: err}
}

func sameDirectory(first, second string) bool {
	firstAbsolute, firstErr := filepath.Abs(first)
	secondAbsolute, secondErr := filepath.Abs(second)
	return firstErr == nil && secondErr == nil && filepath.Clean(firstAbsolute) == filepath.Clean(secondAbsolute)
}

func replaceAtomically(source, destination string) error {
	if err := atomic.ReplaceFile(source, destination); err != nil {
		return fmt.Errorf("replace %q with %q: %w", destination, source, err)
	}
	return nil
}

func lockDestination(path string) func() {
	key, err := filepath.Abs(path)
	if err != nil {
		key = filepath.Clean(path)
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}

	destinationLocks.mu.Lock()
	lock := destinationLocks.locks[key]
	if lock == nil {
		lock = &destinationLock{}
		destinationLocks.locks[key] = lock
	}
	lock.refs++
	destinationLocks.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		destinationLocks.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(destinationLocks.locks, key)
		}
		destinationLocks.mu.Unlock()
	}
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
