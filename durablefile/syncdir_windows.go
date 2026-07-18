package durablefile

// atomic.ReplaceFile uses MoveFileEx with MOVEFILE_WRITE_THROUGH on Windows.
func syncDirectory(string) error {
	return nil
}
