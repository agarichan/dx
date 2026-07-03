// Package raycast installs the embedded Raycast extension into the user's
// Raycast app (extract → npm ci → one-shot `ray develop` import).
package raycast

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DataRoot returns the base dir for dx data files (XDG_DATA_HOME or
// ~/.local/share), mirroring registry.StateRoot.
func DataRoot(getenv func(string) string) string {
	if x := getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "dx")
	}
	return filepath.Join(getenv("HOME"), ".local", "share", "dx")
}

// Extract writes the "raycast/" subtree of src into dst, overwriting existing
// files. Files Extract does not know about (node_modules) are left alone, so
// re-running install updates sources without discarding installed deps.
func Extract(src fs.FS, dst string) error {
	return fs.WalkDir(src, "raycast", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "raycast")
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}
