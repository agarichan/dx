package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/agarichan/dx/internal/db"
	"github.com/agarichan/dx/internal/dburl"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

// dockerRunner executes docker commands and returns combined output.
func dockerRunner(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s %s: %w: %s",
			name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRun returns a function that runs git in the given directory.
func gitRun(dir string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
}

// baseDSN returns the project's base DSN: cfg.DB.Dsn if set, else the value of
// the cfg.DB.URLEnv environment variable.
func baseDSN(cfg *project.Config) (*dburl.DSN, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("[db] table is required")
	}
	raw := cfg.DB.Dsn
	if raw == "" {
		raw = os.Getenv(cfg.DB.URLEnv)
	}
	if raw == "" {
		return nil, fmt.Errorf("[db]: set dsn or env var %q", cfg.DB.URLEnv)
	}
	return dburl.Parse(raw)
}

// containerFor builds a db.Container and the base DB name from dx.toml's [db].
func containerFor(cfg *project.Config) (db.Container, string, error) {
	base, err := baseDSN(cfg)
	if err != nil {
		return db.Container{}, "", err
	}
	return db.Container{Name: cfg.DB.Container, Image: cfg.DB.Image, Volume: cfg.DB.Volume, DSN: base}, base.Name, nil
}

// runDB implements the `dx db <sub>` dispatcher (config from dx.toml).
func runDB(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: dx db <up|down|psql|fork|drop|reset|list|url>")
		return 2
	}
	sub := args[0]
	cfg, wt, err := loadConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c, baseName, err := containerFor(cfg)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if sub == "up" && cfg.DB.Volume == "" {
		fmt.Fprintln(stderr, "[db].volume is required for `dx db up`")
		return 1
	}
	target := worktree.DBName(baseName, wt.Branch, wt.IsPrimary)
	isPrimary := target == baseName

	switch sub {
	case "url":
		fmt.Fprintln(stdout, c.DSN.WithName(target).String())
		return 0
	case "up":
		err = c.Up(dockerRunner)
	case "down":
		err = c.Down(dockerRunner)
	case "fork":
		if isPrimary {
			fmt.Fprintln(stderr, "primary checkout: db fork is a no-op (run from a linked worktree)")
			return 1
		}
		err = c.Fork(dockerRunner, baseName, target)
	case "drop":
		if isPrimary {
			fmt.Fprintln(stderr, "primary checkout: refusing to drop the base database")
			return 1
		}
		err = c.Drop(dockerRunner, target)
	case "reset":
		if isPrimary {
			fmt.Fprintln(stderr, "primary checkout: db reset is a no-op")
			return 1
		}
		if err = c.Drop(dockerRunner, target); err == nil {
			err = c.Fork(dockerRunner, baseName, target)
		}
	case "list":
		var out string
		out, err = c.List(dockerRunner, baseName)
		if err == nil {
			fmt.Fprintln(stdout, out)
		}
	case "psql":
		cmd := exec.Command("docker", c.PsqlArgs(target)...)
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
	if sub != "list" {
		fmt.Fprintf(stdout, "db %s: %s (db=%s)\n", sub, c.Name, target)
	}
	return 0
}
