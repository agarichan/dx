package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/agarichan/dx/internal/config"
	"github.com/agarichan/dx/internal/portless"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

// execService runs cmdArgs in svc's environment (buildEnv: URL vars + db_env)
// and working dir, after the same idempotent DB fork/seed as dx serve —
// a fresh worktree's first command is typically the migration itself.
// The child's exit code is propagated.
func execService(cfg *project.Config, svc project.Service, wt *worktree.Info, cmdArgs []string, stdout, stderr io.Writer) int {
	pcli := portless.Client{R: config.Load(os.Getenv)}
	env, err := buildEnv(os.Environ(), cfg, svc, wt, pcli)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ensureServiceDB(cfg, svc, wt, stderr)
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = serviceDir(cfg, svc)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// runExec implements `dx exec <key> [--] <command...>`.
func runExec(args []string, stdout, stderr io.Writer) int {
	usage := func() int {
		fmt.Fprintln(stderr, "usage: dx exec <key> [--] <command...>")
		return 2
	}
	if len(args) < 2 {
		return usage()
	}
	key, rest := args[0], args[1:]
	if rest[0] == "--" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return usage()
	}
	cfg, wt, err := loadConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	svc, ok := cfg.Service(key)
	if !ok {
		fmt.Fprintf(stderr, "unknown service: %s\n", key)
		return 1
	}
	return execService(cfg, svc, wt, rest, stdout, stderr)
}
