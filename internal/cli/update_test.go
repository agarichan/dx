package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestUpdate_UnknownFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"update", "--wat"}, &out, &errb); rc != 2 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.Contains(errb.String(), "usage: dx update") {
		t.Fatalf("stderr = %q", errb.String())
	}
}

func TestUpdate_Help(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := Run([]string{"update", "-h"}, &out, &errb); rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.Contains(out.String(), "dx update — ") {
		t.Fatalf("help = %q", out.String())
	}
}
