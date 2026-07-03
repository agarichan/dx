package cli

import (
	"fmt"
	"io"

	"github.com/agarichan/dx/internal/selfupdate"
)

// runUpdate implements `dx update [--force]` (self-update from GitHub Releases).
func runUpdate(args []string, stdout, stderr io.Writer) int {
	force := false
	for _, a := range args {
		switch a {
		case "--force", "-f":
			force = true
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\nusage: dx update [--force]\n", a)
			return 2
		}
	}
	err := selfupdate.Run(selfupdate.Options{
		Repo: "agarichan/dx", Current: version, Force: force,
	}, stdout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
