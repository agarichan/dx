package db

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLite_FileAndURL(t *testing.T) {
	s := SQLite{Path: "data/dev.db"}
	if got := s.File("/repo"); got != "/repo/data/dev.db" {
		t.Errorf("File = %q", got)
	}
	if got := s.URL("/repo"); got != "sqlite:////repo/data/dev.db" {
		t.Errorf("URL = %q", got)
	}
}

func TestSQLite_SeedCommandShape(t *testing.T) {
	s := SQLite{Path: "dev.db"}
	base, target := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(base, "dev.db"), []byte("x"), 0o644)

	var got []string
	run := func(name string, args ...string) (string, error) {
		got = append([]string{name}, args...)
		return "", nil
	}
	if err := s.Seed(run, base, target); err != nil {
		t.Fatal(err)
	}
	want := []string{"sqlite3", filepath.Join(base, "dev.db"), ".backup '" + filepath.Join(target, "dev.db") + "'"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("cmd = %v", got)
	}
}

func TestSQLite_SeedIdempotentAndMissingBase(t *testing.T) {
	s := SQLite{Path: "dev.db"}
	base, target := t.TempDir(), t.TempDir()
	calls := 0
	run := func(string, ...string) (string, error) { calls++; return "", nil }

	// base missing → error, no exec
	if err := s.Seed(run, base, target); err == nil || calls != 0 {
		t.Fatalf("want missing-base error without exec, err=%v calls=%d", err, calls)
	}

	os.WriteFile(filepath.Join(base, "dev.db"), []byte("x"), 0o644)
	// target already present → no-op
	os.WriteFile(filepath.Join(target, "dev.db"), []byte("y"), 0o644)
	if err := s.Seed(run, base, target); err != nil || calls != 0 {
		t.Fatalf("want idempotent no-op, err=%v calls=%d", err, calls)
	}
}

func TestSQLite_SeedCreatesParentDir(t *testing.T) {
	s := SQLite{Path: "data/nested/dev.db"}
	base, target := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(base, "data", "nested"), 0o755)
	os.WriteFile(filepath.Join(base, "data", "nested", "dev.db"), []byte("x"), 0o644)
	run := func(string, ...string) (string, error) { return "", nil }
	if err := s.Seed(run, base, target); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(target, "data", "nested")); err != nil || !fi.IsDir() {
		t.Fatalf("parent dir not created: %v", err)
	}
}

func TestSQLite_SeedReal(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	s := SQLite{Path: "dev.db"}
	base, target := t.TempDir(), t.TempDir()
	mk := exec.Command("sqlite3", filepath.Join(base, "dev.db"), "CREATE TABLE t(x); INSERT INTO t VALUES (42);")
	if out, err := mk.CombinedOutput(); err != nil {
		t.Fatalf("create base db: %v\n%s", err, out)
	}
	real := func(name string, args ...string) (string, error) {
		out, err := exec.Command(name, args...).CombinedOutput()
		return string(out), err
	}
	if err := s.Seed(real, base, target); err != nil {
		t.Fatal(err)
	}
	q := exec.Command("sqlite3", filepath.Join(target, "dev.db"), "SELECT x FROM t;")
	out, err := q.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "42" {
		t.Fatalf("seeded db query = %q err=%v", out, err)
	}
}

func TestSQLite_Drop(t *testing.T) {
	s := SQLite{Path: "dev.db"}
	root := t.TempDir()
	for _, f := range []string{"dev.db", "dev.db-wal", "dev.db-shm"} {
		os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644)
	}
	if err := s.Drop(root); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"dev.db", "dev.db-wal", "dev.db-shm"} {
		if _, err := os.Stat(filepath.Join(root, f)); !os.IsNotExist(err) {
			t.Errorf("%s not removed", f)
		}
	}
	// missing file → no error
	if err := s.Drop(root); err != nil {
		t.Fatalf("drop on missing file: %v", err)
	}
}

func TestSQLite_Exists(t *testing.T) {
	s := SQLite{Path: "dev.db"}
	root := t.TempDir()
	if s.Exists(root) {
		t.Fatal("Exists on missing file")
	}
	os.WriteFile(filepath.Join(root, "dev.db"), []byte("x"), 0o644)
	if !s.Exists(root) {
		t.Fatal("Exists on present file")
	}
}
