package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/project"
)

func sqliteDeps(t *testing.T) (wtDeps, string) {
	t.Helper()
	primary := t.TempDir()
	os.WriteFile(filepath.Join(primary, "dev.db"), []byte("x"), 0o644)
	cfg := &project.Config{
		DB:       &project.DB{Engine: "sqlite", Path: "dev.db"},
		Worktree: project.Worktree{Dir: ".claude/worktrees"},
	}
	d := baseDeps(cfg)
	d.PrimaryRoot = primary
	d.ContainerRunning = func(string) bool { t.Fatal("sqlite must not check containers"); return false }
	return d, primary
}

func TestCreate_SQLiteSeeds(t *testing.T) {
	d, primary := sqliteDeps(t)
	var seedCmd []string
	d.Docker = func(name string, args ...string) (string, error) {
		seedCmd = append([]string{name}, args...)
		return "", nil
	}
	rc := createWorktree(createOpts{Branch: "feat-s"}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	wtPath := filepath.Join(primary, ".claude/worktrees", "feat-s")
	if len(seedCmd) == 0 || seedCmd[0] != "sqlite3" || !strings.Contains(strings.Join(seedCmd, " "), wtPath) {
		t.Fatalf("seed cmd = %v", seedCmd)
	}
}

func TestCreate_SQLiteSeedFailureExits3(t *testing.T) {
	d, _ := sqliteDeps(t)
	d.Docker = func(string, ...string) (string, error) { return "", fmt.Errorf("boom") }
	rc := createWorktree(createOpts{Branch: "feat-sf"}, d)
	if rc != 3 {
		t.Fatalf("rc=%d", rc)
	}
}
