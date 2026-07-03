package db

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/dburl"
)

func dockerRunner(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func TestForkAndDrop_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires docker")
	}
	d, _ := dburl.Parse("postgresql://myapp:devpassword@localhost:5434/myapp")
	c := Container{Name: "myapp-postgres", Image: "postgres:18", Volume: "myapp-pgdata-test", DSN: d}
	if err := c.Up(dockerRunner); err != nil {
		t.Skipf("cannot start container: %v", err)
	}
	// give postgres a moment is omitted; Up should retry readiness in real impl if needed
	if err := c.Fork(dockerRunner, "myapp", "myapp_dxtest"); err != nil {
		t.Fatalf("fork: %v", err)
	}
	ok, err := c.Exists(dockerRunner, "myapp_dxtest")
	if err != nil || !ok {
		t.Fatalf("exists after fork: ok=%v err=%v", ok, err)
	}
	if err := c.Drop(dockerRunner, "myapp_dxtest"); err != nil {
		t.Fatalf("drop: %v", err)
	}
}
