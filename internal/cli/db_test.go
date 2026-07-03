package cli

import (
	"testing"

	"github.com/agarichan/dx/internal/project"
)

func TestContainerFor(t *testing.T) {
	// env fallback: Dsn empty, URLEnv set
	t.Setenv("APP_DATABASE_URL", "postgresql+asyncpg://myapp:pw@localhost:5434/myapp")
	cfg := &project.Config{DB: &project.DB{
		Container: "myapp-postgres", URLEnv: "APP_DATABASE_URL", Image: "postgres:18", Volume: "myapp-pgdata",
	}}
	c, base, err := containerFor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "myapp-postgres" || c.Image != "postgres:18" || c.Volume != "myapp-pgdata" {
		t.Fatalf("container = %+v", c)
	}
	if base != "myapp" {
		t.Fatalf("base = %q", base)
	}
}

func TestContainerFor_NoDB(t *testing.T) {
	if _, _, err := containerFor(&project.Config{}); err == nil {
		t.Fatal("expected error when [db] missing")
	}
}

func TestContainerFor_URLEnvUnset(t *testing.T) {
	// both Dsn and URLEnv env are empty → error
	cfg := &project.Config{DB: &project.DB{Container: "c", URLEnv: "DEFINITELY_UNSET_XYZ"}}
	if _, _, err := containerFor(cfg); err == nil {
		t.Fatal("expected error when neither dsn nor url_env set")
	}
}

func TestBaseDSN(t *testing.T) {
	// dsn field takes precedence over env var
	t.Setenv("APP_DATABASE_URL", "postgresql+asyncpg://myapp:pw@localhost:5434/from-env")
	cfg := &project.Config{DB: &project.DB{
		Container: "myapp-postgres",
		Dsn:       "postgresql+asyncpg://myapp:pw@localhost:5434/from-dsn",
		URLEnv:    "APP_DATABASE_URL",
	}}
	dsn, err := baseDSN(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if dsn.Name != "from-dsn" {
		t.Fatalf("expected dsn field to win; got %q", dsn.Name)
	}

	// env fallback when Dsn is empty
	cfg2 := &project.Config{DB: &project.DB{
		Container: "myapp-postgres",
		URLEnv:    "APP_DATABASE_URL",
	}}
	dsn2, err := baseDSN(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	if dsn2.Name != "from-env" {
		t.Fatalf("expected env fallback; got %q", dsn2.Name)
	}

	// error when neither dsn nor url_env env is set
	cfg3 := &project.Config{DB: &project.DB{Container: "c", URLEnv: "DEFINITELY_UNSET_XYZ_DSN"}}
	if _, err := baseDSN(cfg3); err == nil {
		t.Fatal("expected error when neither dsn nor url_env set")
	}

	// error when [db] is nil
	if _, err := baseDSN(&project.Config{}); err == nil {
		t.Fatal("expected error when [db] missing")
	}
}
