package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type Spec struct {
	Dir     string
	Env     []string
	Command []string
	LogPath string
	// ColorLogPath, when set, switches Start to the two-file pump mode: instead of
	// redirecting the child's stdout/stderr straight to LogPath, Start re-invokes the
	// dx binary as the hidden `__logpump` subcommand. The pump becomes the session
	// leader, forks the real child, and fans the combined output out to two files —
	// the ANSI-stripped canonical plain log (LogPath) and the raw-ANSI color log
	// (ColorLogPath). Empty keeps the legacy direct-file behavior.
	ColorLogPath string
}

// Start launches the command in its own process group, detached, logging to LogPath.
func Start(s Spec) (int, error) {
	if len(s.Command) == 0 {
		return 0, fmt.Errorf("empty command")
	}

	var cmd *exec.Cmd
	if s.ColorLogPath != "" {
		// Two-file pump mode: the pump opens the files itself, so no redirect here.
		self, err := os.Executable()
		if err != nil {
			return 0, fmt.Errorf("locate dx: %w", err)
		}
		wrapped := append([]string{self, "__logpump", "--plain", s.LogPath, "--color", s.ColorLogPath, "--"}, s.Command...)
		cmd = exec.Command(wrapped[0], wrapped[1:]...)
		cmd.Dir = s.Dir
		cmd.Env = s.Env
		cmd.Stdin = nil
		// no Stdout/Stderr redirect — the pump opens the files itself
		return startDetached(cmd)
	}

	f, err := os.OpenFile(s.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	cmd = exec.Command(s.Command[0], s.Command[1:]...)
	cmd.Dir = s.Dir
	cmd.Env = s.Env
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil
	return startDetached(cmd)
}

// startDetached starts cmd in a brand-new session and detaches, returning its pid.
//
// Setsid puts the leader in a brand-new session (new process group, NO controlling
// terminal). This fully daemonizes it so launchers that wait on their process group
// or pty — notably `mise run` with reload-spawning servers like uvicorn --reload —
// do not block on the detached child. The leader becomes its own session+group leader
// (pgid == pid), so StopGroup(-pid) still signals the whole group. In two-file pump
// mode the leader is the pump process; the real child is its descendant in the group.
func startDetached(cmd *exec.Cmd) (int, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start: %w", err)
	}
	pid := cmd.Process.Pid
	// Detach: do not wait. Release so the child is reparented.
	_ = cmd.Process.Release()
	return pid, nil
}

// Alive reports whether a process group leader is still running.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Try to reap the process if it's a zombie, which also checks if it exists.
	var ws syscall.WaitStatus
	wpid, _ := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
	if wpid == pid {
		// Successfully reaped the process, so it's dead now.
		return false
	}
	// If waitpid failed or didn't return the expected pid, check with Kill signal 0.
	return syscall.Kill(pid, 0) == nil
}

// StopGroup sends SIGTERM to the entire process group led by pid.
func StopGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	// Negative pid targets the process group.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return fmt.Errorf("kill group %d: %w", pid, err)
	}
	return nil
}
