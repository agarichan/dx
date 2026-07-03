package raycast

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeRay puts an executable shell script at <dir>/node_modules/.bin/ray.
func writeFakeRay(t *testing.T, dir, script string) {
	t.Helper()
	bin := filepath.Join(dir, "node_modules", ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ray"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRayImport_StopsAfterMarker(t *testing.T) {
	dir := t.TempDir()
	// マーカーを出したあと常駐するふり(watch 相当)。SIGINT で止まるはず。
	writeFakeRay(t, dir, `echo "ready"
echo "Built extension successfully"
sleep 30
`)
	var out bytes.Buffer
	start := time.Now()
	if err := rayImport(dir, &out, &out); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("did not stop promptly: %v", elapsed)
	}
	if !strings.Contains(out.String(), "Built extension successfully") {
		t.Errorf("output not forwarded: %q", out.String())
	}
}

func TestRayImport_ExitBeforeMarkerIsError(t *testing.T) {
	dir := t.TempDir()
	writeFakeRay(t, dir, `echo "some error"
exit 1
`)
	err := rayImport(dir, new(bytes.Buffer), new(bytes.Buffer))
	if err == nil || !strings.Contains(err.Error(), "before import completed") {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultTools_Wiring(t *testing.T) {
	tools := DefaultTools()
	if tools.LookPath == nil || tools.NpmCI == nil || tools.RayImport == nil {
		t.Fatal("DefaultTools has nil members")
	}
}
