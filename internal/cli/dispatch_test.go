package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorktreeHelp_MentionsSkipInit(t *testing.T) {
	if !strings.Contains(worktreeHelp, "--skip-init") {
		t.Fatalf("worktree help must mention --skip-init: %q", worktreeHelp)
	}
}

func TestRun_Version(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"version"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "dx") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"frobnicate"}, &out, &errb)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown command")
	}
	if !strings.Contains(errb.String(), "unknown command") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestRun_Help(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"-h"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out.String(), "USAGE:") {
		t.Fatalf("missing USAGE: in %q", out.String())
	}
	if !strings.Contains(out.String(), "worktree") {
		t.Fatalf("missing worktree in %q", out.String())
	}
}

func TestRun_HelpAfterSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"logs", "-h"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out.String(), "USAGE:") {
		t.Fatalf("missing USAGE: in %q", out.String())
	}
}

func TestHelp_PerCommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run([]string{"logs", "-h"}, &out, &errb)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	got := out.String()
	if !strings.Contains(got, "--time") {
		t.Fatalf("logs -h must contain --time: %q", got)
	}
	if !strings.Contains(got, "--follow") {
		t.Fatalf("logs -h must contain --follow: %q", got)
	}
	// Per-command help must NOT bleed in the overview worktree section.
	if strings.Contains(got, "worktree create") {
		t.Fatalf("logs -h must NOT contain 'worktree create': %q", got)
	}
}
