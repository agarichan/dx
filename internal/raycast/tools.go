package raycast

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// importTimeout aborts the ray import when the build marker never appears
// (typical cause: Raycast.app not running).
const importTimeout = 120 * time.Second

// DefaultTools returns Tools backed by real npm / ray executions.
func DefaultTools() Tools {
	return Tools{
		LookPath: exec.LookPath,
		NpmCI: func(dir string, stdout, stderr io.Writer) error {
			// dev 依存も入れる: ray が型定義生成に typescript を要求する
			cmd := exec.Command("npm", "ci")
			cmd.Dir = dir
			cmd.Stdout = stdout
			cmd.Stderr = stderr
			return cmd.Run()
		},
		RayImport: rayImport,
	}
}

// rayImport runs `ray develop -I` (which persistently imports the extension
// into Raycast), then stops it with SIGINT once the build marker appears —
// the Go port of raycast/scripts/import.mjs.
func rayImport(dir string, stdout, stderr io.Writer) error {
	cmd := exec.Command(filepath.Join(dir, "node_modules", ".bin", "ray"), "develop", "-I")
	cmd.Dir = dir
	// ray の子孫プロセスがパイプを握ったままでも Wait が返るようにする
	cmd.WaitDelay = 3 * time.Second
	pr, pw := io.Pipe() // stdout+stderr を1本にまとめて行スキャン
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		return fmt.Errorf("start ray develop: %w", err)
	}
	found := make(chan struct{}, 1)
	go pump(pr, stdout, func() { found <- struct{}{} })
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		pw.Close()
		done <- err
	}()
	stop := func() {
		_ = cmd.Process.Signal(syscall.SIGINT)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}
	select {
	case <-found:
		stop()
		return nil
	case err := <-done:
		return fmt.Errorf("ray develop exited before import completed: %w", err)
	case <-time.After(importTimeout):
		stop()
		return fmt.Errorf("timeout: build marker not seen within %s (is Raycast.app running?)", importTimeout)
	}
}
