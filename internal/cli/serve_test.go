package cli

import (
	"strings"
	"testing"

	"github.com/agarichan/dx/internal/config"
	"github.com/agarichan/dx/internal/portless"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/worktree"
)

func TestBuildEnv_ForcesColor(t *testing.T) {
	cli := portless.Client{R: config.Load(func(string) string { return "" })}
	wt := &worktree.Info{Toplevel: "/x", Branch: "main", IsPrimary: true}
	cfg := &project.Config{Services: map[string]project.Service{}}
	svc := project.Service{Name: "svc"}
	env := buildEnv([]string{}, cfg, svc, wt, cli)
	m := envMap(env)
	if m["FORCE_COLOR"] != "1" {
		t.Fatalf("FORCE_COLOR not set: env=%v", env)
	}
	if m["CLICOLOR_FORCE"] != "1" {
		t.Fatalf("CLICOLOR_FORCE not set: env=%v", env)
	}
}

func TestBuildEnv_InjectsURLs(t *testing.T) {
	routing := map[string]string{
		"DX_PUBLIC_DOMAIN":   "dev.example.com",
		"DX_INTERNAL_DOMAIN": "lan",
		"DX_PROXY_PORT":      "1355",
	}
	cli := portless.Client{R: config.Load(func(k string) string { return routing[k] })}
	wt := &worktree.Info{Toplevel: "/x", Branch: "ws-cat", IsPrimary: false}
	parent := []string{"APP_DATABASE_URL=postgresql+asyncpg://myapp:pw@localhost:5434/myapp"}
	cfg := &project.Config{
		Services: map[string]project.Service{
			"api": {Name: "myapp-api"},
			"web": {Name: "myapp"},
		},
	}
	// Current service is "api" (myapp-api); references "web" key and "self".
	svc := project.Service{
		Name:     "myapp-api",
		Pub:      map[string]string{"APP_API_URL": "self", "APP_WEBAPP_URL": "web"},
		Internal: map[string]string{"VITE_API_URL": "api"},
	}
	env := buildEnv(parent, cfg, svc, wt, cli)
	m := envMap(env)
	// "self" → svc.Name = "myapp-api" → portless name "myapp-api-ws-cat"
	if m["APP_API_URL"] != "https://myapp-api-ws-cat.dev.example.com" {
		t.Fatalf("self pub = %q", m["APP_API_URL"])
	}
	// "web" key → cfg.Services["web"].Name = "myapp" → portless name "myapp-ws-cat"
	if m["APP_WEBAPP_URL"] != "https://myapp-ws-cat.dev.example.com" {
		t.Fatalf("pub web = %q", m["APP_WEBAPP_URL"])
	}
	// "api" key → cfg.Services["api"].Name = "myapp-api" → portless name "myapp-api-ws-cat"
	if m["VITE_API_URL"] != "https://myapp-api-ws-cat.lan:1355" {
		t.Fatalf("internal api = %q", m["VITE_API_URL"])
	}
	// DB env must NOT be rewritten by buildEnv — child inherits the mise-set env as-is
	if m["APP_DATABASE_URL"] != "postgresql+asyncpg://myapp:pw@localhost:5434/myapp" {
		t.Fatalf("db url should be unchanged = %q", m["APP_DATABASE_URL"])
	}
}

func TestServiceDir(t *testing.T) {
	cfg := &project.Config{Root: "/repo"}
	if got := serviceDir(cfg, project.Service{Dir: "api"}); got != "/repo/api" {
		t.Fatalf("dir=api => %q", got)
	}
	if got := serviceDir(cfg, project.Service{}); got != "/repo" {
		t.Fatalf("empty dir => %q", got)
	}
}

func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i >= 0 {
			m[e[:i]] = e[i+1:]
		}
	}
	return m
}
