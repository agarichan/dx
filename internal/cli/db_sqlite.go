package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/agarichan/dx/internal/db"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

// worktreeRoots extracts checkout roots from `git worktree list --porcelain`.
func worktreeRoots(porcelain string) []string {
	var roots []string
	for _, line := range strings.Split(porcelain, "\n") {
		if rest, ok := strings.CutPrefix(line, "worktree "); ok {
			roots = append(roots, strings.TrimSpace(rest))
		}
	}
	return roots
}

// runDBSQLite implements `dx db <sub>` for engine = "sqlite".
// The db is a checkout-relative file: up/down have nothing to manage, fork
// seeds the worktree's file from the primary, drop/reset operate on the file.
// scheme applies to `url` only (e.g. sqlite+aiosqlite).
func runDBSQLite(sub string, cfg *project.Config, wt *worktree.Info, scheme string, stdout, stderr io.Writer) int {
	s := db.SQLite{Path: cfg.DB.Path}
	var err error
	switch sub {
	case "url":
		v, uerr := dbEnvValue(cfg, wt, scheme, os.Getenv)
		if uerr != nil {
			fmt.Fprintln(stderr, uerr)
			return 1
		}
		fmt.Fprintln(stdout, v)
		return 0
	case "up", "down":
		fmt.Fprintf(stdout, "db %s: engine=sqlite — no container to manage\n", sub)
		return 0
	case "fork":
		if wt.IsPrimary {
			fmt.Fprintln(stderr, "primary checkout: db fork is a no-op (run from a linked worktree)")
			return 1
		}
		err = s.Seed(dockerRunner, wt.PrimaryRoot, wt.Toplevel)
	case "drop":
		if wt.IsPrimary {
			fmt.Fprintln(stderr, "primary checkout: refusing to drop the base database")
			return 1
		}
		err = s.Drop(wt.Toplevel)
	case "reset":
		if wt.IsPrimary {
			fmt.Fprintln(stderr, "primary checkout: db reset is a no-op")
			return 1
		}
		if err = s.Drop(wt.Toplevel); err == nil {
			err = s.Seed(dockerRunner, wt.PrimaryRoot, wt.Toplevel)
		}
	case "list":
		out, lerr := gitRun(wt.Toplevel)("worktree", "list", "--porcelain")
		if lerr != nil {
			err = lerr
			break
		}
		for _, root := range worktreeRoots(out) {
			mark := "-"
			if fi, serr := os.Stat(s.File(root)); serr == nil {
				mark = fmt.Sprintf("%d bytes", fi.Size())
			}
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", root, s.Path, mark)
		}
		return 0
	case "psql":
		argv := s.ShellArgs(wt.Toplevel)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err = cmd.Run()
	default:
		fmt.Fprintf(stderr, "unknown db subcommand: %s\n", sub)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if sub != "psql" {
		fmt.Fprintf(stdout, "db %s: %s\n", sub, s.File(wt.Toplevel))
	}
	return 0
}
