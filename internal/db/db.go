package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/agarichan/dx/internal/dburl"
)

type Container struct {
	Name   string
	Image  string
	Volume string
	DSN    *dburl.DSN
}

// RunArgs builds `docker run` args for the postgres container.
func (c Container) RunArgs() []string {
	return []string{
		"run", "-d", "--name", c.Name,
		"-e", "POSTGRES_USER=" + c.DSN.User,
		"-e", "POSTGRES_PASSWORD=" + c.DSN.Password,
		"-e", "POSTGRES_DB=" + c.DSN.Name,
		"-p", c.DSN.Port + ":5432",
		"-v", c.Volume + ":/var/lib/postgresql",
		c.Image,
	}
}

// ExecArgs builds `docker exec` args to run a single SQL statement via psql.
func (c Container) ExecArgs(maintDB, sql string) []string {
	return []string{
		"exec", "-e", "PGPASSWORD=" + c.DSN.Password, c.Name,
		"psql", "-U", c.DSN.User, "-d", maintDB, "-tAc", sql,
	}
}

// PsqlArgs builds an interactive `docker exec -it ... psql` invocation.
func (c Container) PsqlArgs(dbname string) []string {
	return []string{
		"exec", "-it", "-e", "PGPASSWORD=" + c.DSN.Password, c.Name,
		"psql", "-U", c.DSN.User, "-d", dbname,
	}
}

func TerminateSQL(base string) string {
	return fmt.Sprintf(
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid();",
		base)
}

// CreateDatabaseSQL creates an empty database (copies template1, which has no
// app connections — so it does NOT lock base, unlike a TEMPLATE base copy).
func CreateDatabaseSQL(target string) string {
	return fmt.Sprintf(`CREATE DATABASE "%s";`, target)
}

func DropSQL(target string) string {
	return fmt.Sprintf(`DROP DATABASE IF EXISTS "%s";`, target)
}

// ListSQL lists databases whose name equals base or starts with base_.
func ListSQL(base string) string {
	return fmt.Sprintf(
		"SELECT datname, pg_size_pretty(pg_database_size(datname)) FROM pg_database "+
			"WHERE datname = '%s' OR datname LIKE '%s\\_%%' ORDER BY datname;",
		base, base)
}

// Runner is a function that executes a command and returns combined output.
type Runner func(name string, args ...string) (string, error)

// DumpRestoreArgs builds `docker exec ... sh -c "pg_dump -d base | psql -d target"`.
// pg_dump takes a consistent read snapshot, so it works while base has active
// connections (no lock), unlike CREATE DATABASE ... TEMPLATE.
func (c Container) DumpRestoreArgs(base, target string) []string {
	pipe := fmt.Sprintf("pg_dump -U %s -d %s | psql -q -U %s -d %s",
		c.DSN.User, base, c.DSN.User, target)
	return []string{
		"exec", "-e", "PGPASSWORD=" + c.DSN.Password, c.Name,
		"sh", "-c", pipe,
	}
}

// exec runs a SQL statement inside the container via psql.
func (c Container) exec(run Runner, maintDB, sql string) (string, error) {
	return run("docker", c.ExecArgs(maintDB, sql)...)
}

// waitReady polls postgres inside the container until it accepts connections,
// with 0.5s interval and up to 20 tries. Returns nil on first success.
func (c Container) waitReady(run Runner) error {
	for i := 0; i < 20; i++ {
		if _, err := c.exec(run, "postgres", "SELECT 1"); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("postgres in container %s did not become ready", c.Name)
}

// Up starts the container (docker start), falling back to docker run if not found.
// After the container starts it waits for postgres to be ready.
func (c Container) Up(run Runner) error {
	if _, err := run("docker", "start", c.Name); err == nil {
		return c.waitReady(run)
	}
	if _, err := run("docker", c.RunArgs()...); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}
	if err := c.waitReady(run); err != nil {
		return err
	}
	return nil
}

// Down stops and removes the container.
func (c Container) Down(run Runner) error {
	_, _ = run("docker", "stop", c.Name)
	_, err := run("docker", "rm", c.Name)
	if err != nil {
		return fmt.Errorf("docker rm %s: %w", c.Name, err)
	}
	return nil
}

// Exists reports whether the named database exists in the container.
func (c Container) Exists(run Runner, dbname string) (bool, error) {
	out, err := c.exec(run, "postgres",
		fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s';", dbname))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "1", nil
}

// Fork creates target as a copy of base. Idempotent: no-op if target already exists.
// Uses pg_dump|psql instead of CREATE DATABASE ... TEMPLATE so that active
// connections to base do not block the copy.
func (c Container) Fork(run Runner, base, target string) error {
	ok, err := c.Exists(run, target)
	if err != nil {
		return err
	}
	if ok {
		return nil // idempotent
	}
	if _, err := c.exec(run, "postgres", CreateDatabaseSQL(target)); err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := run("docker", c.DumpRestoreArgs(base, target)...); err != nil {
		// Drop the half-copied target so a retry re-forks instead of the
		// idempotent Exists check silently accepting a partial/empty DB.
		_, _ = c.exec(run, "postgres", DropSQL(target))
		return fmt.Errorf("copy %s -> %s: %w", base, target, err)
	}
	return nil
}

// Drop terminates connections to target and drops the database.
func (c Container) Drop(run Runner, target string) error {
	if _, err := c.exec(run, "postgres", TerminateSQL(target)); err != nil {
		return fmt.Errorf("terminate %s: %w", target, err)
	}
	_, err := c.exec(run, "postgres", DropSQL(target))
	if err != nil {
		return fmt.Errorf("drop %s: %w", target, err)
	}
	return nil
}

// List returns names and sizes of databases derived from base.
func (c Container) List(run Runner, base string) (string, error) {
	return c.exec(run, "postgres", ListSQL(base))
}
