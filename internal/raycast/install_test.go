package raycast

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPump_DetectsMarkerAcrossChunks(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer
	hits := 0
	done := make(chan struct{})
	go func() {
		pump(pr, &out, func() { hits++ })
		close(done)
	}()
	// 行の途中でチャンクが切れても行単位で検知できること
	io.WriteString(pw, "compiling...\nBuilt extension ")
	io.WriteString(pw, "successfully in 1.2s\ntail\n")
	pw.Close()
	<-done
	if hits != 1 {
		t.Fatalf("marker hits = %d", hits)
	}
	if !strings.Contains(out.String(), "Built extension successfully in 1.2s") {
		t.Errorf("output not forwarded: %q", out.String())
	}
}

func TestPump_NoMarker(t *testing.T) {
	var out bytes.Buffer
	hits := 0
	pump(strings.NewReader("error: something\n"), &out, func() { hits++ })
	if hits != 0 {
		t.Fatalf("unexpected marker hit")
	}
}

func okTools(calls *[]string) Tools {
	return Tools{
		LookPath: func(string) (string, error) { *calls = append(*calls, "lookpath"); return "/usr/bin/npm", nil },
		NpmCI: func(dir string, _, _ io.Writer) error {
			*calls = append(*calls, "npmci:"+filepath.Base(dir))
			return nil
		},
		RayImport: func(dir string, _, _ io.Writer) error {
			*calls = append(*calls, "ray:"+filepath.Base(dir))
			return nil
		},
	}
}

func TestInstall_RunsPipelineInOrder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ext")
	var calls []string
	src := fstest.MapFS{"raycast/package.json": {Data: []byte("{}")}}
	var out, errb bytes.Buffer
	if err := Install(src, dir, okTools(&calls), &out, &errb); err != nil {
		t.Fatal(err)
	}
	want := []string{"lookpath", "npmci:ext", "ray:ext"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Fatalf("calls = %v", calls)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		t.Errorf("not extracted: %v", err)
	}
	if !strings.Contains(out.String(), "dx raycast install") {
		t.Errorf("update hint missing: %q", out.String())
	}
}

func TestInstall_NpmMissing(t *testing.T) {
	tools := Tools{
		LookPath:  func(string) (string, error) { return "", errors.New("not found") },
		NpmCI:     func(string, io.Writer, io.Writer) error { t.Fatal("must not run"); return nil },
		RayImport: func(string, io.Writer, io.Writer) error { t.Fatal("must not run"); return nil },
	}
	err := Install(fstest.MapFS{}, t.TempDir(), tools, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "npm not found in PATH") {
		t.Fatalf("err = %v", err)
	}
}

func TestUninstall_RemovesDirAndPrintsGuidance(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ext")
	os.MkdirAll(dir, 0o755)
	var out bytes.Buffer
	if err := Uninstall(dir, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir still exists")
	}
	if !strings.Contains(out.String(), "Remove Extension") {
		t.Errorf("guidance missing: %q", out.String())
	}
}

func TestUninstall_MissingDirIsOK(t *testing.T) {
	var out bytes.Buffer
	if err := Uninstall(filepath.Join(t.TempDir(), "nope"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "nothing to remove") {
		t.Errorf("message missing: %q", out.String())
	}
}
