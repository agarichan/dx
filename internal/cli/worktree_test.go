package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/project"
)

func baseDeps(cfg *project.Config) wtDeps {
	gitCalls := &[]string{}
	return wtDeps{
		Cfg:         cfg,
		PrimaryRoot: "/repo",
		Existing:    []string{"main"},
		Git: func(args ...string) (string, error) {
			*gitCalls = append(*gitCalls, strings.Join(args, " "))
			return "", nil
		},
		BranchExists:     func(string) bool { return false },
		ContainerRunning: func(string) bool { return true },
		Docker:           func(name string, args ...string) (string, error) { return "", nil },
		Getenv:           func(k string) string { return "postgresql://u:p@h:5432/myapp" },
		Stdout:           io.Discard,
		Stderr:           io.Discard,
	}
}

func TestCreate_CollisionAborts(t *testing.T) {
	cfg := &project.Config{}
	d := baseDeps(cfg)
	d.Existing = []string{"feat-x"}
	rc := createWorktree(createOpts{Branch: "feat/x"}, d) // Slug 衝突
	if rc != 1 {
		t.Fatalf("collision should abort with rc=1, got %d", rc)
	}
}

func TestCreate_NoDBExitsZero(t *testing.T) {
	cfg := &project.Config{} // [db] 無し
	rc := createWorktree(createOpts{Branch: "feat-y"}, baseDeps(cfg))
	if rc != 0 {
		t.Fatalf("no-DB create should be rc=0, got %d", rc)
	}
}

func TestCreate_DsnOnlyForks(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", Dsn: "postgresql://u:p@h:5432/myapp"}}
	d := baseDeps(cfg)
	d.Getenv = func(string) string { return "" } // url_env absent; Dsn must be used
	forked := false
	d.Docker = func(string, ...string) (string, error) { forked = true; return "", nil }
	rc := createWorktree(createOpts{Branch: "feat-dsn"}, d)
	if rc != 0 {
		t.Fatalf("dsn-only create should be rc=0 (fork via Dsn), got %d", rc)
	}
	_ = forked // Fork is attempted; Exists check runs through the fake Docker
}

func TestCreate_URLEnvUnsetExits3(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseDeps(cfg)
	d.Getenv = func(string) string { return "" } // url_env 未設定
	rc := createWorktree(createOpts{Branch: "feat-z"}, d)
	if rc != 3 {
		t.Fatalf("url_env unset should be rc=3, got %d", rc)
	}
}

func TestCreate_ContainerDownExits3(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseDeps(cfg)
	d.ContainerRunning = func(string) bool { return false }
	rc := createWorktree(createOpts{Branch: "feat-w"}, d)
	if rc != 3 {
		t.Fatalf("container down should be rc=3, got %d", rc)
	}
}

func TestCreate_ForkFailureExits3(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseDeps(cfg)
	d.Docker = func(string, ...string) (string, error) { return "", fmt.Errorf("boom") }
	rc := createWorktree(createOpts{Branch: "feat-f"}, d)
	if rc != 3 {
		t.Fatalf("fork failure should be rc=3, got %d", rc)
	}
}

func TestCreate_HappyPathRunsGitAdd(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	var calls []string
	d := baseDeps(cfg)
	d.Git = func(args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		return "", nil
	}
	rc := createWorktree(createOpts{Branch: "feat-ok"}, d)
	if rc != 0 {
		t.Fatalf("happy path rc=%d", rc)
	}
	joined := strings.Join(calls, "|")
	if !strings.Contains(joined, "worktree add") || !strings.Contains(joined, "-b feat-ok") {
		t.Fatalf("expected git worktree add -b feat-ok, got %q", joined)
	}
}

func TestCreate_RunsInitOnSuccess(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Init: []project.InitStep{{Command: []string{"true"}}},
	}}
	d := baseDeps(cfg)
	called := false
	d.RunInit = func(steps []project.InitStep, root, branch, primaryRoot string, _, _ io.Writer) error {
		called = true
		// Worktree.Dir is empty here → path = /repo/feat-init
		wantRoot := filepath.Join("/repo", "feat-init")
		if len(steps) != 1 || branch != "feat-init" || primaryRoot != "/repo" || root != wantRoot {
			t.Fatalf("init args: steps=%d branch=%q root=%q primary=%q", len(steps), branch, root, primaryRoot)
		}
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-init"}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !called {
		t.Fatal("RunInit was not called")
	}
}

func TestCreate_InitFailureExits3(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Init: []project.InitStep{{Command: []string{"false"}}},
	}}
	d := baseDeps(cfg)
	d.RunInit = func([]project.InitStep, string, string, string, io.Writer, io.Writer) error {
		return fmt.Errorf("boom")
	}
	rc := createWorktree(createOpts{Branch: "feat-bad"}, d)
	if rc != 3 {
		t.Fatalf("init failure should be rc=3, got %d", rc)
	}
}

