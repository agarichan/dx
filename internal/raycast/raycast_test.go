package raycast

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	dx "github.com/agarichan/dx"
)

func TestDataRoot(t *testing.T) {
	xdg := DataRoot(func(k string) string {
		return map[string]string{"XDG_DATA_HOME": "/x/data"}[k]
	})
	if xdg != "/x/data/dx" {
		t.Errorf("XDG_DATA_HOME: %q", xdg)
	}
	home := DataRoot(func(k string) string {
		return map[string]string{"HOME": "/Users/u"}[k]
	})
	if home != "/Users/u/.local/share/dx" {
		t.Errorf("HOME fallback: %q", home)
	}
}

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"raycast/package.json": {Data: []byte(`{"name":"dx"}`)},
		"raycast/src/dx.ts":    {Data: []byte("export {}\n")},
	}
}

func TestExtract_WritesTree(t *testing.T) {
	dst := t.TempDir()
	if err := Extract(testFS(), dst); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dst, "src", "dx.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "export {}\n" {
		t.Errorf("content = %q", b)
	}
}

func TestExtract_OverwritesButKeepsNodeModules(t *testing.T) {
	dst := t.TempDir()
	// 前回展開の残骸: 古い package.json と node_modules
	os.WriteFile(filepath.Join(dst, "package.json"), []byte("old"), 0o644)
	os.MkdirAll(filepath.Join(dst, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(dst, "node_modules", "x", "a.js"), []byte("keep"), 0o644)

	if err := Extract(testFS(), dst); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dst, "package.json"))
	if string(b) != `{"name":"dx"}` {
		t.Errorf("not overwritten: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dst, "node_modules", "x", "a.js")); err != nil {
		t.Errorf("node_modules not preserved: %v", err)
	}
}

func TestExtract_RealEmbedFS(t *testing.T) {
	dst := t.TempDir()
	if err := Extract(dx.RaycastExtension, dst); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"package.json", "package-lock.json", "tsconfig.json", "raycast-env.d.ts", "src/list-services.tsx", "assets/extension-icon.png"} {
		if _, err := os.Stat(filepath.Join(dst, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}
