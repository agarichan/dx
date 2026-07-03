package db

import (
	"fmt"
	"os"
	"path/filepath"
)

// SQLite manages a checkout-relative database file. Per-worktree isolation is
// inherent (each checkout has its own file); Seed copies the primary's data
// into a fresh worktree.
type SQLite struct {
	Path string // relative to a checkout root
}

// File returns the absolute db file path for the given checkout root.
func (s SQLite) File(root string) string {
	return filepath.Join(root, filepath.FromSlash(s.Path))
}

// URL returns the SQLAlchemy-style DSN for the given checkout root
// (sqlite:////abs/path — four slashes = absolute).
func (s SQLite) URL(root string) string {
	return "sqlite:///" + s.File(root)
}

// Exists reports whether the checkout's db file exists.
func (s SQLite) Exists(root string) bool {
	_, err := os.Stat(s.File(root))
	return err == nil
}

// Seed copies baseRoot's db into targetRoot via `sqlite3 ".backup"`, which
// takes a consistent snapshot even while the base has active writers.
// Idempotent: an existing target is left untouched. A missing base errors.
func (s SQLite) Seed(run Runner, baseRoot, targetRoot string) error {
	base, target := s.File(baseRoot), s.File(targetRoot)
	if _, err := os.Stat(base); err != nil {
		return fmt.Errorf("base db %s: %w", base, err)
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", target, err)
	}
	if out, err := run("sqlite3", base, fmt.Sprintf(".backup '%s'", target)); err != nil {
		return fmt.Errorf("sqlite3 .backup: %w: %s", err, out)
	}
	return nil
}

// Drop removes the checkout's db file and its -wal/-shm siblings.
// Missing files are not an error.
func (s SQLite) Drop(root string) error {
	f := s.File(root)
	for _, p := range []string{f, f + "-wal", f + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// ShellArgs returns the argv for an interactive sqlite3 shell.
func (s SQLite) ShellArgs(root string) []string {
	return []string{"sqlite3", s.File(root)}
}
