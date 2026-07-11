package migration

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	extplatform "github.com/rygel/gouterstellar-platform/platform"
)

var versionPattern = regexp.MustCompile(`^V(\d+)__.*\.sql$`)

type migrationFile struct {
	Version  int64
	Filename string
	Content  string
}

type setEntry struct {
	extensionID string
	table       string
	fs          fs.FS
	dir         string
}

type byExtensionID []setEntry

func (s byExtensionID) Len() int { return len(s) }
func (s byExtensionID) Less(i, j int) bool {
	if s[i].extensionID == "platform-core" {
		return true
	}
	if s[j].extensionID == "platform-core" {
		return false
	}
	return s[i].extensionID < s[j].extensionID
}
func (s byExtensionID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type Runner struct {
	pool *pgxpool.Pool
	sets []setEntry
}

func NewRunner(pool *pgxpool.Pool, sets []extplatform.MigrationSet) *Runner {
	entries := make([]setEntry, len(sets))
	for i, s := range sets {
		entries[i] = setEntry{
			extensionID: s.ExtensionID,
			table:       s.Table,
			fs:          s.FS,
			dir:         s.Directory,
		}
	}
	sort.Sort(byExtensionID(entries))
	return &Runner{pool: pool, sets: entries}
}

func (r *Runner) Run(ctx context.Context) error {
	for _, set := range r.sets {
		if err := r.runSet(ctx, set); err != nil {
			return fmt.Errorf("migration set %s: %w", set.extensionID, err)
		}
	}
	return nil
}

func (r *Runner) runSet(ctx context.Context, set setEntry) error {
	if err := r.ensureHistoryTable(ctx, set); err != nil {
		return err
	}

	applied, err := r.appliedVersions(ctx, set)
	if err != nil {
		return err
	}

	files, err := readMigrationFiles(set)
	if err != nil {
		return err
	}

	pending := pendingFiles(files, applied)

	for _, m := range pending {
		if err := r.applyOne(ctx, set, m); err != nil {
			return err
		}
	}

	slog.Info("migration set complete",
		"extension", set.extensionID,
		"applied", len(pending),
		"skipped", len(applied),
	)
	return nil
}

func (r *Runner) ensureHistoryTable(ctx context.Context, set setEntry) error {
	_, err := r.pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version    BIGINT PRIMARY KEY,
			filename   TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, set.table))
	if err != nil {
		return fmt.Errorf("create history table %s: %w", set.table, err)
	}
	return nil
}

func (r *Runner) appliedVersions(ctx context.Context, set setEntry) (map[int64]bool, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf("SELECT version FROM %s", set.table))
	if err != nil {
		return nil, fmt.Errorf("read history %s: %w", set.table, err)
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func (r *Runner) applyOne(ctx context.Context, set setEntry, m migrationFile) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.Content); err != nil {
		return fmt.Errorf("apply %s: %w", m.Filename, err)
	}

	_, err = tx.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s (version, filename) VALUES ($1, $2)", set.table),
		m.Version, m.Filename)
	if err != nil {
		return fmt.Errorf("record %s: %w", m.Filename, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	slog.Info("migration applied",
		"extension", set.extensionID,
		"version", m.Version,
		"file", m.Filename,
	)
	return nil
}

func parseVersion(filename string) (int64, error) {
	matches := versionPattern.FindStringSubmatch(filename)
	if matches == nil {
		return 0, fmt.Errorf("not a migration file: %s (expected V<NNN>__name.sql)", filename)
	}
	return strconv.ParseInt(matches[1], 10, 64)
}

func readMigrationFiles(set setEntry) ([]migrationFile, error) {
	entries, err := fs.ReadDir(set.fs, set.dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", set.dir, err)
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ver, err := parseVersion(entry.Name())
		if err != nil {
			continue
		}
		content, err := fs.ReadFile(set.fs, set.dir+"/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		files = append(files, migrationFile{
			Version:  ver,
			Filename: entry.Name(),
			Content:  string(content),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})

	return files, nil
}

func pendingFiles(files []migrationFile, applied map[int64]bool) []migrationFile {
	var pending []migrationFile
	for _, f := range files {
		if !applied[f.Version] {
			pending = append(pending, f)
		}
	}
	return pending
}
