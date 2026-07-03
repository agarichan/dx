package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/project"
)

func writeFile(t *testing.T, path, body string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), perm); err != nil {
		t.Fatal(err)
	}
}

func TestRunCopySteps_FileAndDir(t *testing.T) {
	primary := t.TempDir()
	wt := t.TempDir()
	writeFile(t, filepath.Join(primary, ".myapp/credentials.json"), `{"k":"v"}`, 0o600)
	writeFile(t, filepath.Join(primary, ".myapp/sessions/a.json"), `session`, 0o644)
	writeFile(t, filepath.Join(primary, ".env.local"), "SECRET=1", 0o644)

	var out bytes.Buffer
	steps := []project.CopyStep{
		{From: ".myapp"},
		{From: ".env.local"},
	}
	if err := runCopySteps(steps, primary, wt, &out, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// dir copied recursively, permissions preserved on the file
	got, err := os.ReadFile(filepath.Join(wt, ".myapp/credentials.json"))
	if err != nil || string(got) != `{"k":"v"}` {
		t.Fatalf("credentials.json body = %q err=%v", got, err)
	}
	info, err := os.Stat(filepath.Join(wt, ".myapp/credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(wt, ".myapp/sessions/a.json")); err != nil {
		t.Fatalf("nested file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, ".env.local")); err != nil {
		t.Fatalf(".env.local missing: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "[copy 1/2]") || !strings.Contains(s, "[copy 2/2]") {
		t.Fatalf("missing copy headers: %q", s)
	}
}

func TestRunCopySteps_MissingSourceIsSkipped(t *testing.T) {
	primary := t.TempDir()
	wt := t.TempDir()
	// only step2's source exists
	writeFile(t, filepath.Join(primary, "present.txt"), "x", 0o644)
	var out bytes.Buffer
	steps := []project.CopyStep{
		{From: "missing.txt"},
		{From: "present.txt"},
	}
	if err := runCopySteps(steps, primary, wt, &out, &out); err != nil {
		t.Fatalf("missing source must not fail: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "missing.txt")); !os.IsNotExist(err) {
		t.Fatalf("missing.txt must not be created")
	}
	if _, err := os.Stat(filepath.Join(wt, "present.txt")); err != nil {
		t.Fatalf("present.txt should exist: %v", err)
	}
	if !strings.Contains(out.String(), "source missing, skipped") {
		t.Fatalf("expected skip log: %q", out.String())
	}
}

func TestRunCopySteps_ExistingDestinationIsSkipped(t *testing.T) {
	primary := t.TempDir()
	wt := t.TempDir()
	writeFile(t, filepath.Join(primary, "config.json"), "PRIMARY", 0o644)
	writeFile(t, filepath.Join(wt, "config.json"), "WORKTREE", 0o644)
	var out bytes.Buffer
	steps := []project.CopyStep{{From: "config.json"}}
	if err := runCopySteps(steps, primary, wt, &out, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(wt, "config.json"))
	if string(got) != "WORKTREE" {
		t.Fatalf("destination must not be overwritten, got %q", got)
	}
	if !strings.Contains(out.String(), "destination exists, skipped") {
		t.Fatalf("expected skip log: %q", out.String())
	}
}

func TestRunCopySteps_NestedDestinationParentIsCreated(t *testing.T) {
	primary := t.TempDir()
	wt := t.TempDir()
	writeFile(t, filepath.Join(primary, "a/b/c.txt"), "hi", 0o644)
	var out bytes.Buffer
	steps := []project.CopyStep{{From: "a/b/c.txt"}}
	if err := runCopySteps(steps, primary, wt, &out, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(wt, "a/b/c.txt"))
	if err != nil || string(got) != "hi" {
		t.Fatalf("copy result = %q err=%v", got, err)
	}
}

func TestRunCopySteps_SymlinkPreserved(t *testing.T) {
	primary := t.TempDir()
	wt := t.TempDir()
	writeFile(t, filepath.Join(primary, "real.txt"), "R", 0o644)
	if err := os.Symlink("real.txt", filepath.Join(primary, "link.txt")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	var out bytes.Buffer
	steps := []project.CopyStep{{From: "link.txt"}}
	if err := runCopySteps(steps, primary, wt, &out, &out); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	target, err := os.Readlink(filepath.Join(wt, "link.txt"))
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if target != "real.txt" {
		t.Fatalf("symlink target = %q", target)
	}
}
