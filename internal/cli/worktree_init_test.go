package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/project"
)

func TestRunInitSteps_AllSucceed(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	steps := []project.InitStep{
		{Command: []string{"sh", "-c", "echo one"}},
		{Command: []string{"sh", "-c", "echo two"}},
	}
	if err := runInitSteps(steps, root, "feat-x", "/repo", &out, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "one") || !strings.Contains(s, "two") {
		t.Fatalf("missing child output: %q", s)
	}
	if !strings.Contains(s, "[init 1/2]") || !strings.Contains(s, "[init 2/2]") {
		t.Fatalf("missing init headers: %q", s)
	}
}

func TestRunInitSteps_FailFast(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "third-ran")
	var out bytes.Buffer
	steps := []project.InitStep{
		{Command: []string{"sh", "-c", "echo first"}},
		{Command: []string{"sh", "-c", "exit 7"}},
		{Command: []string{"sh", "-c", "touch " + marker}},
	}
	err := runInitSteps(steps, root, "feat-x", "/repo", &out, &out)
	if err == nil {
		t.Fatal("expected error from failing step")
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("third step must not run after fail-fast")
	}
}

func TestRunInitSteps_DirAndEnv(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "webapp")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	steps := []project.InitStep{
		// cwd は webapp、3つの DX_* env を書き出す
		{Command: []string{"sh", "-c",
			"pwd > pwd.txt; printf '%s' \"$DX_WORKTREE_BRANCH\" > branch.txt; " +
				"printf '%s' \"$DX_WORKTREE_PATH\" > path.txt; printf '%s' \"$DX_PRIMARY_ROOT\" > primary.txt"},
			Dir: "webapp"},
	}
	if err := runInitSteps(steps, root, "feat-x", "/repo", &out, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	pwd, _ := os.ReadFile(filepath.Join(sub, "pwd.txt"))
	if got := strings.TrimSpace(string(pwd)); !strings.HasSuffix(got, "webapp") {
		t.Fatalf("cwd = %q, want .../webapp", got)
	}
	br, _ := os.ReadFile(filepath.Join(sub, "branch.txt"))
	if string(br) != "feat-x" {
		t.Fatalf("DX_WORKTREE_BRANCH = %q", string(br))
	}
	wp, _ := os.ReadFile(filepath.Join(sub, "path.txt"))
	if string(wp) != root {
		t.Fatalf("DX_WORKTREE_PATH = %q, want %q", string(wp), root)
	}
	pr, _ := os.ReadFile(filepath.Join(sub, "primary.txt"))
	if string(pr) != "/repo" {
		t.Fatalf("DX_PRIMARY_ROOT = %q, want /repo", string(pr))
	}
}
