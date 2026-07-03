package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

func sqliteCfg() *project.Config {
	return &project.Config{DB: &project.DB{Engine: "sqlite", Path: "dev.db"}}
}

func TestRunDBSQLite_URL(t *testing.T) {
	wt := &worktree.Info{Toplevel: "/repo", IsPrimary: true, PrimaryRoot: "/repo"}
	var out, errb bytes.Buffer
	if rc := runDBSQLite("url", sqliteCfg(), wt, "", &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	if strings.TrimSpace(out.String()) != "sqlite:////repo/dev.db" {
		t.Fatalf("url = %q", out.String())
	}
}

func TestRunDBSQLite_UpDownNoop(t *testing.T) {
	wt := &worktree.Info{Toplevel: "/repo", IsPrimary: true, PrimaryRoot: "/repo"}
	for _, sub := range []string{"up", "down"} {
		var out, errb bytes.Buffer
		if rc := runDBSQLite(sub, sqliteCfg(), wt, "", &out, &errb); rc != 0 {
			t.Fatalf("%s rc=%d", sub, rc)
		}
		if !strings.Contains(out.String(), "no container") {
			t.Fatalf("%s out = %q", sub, out.String())
		}
	}
}

func TestRunDBSQLite_ForkRefusedOnPrimary(t *testing.T) {
	wt := &worktree.Info{Toplevel: "/repo", IsPrimary: true, PrimaryRoot: "/repo"}
	var out, errb bytes.Buffer
	if rc := runDBSQLite("fork", sqliteCfg(), wt, "", &out, &errb); rc != 1 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(errb.String(), "primary") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestRunDBSQLite_ForkAndDropOnWorktree(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not on PATH")
	}
	primary, wtRoot := t.TempDir(), t.TempDir()
	mk := exec.Command("sqlite3", filepath.Join(primary, "dev.db"), "CREATE TABLE t(x);")
	if out, err := mk.CombinedOutput(); err != nil {
		t.Fatalf("mk: %v %s", err, out)
	}
	wt := &worktree.Info{Toplevel: wtRoot, IsPrimary: false, PrimaryRoot: primary}
	var out, errb bytes.Buffer
	if rc := runDBSQLite("fork", sqliteCfg(), wt, "", &out, &errb); rc != 0 {
		t.Fatalf("fork rc=%d %s", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(wtRoot, "dev.db")); err != nil {
		t.Fatalf("seeded file missing: %v", err)
	}
	if rc := runDBSQLite("drop", sqliteCfg(), wt, "", &out, &errb); rc != 0 {
		t.Fatalf("drop rc=%d %s", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(wtRoot, "dev.db")); !os.IsNotExist(err) {
		t.Fatal("file not dropped")
	}
}

func TestWorktreeRootsFromPorcelain(t *testing.T) {
	porcelain := `worktree /home/u/work/myapp
HEAD abc
branch refs/heads/main

worktree /home/u/work/myapp/.claude/worktrees/x
HEAD def
branch refs/heads/x
`
	got := worktreeRoots(porcelain)
	want := []string{"/home/u/work/myapp", "/home/u/work/myapp/.claude/worktrees/x"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("roots = %v", got)
	}
}

func TestRunDBSQLite_URLWithScheme(t *testing.T) {
	wt := &worktree.Info{Toplevel: "/repo", IsPrimary: true, PrimaryRoot: "/repo"}
	var out, errb bytes.Buffer
	if rc := runDBSQLite("url", sqliteCfg(), wt, "sqlite+aiosqlite", &out, &errb); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errb.String())
	}
	if strings.TrimSpace(out.String()) != "sqlite+aiosqlite:////repo/dev.db" {
		t.Fatalf("url = %q", out.String())
	}
}

func TestURLScheme_FlagParsing(t *testing.T) {
	s, err := urlScheme([]string{"--scheme", "postgresql+psycopg"})
	if err != nil || s != "postgresql+psycopg" {
		t.Fatalf("s=%q err=%v", s, err)
	}
	if _, err := urlScheme([]string{"--wat"}); err == nil {
		t.Fatal("unknown flag must error")
	}
	if _, err := urlScheme([]string{"--scheme"}); err == nil {
		t.Fatal("missing value must error")
	}
}