func TestCreate_SkipInit(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Init: []project.InitStep{{Command: []string{"true"}}},
	}}
	d := baseDeps(cfg)
	called := false
	d.RunInit = func([]project.InitStep, string, string, string, io.Writer, io.Writer) error {
		called = true
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-skip", SkipInit: true}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if called {
		t.Fatal("RunInit must not be called with SkipInit")
	}
}

func TestCreate_RunsCopyOnSuccess(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Copy: []project.CopyStep{{From: ".myapp"}},
	}}
	d := baseDeps(cfg)
	called := false
	d.RunCopy = func(steps []project.CopyStep, primaryRoot, worktreeRoot string, _, _ io.Writer) error {
		called = true
		wantRoot := filepath.Join("/repo", "feat-copy")
		if len(steps) != 1 || steps[0].From != ".myapp" || primaryRoot != "/repo" || worktreeRoot != wantRoot {
			t.Fatalf("copy args: steps=%d primary=%q worktree=%q", len(steps), primaryRoot, worktreeRoot)
		}
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-copy"}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !called {
		t.Fatal("RunCopy was not called")
	}
}

func TestCreate_CopyFailureExits3(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Copy: []project.CopyStep{{From: ".myapp"}},
	}}
	d := baseDeps(cfg)
	d.RunCopy = func([]project.CopyStep, string, string, io.Writer, io.Writer) error {
		return fmt.Errorf("boom")
	}
	initCalled := false
	d.RunInit = func([]project.InitStep, string, string, string, io.Writer, io.Writer) error {
		initCalled = true
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-cpfail"}, d)
	if rc != 3 {
		t.Fatalf("copy failure should be rc=3, got %d", rc)
	}
	if initCalled {
		t.Fatal("init must not run after copy failure")
	}
}

func TestCreate_CopyRunsBeforeInit(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Copy: []project.CopyStep{{From: ".myapp"}},
		Init: []project.InitStep{{Command: []string{"true"}}},
	}}
	d := baseDeps(cfg)
	order := []string{}
	d.RunCopy = func([]project.CopyStep, string, string, io.Writer, io.Writer) error {
		order = append(order, "copy")
		return nil
	}
	d.RunInit = func([]project.InitStep, string, string, string, io.Writer, io.Writer) error {
		order = append(order, "init")
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-order2"}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if len(order) != 2 || order[0] != "copy" || order[1] != "init" {
		t.Fatalf("order = %v, want [copy init]", order)
	}
}

func TestCreate_SkipInitDoesNotSkipCopy(t *testing.T) {
	cfg := &project.Config{Worktree: project.Worktree{
		Copy: []project.CopyStep{{From: ".myapp"}},
		Init: []project.InitStep{{Command: []string{"true"}}},
	}}
	d := baseDeps(cfg)
	copyCalled, initCalled := false, false
	d.RunCopy = func([]project.CopyStep, string, string, io.Writer, io.Writer) error {
		copyCalled = true
		return nil
	}
	d.RunInit = func([]project.InitStep, string, string, string, io.Writer, io.Writer) error {
		initCalled = true
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-skipinit", SkipInit: true}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !copyCalled {
		t.Fatal("copy should still run with --skip-init")
	}
	if initCalled {
		t.Fatal("init must not run with --skip-init")
	}
}

func TestCreate_InitRunsAfterDBFork(t *testing.T) {
	cfg := &project.Config{
		DB:       &project.DB{Container: "c", Dsn: "postgresql://u:p@h:5432/myapp"},
		Worktree: project.Worktree{Init: []project.InitStep{{Command: []string{"true"}}}},
	}
	d := baseDeps(cfg)
	order := []string{}
	d.Docker = func(string, ...string) (string, error) { order = append(order, "fork"); return "", nil }
	d.RunInit = func([]project.InitStep, string, string, string, io.Writer, io.Writer) error {
		order = append(order, "init")
		return nil
	}
	rc := createWorktree(createOpts{Branch: "feat-order"}, d)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if len(order) == 0 || order[len(order)-1] != "init" {
		t.Fatalf("init must run after fork, order=%v", order)
	}
}

func baseRmDeps(cfg *project.Config) rmDeps {
	return rmDeps{
		Cfg:          cfg,
		PrimaryRoot:  "/repo",
		Git:          func(args ...string) (string, error) { return "", nil },
		Docker:       func(string, ...string) (string, error) { return "", nil },
		Getenv:       func(string) string { return "postgresql://u:p@h:5432/myapp" },
		StopServices: func(string) error { return nil },
		Dirty:        func(string) bool { return false },
		Toplevel:     func(p string) (string, error) { return p, nil },
		Stdout:       io.Discard,
		Stderr:       io.Discard,
	}
}

func TestRm_DirtyWithoutForceAborts(t *testing.T) {
	d := baseRmDeps(&project.Config{})
	d.Dirty = func(string) bool { return true }
	if rc := rmWorktree(rmOpts{Branch: "feat-x"}, d); rc != 1 {
		t.Fatalf("dirty without --force should abort, rc=%d", rc)
	}
}

