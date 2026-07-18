package migration

import (
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4/database/stub"
)

func TestRunnerAppliesEmbeddedMigrationsAndAcceptsNoChange(t *testing.T) {
	files := fstest.MapFS{
		"migrations/000001_users.up.sql":   {Data: []byte("CREATE TABLE users (id int);")},
		"migrations/000001_users.down.sql": {Data: []byte("DROP TABLE users;")},
	}
	driver, err := (&stub.Stub{}).Open("stub://")
	if err != nil {
		t.Fatal(err)
	}
	runner, err := New(files, "migrations", "stub", driver)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	if err := runner.Up(); err != nil {
		t.Fatal(err)
	}
	if err := runner.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
}

func TestRunnerRejectsMissingDirectory(t *testing.T) {
	driver, err := (&stub.Stub{}).Open("stub://")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(fstest.MapFS{}, "missing", "stub", driver); err == nil {
		t.Fatal("missing migration directory was accepted")
	}
}
