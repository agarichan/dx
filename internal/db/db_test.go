package db

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/dburl"
)

func TestFork_CleanupOnCopyFailure(t *testing.T) {
	c := container()
	var calls []string
	run := func(name string, args ...string) (string, error) {
		joined := strings.Join(append([]string{name}, args...), " ")
		calls = append(calls, joined)
		if strings.Contains(joined, "pg_dump") {
			return "", fmt.Errorf("copy boom")
		}
		return "", nil // Exists absent, CREATE ok, DROP cleanup ok
	}
	if err := c.Fork(run, "myapp", "myapp_featx"); err == nil {
		t.Fatal("expected error when the pg_dump|psql copy fails")
	}
	combined := strings.Join(calls, "\n")
	if !strings.Contains(combined, `DROP DATABASE IF EXISTS "myapp_featx"`) {
		t.Errorf("must drop the half-copied target on copy failure, got:\n%s", combined)
	}
}

func container() Container {
	d, _ := dburl.Parse("postgresql+asyncpg://myapp:devpassword@localhost:5434/myapp")
	return Container{Name: "myapp-postgres", Image: "postgres:18", Volume: "myapp-pgdata", DSN: d}
}

func TestRunArgs(t *testing.T) {
	a := strings.Join(container().RunArgs(), " ")
	for _, want := range []string{
		"run -d --name myapp-postgres",
		"-e POSTGRES_USER=myapp",
		"-e POSTGRES_PASSWORD=devpassword",
		"-e POSTGRES_DB=myapp",
		"-p 5434:5432",
		"-v myapp-pgdata:/var/lib/postgresql",
		"postgres:18",
	} {
		if !strings.Contains(a, want) {
			t.Errorf("RunArgs missing %q in %q", want, a)
		}
	}
}

func TestExecArgs(t *testing.T) {
	a := container().ExecArgs("postgres", "SELECT 1")
	joined := strings.Join(a, " ")
	if !strings.Contains(joined, "exec -e PGPASSWORD=devpassword myapp-postgres psql -U myapp -d postgres -tAc") {
		t.Fatalf("ExecArgs = %q", joined)
	}
	if a[len(a)-1] != "SELECT 1" {
		t.Fatalf("last arg = %q", a[len(a)-1])
	}
}

func TestSQLBuilders(t *testing.T) {
	if !strings.Contains(TerminateSQL("myapp"), "datname = 'myapp'") {
		t.Error("TerminateSQL")
	}
	if CreateDatabaseSQL("myapp_featx") != `CREATE DATABASE "myapp_featx";` {
		t.Errorf("CreateDatabaseSQL = %q", CreateDatabaseSQL("myapp_featx"))
	}
	if DropSQL("myapp_featx") != `DROP DATABASE IF EXISTS "myapp_featx";` {
		t.Error("DropSQL")
	}
	if !strings.Contains(ListSQL("myapp"), "myapp") {
		t.Error("ListSQL")
	}
}

func TestDumpRestoreArgs(t *testing.T) {
	c := container()
	args := c.DumpRestoreArgs("myapp", "myapp_featx")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-e PGPASSWORD=devpassword",
		"myapp-postgres",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("DumpRestoreArgs missing %q in %q", want, joined)
		}
	}
	// structure: exec -e PGPASSWORD=... <name> sh -c <script>
	// args[0]="exec" [1]="-e" [2]="PGPASSWORD=..." [3]=<name> [4]="sh" [5]="-c" [6]=<script>
	if args[4] != "sh" {
		t.Errorf("args[4] = %q, want \"sh\"", args[4])
	}
	if args[5] != "-c" {
		t.Errorf("args[5] = %q, want \"-c\"", args[5])
	}
	// last arg is the pipe script
	script := args[len(args)-1]
	if !strings.Contains(script, "pg_dump -U myapp -d myapp") {
		t.Errorf("script missing pg_dump clause: %q", script)
	}
	if !strings.Contains(script, "psql -q -U myapp -d myapp_featx") {
		t.Errorf("script missing psql clause: %q", script)
	}
}

func TestFork_DumpRestore(t *testing.T) {
	c := container()
	var calls []string
	run := func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return "", nil
	}
	if err := c.Fork(run, "myapp", "myapp_featx"); err != nil {
		t.Fatalf("Fork error: %v", err)
	}
	combined := strings.Join(calls, "\n")
	if strings.Contains(combined, "TEMPLATE") {
		t.Errorf("Fork must not use TEMPLATE, got:\n%s", combined)
	}
	if !strings.Contains(combined, `CREATE DATABASE "myapp_featx"`) {
		t.Errorf("Fork must call CREATE DATABASE \"myapp_featx\", got:\n%s", combined)
	}
	if !strings.Contains(combined, "pg_dump") {
		t.Errorf("Fork must call pg_dump, got:\n%s", combined)
	}
}

func TestFork_Idempotent(t *testing.T) {
	c := container()
	callCount := 0
	var extraCalls []string
	run := func(name string, args ...string) (string, error) {
		callCount++
		joined := strings.Join(append([]string{name}, args...), " ")
		if callCount == 1 {
			// First call is the Exists query — return "1" (target present)
			return "1", nil
		}
		extraCalls = append(extraCalls, joined)
		return "", nil
	}
	if err := c.Fork(run, "myapp", "myapp_featx"); err != nil {
		t.Fatalf("Fork error: %v", err)
	}
	for _, call := range extraCalls {
		if strings.Contains(call, "CREATE DATABASE") || strings.Contains(call, "pg_dump") {
			t.Errorf("idempotent Fork must not create/copy when target exists, got: %q", call)
		}
	}
}