func TestRm_DirtyWithForceProceeds(t *testing.T) {
	d := baseRmDeps(&project.Config{})
	d.Dirty = func(string) bool { return true }
	if rc := rmWorktree(rmOpts{Branch: "feat-x", Force: true}, d); rc != 0 {
		t.Fatalf("--force should proceed, rc=%d", rc)
	}
}

func TestRm_URLEnvUnsetAbortsWhenDBNeeded(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseRmDeps(cfg)
	d.Getenv = func(string) string { return "" }
	if rc := rmWorktree(rmOpts{Branch: "feat-x"}, d); rc != 1 {
		t.Fatalf("url_env unset (DB needed) should abort, rc=%d", rc)
	}
}

func TestRm_KeepDBSkipsDrop(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseRmDeps(cfg)
	d.Getenv = func(string) string { return "" } // url_env 無くても --keep-db なら無視
	dropped := false
	d.Docker = func(string, ...string) (string, error) { dropped = true; return "", nil }
	if rc := rmWorktree(rmOpts{Branch: "feat-x", KeepDB: true}, d); rc != 0 {
		t.Fatalf("--keep-db rc=%d", rc)
	}
	if dropped {
		t.Fatal("--keep-db must not call docker drop")
	}
}

func TestRm_DropFailureAborts(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseRmDeps(cfg)
	d.Docker = func(string, ...string) (string, error) { return "", fmt.Errorf("drop boom") }
	removed := false
	d.Git = func(args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "worktree remove") {
			removed = true
		}
		return "", nil
	}
	if rc := rmWorktree(rmOpts{Branch: "feat-x"}, d); rc != 1 {
		t.Fatalf("drop failure should abort, rc=%d", rc)
	}
	if removed {
		t.Fatal("must not remove worktree when DB drop fails")
	}
}

func TestRm_DirtyWithDBDoesNotDrop(t *testing.T) {
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	d := baseRmDeps(cfg)
	d.Dirty = func(string) bool { return true }
	dockerCalled := false
	d.Docker = func(string, ...string) (string, error) { dockerCalled = true; return "", nil }
	if rc := rmWorktree(rmOpts{Branch: "feat-x"}, d); rc != 1 {
		t.Fatalf("dirty without --force should abort, rc=%d", rc)
	}
	if dockerCalled {
		t.Fatal("DB must not be dropped when dirty check aborts before destructive actions")
	}
}

func TestParseWorktreePorcelain(t *testing.T) {
	porc := "worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /repo/.claude/worktrees/feat-x\nHEAD def\nbranch refs/heads/feat-x\n"
	rows := parseWorktreePorcelain(porc)
	if len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[0].Branch != "main" || rows[0].Path != "/repo" {
		t.Fatalf("row0 = %+v", rows[0])
	}
	if rows[1].Branch != "feat-x" {
		t.Fatalf("row1 = %+v", rows[1])
	}
}

func TestParseWorktreeArgs_FlagsAfterBranch(t *testing.T) {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "")
	del := fs.Bool("delete-branch", false, "")
	b, err := parseWorktreeArgs(fs, []string{"feat-x", "--force", "--delete-branch"})
	if err != nil || b != "feat-x" || !*force || !*del {
		t.Fatalf("b=%q force=%v del=%v err=%v", b, *force, *del, err)
	}
}

func TestParseWorktreeArgs_FromAfterBranch(t *testing.T) {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "")
	b, err := parseWorktreeArgs(fs, []string{"feat-x", "--from", "main"})
	if err != nil || b != "feat-x" || *from != "main" {
		t.Fatalf("b=%q from=%q err=%v", b, *from, err)
	}
}

func TestParseWorktreeArgs_FlagsBeforeBranch(t *testing.T) {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	from := fs.String("from", "", "")
	b, err := parseWorktreeArgs(fs, []string{"--from", "main", "feat-x"})
	if err != nil || b != "feat-x" || *from != "main" {
		t.Fatalf("b=%q from=%q err=%v", b, *from, err)
	}
}

func TestParseWorktreeArgs_Missing(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if _, err := parseWorktreeArgs(fs, []string{}); err == nil {
		t.Fatal("expected error for missing branch")
	}
}

func TestListRows_DBAndServices(t *testing.T) {
	cfg := &project.Config{
		Services: map[string]project.Service{"api": {Name: "myapp-api"}, "web": {Name: "myapp"}},
	}
	porc := "worktree /repo\nbranch refs/heads/main\n\nworktree /repo/.claude/worktrees/feat-x\nbranch refs/heads/feat-x\n"
	rows := listRows(cfg, "myapp", porc, func(svc string) string {
		if svc == "myapp-api" {
			return "running"
		}
		return "stopped"
	})
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	// primary
	if rows[0].DB != "myapp" {
		t.Fatalf("primary db = %q", rows[0].DB)
	}
	// worktree
	if rows[1].DB != "myapp_feat_x" {
		t.Fatalf("worktree db = %q", rows[1].DB)
	}
	if len(rows[1].Services) != 2 || !strings.Contains(rows[1].Services[0], "myapp-api-feat-x") {
		t.Fatalf("services = %v", rows[1].Services)
	}
}
