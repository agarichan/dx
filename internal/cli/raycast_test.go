package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRaycast_NoSubcommandUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"raycast"}, &out, &errb); rc != 2 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.Contains(errb.String(), "usage: dx raycast <install|uninstall>") {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestRaycast_UnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"raycast", "wat"}, &out, &errb); rc != 2 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.Contains(errb.String(), "unknown raycast subcommand: wat") {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestRaycast_UninstallNothingToRemove(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var out, errb bytes.Buffer
	if rc := Run([]string{"raycast", "uninstall"}, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d, stderr = %q", rc, errb.String())
	}
	if !strings.Contains(out.String(), "nothing to remove") {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRaycast_Help(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"raycast", "-h"}, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.Contains(out.String(), "dx raycast — ") {
		t.Errorf("help = %q", out.String())
	}
}
