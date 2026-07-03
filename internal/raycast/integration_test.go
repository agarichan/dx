package raycast

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	dx "github.com/agarichan/dx"
)

// TestIntegration_EmbeddedSourceBuilds proves the embedded file set is
// complete: extract → npm ci → ray build succeeds.
// Requires network (npm) — excluded from -short runs. The real Raycast import
// is NOT exercised (it would register the extension into the developer's app).
func TestIntegration_EmbeddedSourceBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: skipped with -short")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("integration: npm not on PATH")
	}
	dir := t.TempDir()
	if err := Extract(dx.RaycastExtension, dir); err != nil {
		t.Fatal(err)
	}
	npm := exec.Command("npm", "ci")
	npm.Dir = dir
	if out, err := npm.CombinedOutput(); err != nil {
		t.Fatalf("npm ci: %v\n%s", err, out)
	}
	ray := exec.Command(filepath.Join(dir, "node_modules", ".bin", "ray"), "build", "-e", "dist", "-o", filepath.Join(dir, "dist"))
	ray.Dir = dir
	if out, err := ray.CombinedOutput(); err != nil {
		t.Fatalf("ray build: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "dist")); err != nil {
		t.Fatalf("dist not produced: %v", err)
	}
}
