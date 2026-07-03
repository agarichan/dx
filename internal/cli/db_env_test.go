package cli

import (
	"testing"

	"github.com/agarichan/dx/internal/config"
	"github.com/agarichan/dx/internal/portless"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

func TestBuildEnv_InjectsDBEnv_Postgres(t *testing.T) {
	cli := portless.Client{R: config.Load(func(string) string { return "" })}
	wt := &worktree.Info{Toplevel: "/wt", Branch: "feat-x", IsPrimary: false, PrimaryRoot: "/repo"}
	cfg := &project.Config{
		DB:       &project.DB{Container: "c", Dsn: "postgres://u:p@localhost:5432/app"},
		Services: map[string]project.Service{},
	}
	svc := project.Service{
		Name:  "api",
		DBEnv: &project.DBEnv{Name: "APP_DATABASE_URL", Scheme: "postgresql+psycopg"},
	}
	env, err := buildEnv([]string{}, cfg, svc, wt, cli)
	if err != nil {
		t.Fatal(err)
	}
	m := envMap(env)
	// worktree DB name + overridden scheme
	if m["APP_DATABASE_URL"] != "postgresql+psycopg://u:p@localhost:5432/app_feat_x" {
		t.Fatalf("APP_DATABASE_URL = %q", m["APP_DATABASE_URL"])
	}
}

func TestBuildEnv_InjectsDBEnv_PostgresPrimaryDefaultScheme(t *testing.T) {
	cli := portless.Client{R: config.Load(func(string) string { return "" })}
	wt := &worktree.Info{Toplevel: "/repo", Branch: "main", IsPrimary: true, PrimaryRoot: "/repo"}
	cfg := &project.Config{
		DB:       &project.DB{Container: "c", Dsn: "postgresql+asyncpg://u:p@localhost:5432/app"},
		Services: map[string]project.Service{},
	}
	svc := project.Service{Name: "api", DBEnv: &project.DBEnv{Name: "APP_DATABASE_URL"}}
	env, err := buildEnv([]string{}, cfg, svc, wt, cli)
	if err != nil {
		t.Fatal(err)
	}
	m := envMap(env)
	// primary: base name unchanged; scheme untouched when omitted
	if m["APP_DATABASE_URL"] != "postgresql+asyncpg://u:p@localhost:5432/app" {
		t.Fatalf("APP_DATABASE_URL = %q", m["APP_DATABASE_URL"])
	}
}

func TestBuildEnv_InjectsDBEnv_SQLite(t *testing.T) {
	cli := portless.Client{R: config.Load(func(string) string { return "" })}
	wt := &worktree.Info{Toplevel: "/wt", Branch: "feat-x", IsPrimary: false, PrimaryRoot: "/repo"}
	cfg := &project.Config{
		DB:       &project.DB{Engine: "sqlite", Path: "dev.db"},
		Services: map[string]project.Service{},
	}
	svc := project.Service{Name: "api", DBEnv: &project.DBEnv{Name: "APP_DATABASE_URL", Scheme: "sqlite+aiosqlite"}}
	env, err := buildEnv([]string{}, cfg, svc, wt, cli)
	if err != nil {
		t.Fatal(err)
	}
	m := envMap(env)
	if m["APP_DATABASE_URL"] != "sqlite+aiosqlite:////wt/dev.db" {
		t.Fatalf("APP_DATABASE_URL = %q", m["APP_DATABASE_URL"])
	}
}
