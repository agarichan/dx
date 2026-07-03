package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	dx "github.com/agarichan/dx"
	"github.com/agarichan/dx/internal/raycast"
)

// extensionDir is where the embedded extension source is extracted for Raycast.
func extensionDir() string {
	return filepath.Join(raycast.DataRoot(os.Getenv), "raycast-extension")
}

// runRaycast implements `dx raycast <install|uninstall>`.
func runRaycast(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: dx raycast <install|uninstall>")
		return 2
	}
	switch args[0] {
	case "install":
		if err := raycast.Install(dx.RaycastExtension, extensionDir(), raycast.DefaultTools(), stdout, stderr); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "uninstall":
		if err := raycast.Uninstall(extensionDir(), stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown raycast subcommand: %s\n", args[0])
		return 2
	}
}
