package supervisor

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var testAnsiRE = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// TestMain lets this test binary stand in for the real `dx __logpump` pump.
// supervisor.Start re-execs os.Executable() with the __logpump subcommand; under
// `go test` os.Executable() is this binary, and supervisor cannot import the cli
// package (cli imports supervisor → cycle). So we emulate the fan-out here.
func TestMain(m *testing.M) {
	for i, a := range os.Args[1:] {
		if a == "__logpump" {
			os.Exit(testPump(os.Args[1:][i+1:]))
		}
	}
	os.Exit(m.Run())
}

func testPump(args []string) int {
	var plainPath, colorPath string
	var child []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--plain":
			i++
			plainPath = args[i]
		case "--color":
			i++
			colorPath = args[i]
		case "--":
			child = args[i+1:]
			i = len(args)
		}
	}
	if plainPath == "" || colorPath == "" || len(child) == 0 {
		return 2
	}
	pf, err := os.OpenFile(plainPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 1
	}
	defer pf.Close()
	cf, err := os.OpenFile(colorPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 1
	}
	defer cf.Close()
	cmd := exec.Command(child[0], child[1:]...)
	pr, err := cmd.StdoutPipe()
	if err != nil {
		return 1
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return 1
	}
	sc := bufio.NewScanner(pr)
	for sc.Scan() {
		line := sc.Text()
		_, _ = cf.WriteString(line + "\n")
		_, _ = pf.WriteString(testAnsiRE.ReplaceAllString(line, "") + "\n")
	}
	_ = cmd.Wait()
	return 0
}

func TestStartAliveStop(t *testing.T) {
	log := filepath.Join(t.TempDir(), "out.log")
	pid, err := Start(Spec{
		Command: []string{"/bin/sh", "-c", "echo hello; sleep 30"},
		LogPath: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !Alive(pid) {
		t.Fatal("expected alive")
	}
	// log should capture stdout
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(log)
		if len(b) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := os.ReadFile(log)
	if string(b) == "" {
		t.Fatal("expected log output")
	}
	if err := StopGroup(pid); err != nil {
		t.Fatal(err)
	}
	// process should die shortly
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && Alive(pid) {
		time.Sleep(50 * time.Millisecond)
	}
	if Alive(pid) {
		t.Fatal("expected process to terminate")
	}
}

func TestStart_TwoFileColorLog(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "out.log")
	color := filepath.Join(dir, "out.color.log")
	_, err := Start(Spec{
		Command:      []string{"/bin/sh", "-c", `printf "\033[31mhi\033[0m\n"`},
		LogPath:      plain,
		ColorLogPath: color,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The pump + short-lived child write both files asynchronously; poll.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cb, _ := os.ReadFile(color)
		pb, _ := os.ReadFile(plain)
		if len(cb) > 0 && len(pb) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cb, _ := os.ReadFile(color)
	pb, _ := os.ReadFile(plain)
	if !strings.Contains(string(cb), "\x1b[31m") {
		t.Fatalf("color log missing ESC: %q", cb)
	}
	if strings.Contains(string(pb), "\x1b") {
		t.Fatalf("plain log must not contain ESC: %q", pb)
	}
	if !strings.Contains(string(pb), "hi") {
		t.Fatalf("plain log missing %q: %q", "hi", pb)
	}
}
