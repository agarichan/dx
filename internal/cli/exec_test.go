package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

func execFixture(t *testing.T) (*project.Config, *worktree.Info) {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "api"), 0o755)
	os.WriteFile(filepath.Join(root, "dev.db"), []byte("x"), 0o644)
	cfg := &project.Config{
		Root: root,
		DB:   &project.DB{Engine: "sqlite", Path: "dev.db"},
		Services: map[string]project.Service{
			"api": {Key: "api", Name: "api", Dir: "api",
				DBEnv: &project.DBEnv{Name: "APP_DATABASE_URL", Scheme: "sqlite+aiosqlite"}},
		},
	}
	wt := &worktree.Info{Toplevel: root, Branch: "main", IsPrimary: true, PrimaryRoot: root}
	return cfg, wt
}

func TestExecService_EnvDirAndExitCode(t *testing.T) {
	cfg, wt := execFixture(t)
	var out, errb bytes.Buffer
	rc := execService(cfg, cfg.Services["api"], wt,
		[]string{"sh", "-c", "echo $APP_DATABASE_URL; pwd"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("out = %q", out.String())
	}
	if want := "sqlite+aiosqlite:///" + filepath.Join(cfg.Root, "dev.db"); lines[0] != want {
		t.Fatalf("env = %q want %q", lines[0], want)
	}
	if got := lines[1]; filepath.Base(got) != "api" {
		t.Fatalf("cwd = %q", got)
	}
}

func TestExecService_PropagatesExitCode(t *testing.T) {
	cfg, wt := execFixture(t)
	rc := execService(cfg, cfg.Services["api"], wt,
		[]string{"sh", "-c", "exit 7"}, new(bytes.Buffer), new(bytes.Buffer))
	if rc != 7 {
		t.Fatalf("rc=%d", rc)
	}
}

func TestExecService_SeedsWorktreeDB(t *testing.T) {
	cfg, wt := execFixture(t)
	// linked worktree: primary has the db, the worktree does not yet
	wtRoot := t.TempDir()
	os.MkdirAll(filepath.Join(wtRoot, "api"), 0o755)
	lw := &worktree.Info{Toplevel: wtRoot, Branch: "feat-x", IsPrimary: false, PrimaryRoot: cfg.Root}
	wcfg := *cfg
	wcfg.Root = wtRoot
	rc := execService(&wcfg, cfg.Services["api"], lw,
		[]string{"sh", "-c", "test -f $DX_TEST_UNUSED; true"}, new(bytes.Buffer), new(bytes.Buffer))
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if _, err := os.Stat(filepath.Join(wtRoot, "dev.db")); err != nil {
		t.Fatalf("worktree db not seeded: %v", err)
	}
	_ = wt
}

func TestRunExec_Usage(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"exec"}, &out, &errb); rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(errb.String(), "usage: dx exec") {
		t.Fatalf("stderr=%q", errb.String())
	}
}
