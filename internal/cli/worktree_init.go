package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agarichan/dx/internal/project"
)

// runInitSteps executes each [[worktree.init]] step in order, fail-fast.
// root is the new worktree's absolute path; each step runs in root or, when
// step.Dir is set, in root/<dir>. The child inherits the parent env plus
// DX_WORKTREE_BRANCH / DX_WORKTREE_PATH / DX_PRIMARY_ROOT, and its
// stdout/stderr stream straight through.
func runInitSteps(steps []project.InitStep, root, branch, primaryRoot string, stdout, stderr io.Writer) error {
	env := append(os.Environ(),
		"DX_WORKTREE_BRANCH="+branch,
		"DX_WORKTREE_PATH="+root,
		"DX_PRIMARY_ROOT="+primaryRoot,
	)
	for i, step := range steps {
		dir := root
		label := strings.Join(step.Command, " ")
		if step.Dir != "" {
			dir = filepath.Join(root, step.Dir)
			fmt.Fprintf(stdout, "> [init %d/%d] %s (in %s)\n", i+1, len(steps), label, step.Dir)
		} else {
			fmt.Fprintf(stdout, "> [init %d/%d] %s\n", i+1, len(steps), label)
		}
		cmd := exec.Command(step.Command[0], step.Command[1:]...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, label, err)
		}
	}
	return nil
}
