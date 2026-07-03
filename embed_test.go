package dx

import (
	"io/fs"
	"testing"
)

// 拡張の動作に必須のファイルが埋め込まれていることを固定する。
func TestRaycastExtension_ContainsRequiredFiles(t *testing.T) {
	required := []string{
		"raycast/package.json",
		"raycast/package-lock.json",
		"raycast/tsconfig.json",
		"raycast/raycast-env.d.ts",
		"raycast/src/dx.ts",
		"raycast/src/list-services.tsx",
		"raycast/assets/extension-icon.png",
	}
	for _, p := range required {
		if _, err := fs.Stat(RaycastExtension, p); err != nil {
			t.Errorf("missing embedded file %s: %v", p, err)
		}
	}
}

func TestRaycastExtension_NoNodeModules(t *testing.T) {
	err := fs.WalkDir(RaycastExtension, "raycast", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			t.Fatalf("node_modules leaked into embed: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
