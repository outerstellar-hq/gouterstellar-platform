package migration

import (
	"testing"
	"testing/fstest"

	_ "github.com/golang-migrate/migrate/v4/database/stub"
)

func TestRunnerAppliesEmbeddedMigrationsAndAcceptsNoChange(t *testing.T) {
	files := fstest.MapFS{
		"migrations/000001_users.up.sql":   {Data: []byte("CREATE TABLE users (id int);")},
		"migrations/000001_users.down.sql": {Data: []byte("DROP TABLE users;")},
	}
	runner, err := New(files, "migrations", "stub://migration-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	if err := runner.Up(); err != nil {
		t.Fatal(err)
	}
	version, dirty, err := runner.Version()
	if err != nil || version != 1 || dirty {
		t.Fatalf("version=%d dirty=%v err=%v", version, dirty, err)
	}
	if err := runner.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
}

func TestRunnerRejectsMissingDirectory(t *testing.T) {
	if _, err := New(fstest.MapFS{}, "missing", "stub://migration-test"); err == nil {
		t.Fatal("missing migration directory was accepted")
	}
}

func TestRunnerRequiresDatabaseURL(t *testing.T) {
	files := fstest.MapFS{
		"migrations/000001_users.up.sql":   {Data: []byte("CREATE TABLE users (id int);")},
		"migrations/000001_users.down.sql": {Data: []byte("DROP TABLE users;")},
	}
	if _, err := New(files, "migrations", ""); err == nil {
		t.Fatal("empty database URL was accepted")
	}
}
