package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "dx.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_FullConfig(t *testing.T) {
	p := writeTOML(t, `
[worktree]
dir = ".claude/worktrees"

[db]
container = "myapp-postgres"
dsn = "postgresql+asyncpg://myapp:pw@localhost:5434/myapp"
url_env = "APP_DATABASE_URL"
volume = "myapp-pgdata"

[service.api]
name = "myapp-api"
command = ["uvicorn", "app:app", "--port", "{port}"]
db = true
dir = "api"
pub = { APP_API_URL = "self", APP_WEBAPP_URL = "web" }

[service.web]
name = "myapp"
command = ["vite", "--port", "{port}"]
internal = { VITE_API_URL = "api" }
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Worktree.Dir != ".claude/worktrees" {
		t.Fatalf("dir = %q", c.Worktree.Dir)
	}
	if c.DB == nil || c.DB.Container != "myapp-postgres" || c.DB.URLEnv != "APP_DATABASE_URL" {
		t.Fatalf("db = %+v", c.DB)
	}
	if c.DB.Dsn != "postgresql+asyncpg://myapp:pw@localhost:5434/myapp" {
		t.Fatalf("db.dsn = %q", c.DB.Dsn)
	}
	if c.DB.Image != "postgres:18" {
		t.Fatalf("image default = %q", c.DB.Image)
	}
	if len(c.Services) != 2 {
		t.Fatalf("services = %d", len(c.Services))
	}
	api, ok := c.Services["api"]
	if !ok {
		t.Fatal("missing service key \"api\"")
	}
	if api.Name != "myapp-api" || !api.DB || len(api.Command) != 4 {
		t.Fatalf("api = %+v", api)
	}
	if api.Dir != "api" {
		t.Fatalf("dir = %q", api.Dir)
	}
	if api.Pub["APP_WEBAPP_URL"] != "web" {
		t.Fatalf("pub = %v", api.Pub)
	}
	web, ok := c.Services["web"]
	if !ok {
		t.Fatal("missing service key \"web\"")
	}
	if web.Internal["VITE_API_URL"] != "api" {
		t.Fatalf("internal = %v", web.Internal)
	}
	if c.Root != filepath.Dir(p) {
		t.Fatalf("root = %q", c.Root)
	}
}

func TestLoad_NoDBTable(t *testing.T) {
	p := writeTOML(t, `
[service.web]
name = "web"
command = ["vite"]
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.DB != nil {
		t.Fatalf("expected nil DB, got %+v", c.DB)
	}
	if c.Worktree.Dir != ".claude/worktrees" {
		t.Fatalf("default dir = %q", c.Worktree.Dir)
	}
}

func TestLoad_DefaultName(t *testing.T) {
	p := writeTOML(t, `
[service.myapi]
command = ["go", "run", "."]
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	svc, ok := c.Services["myapi"]
	if !ok {
		t.Fatal("missing service key \"myapi\"")
	}
	if svc.Name != "myapi" {
		t.Fatalf("default name should be key \"myapi\", got %q", svc.Name)
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "dx.toml"))
	if err == nil {
		t.Fatal("expected error for missing dx.toml")
	}
}

func TestValidate_OK(t *testing.T) {
	c := &Config{
		DB:       &DB{Container: "c", URLEnv: "U"},
		Services: map[string]Service{"api": {Name: "api", Command: []string{"x"}, DB: true}},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_DBMissingFields(t *testing.T) {
	c := &Config{DB: &DB{Container: "c"}} // dsn も url_env も欠落
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing dsn and url_env")
	}
}

func TestValidate_DsnOrURLEnv(t *testing.T) {
	// neither dsn nor url_env → error
	c := &Config{DB: &DB{Container: "c"}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error: neither dsn nor url_env")
	}

	// only dsn → ok
	c = &Config{DB: &DB{Container: "c", Dsn: "postgresql://myapp:pw@localhost:5432/myapp"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error with dsn only: %v", err)
	}

	// only url_env → ok
	c = &Config{DB: &DB{Container: "c", URLEnv: "APP_DATABASE_URL"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error with url_env only: %v", err)
	}
}

func TestValidate_ServiceDBWithoutDBTable(t *testing.T) {
	c := &Config{Services: map[string]Service{"api": {Name: "api", Command: []string{"x"}, DB: true}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error: db=true but no [db]")
	}
}

func TestValidate_ServiceMissingCommand(t *testing.T) {
	c := &Config{Services: map[string]Service{"api": {Name: "api"}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestValidate_InitEmptyCommand(t *testing.T) {
	c := &Config{Worktree: Worktree{Init: []InitStep{{Command: nil}}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty init command")
	}
}

func TestValidate_InitDirEscapes(t *testing.T) {
	c := &Config{Worktree: Worktree{Init: []InitStep{
		{Command: []string{"ls"}, Dir: "../escape"},
	}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for init dir escaping worktree")
	}
}

func TestValidate_InitDirAbsolute(t *testing.T) {
	c := &Config{Worktree: Worktree{Init: []InitStep{
		{Command: []string{"ls"}, Dir: "/opt/app"},
	}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for absolute init dir")
	}
}

func TestValidate_InitOK(t *testing.T) {
	c := &Config{Worktree: Worktree{Init: []InitStep{
		{Command: []string{"pnpm", "install"}, Dir: "webapp"},
		{Command: []string{"cp", ".env.local", ".env"}},
	}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_CopyEmptyFrom(t *testing.T) {
	c := &Config{Worktree: Worktree{Copy: []CopyStep{{From: ""}}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for empty copy from")
	}
}

func TestValidate_CopyFromEscapes(t *testing.T) {
	c := &Config{Worktree: Worktree{Copy: []CopyStep{{From: "../secrets"}}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for copy from escaping worktree")
	}
}

func TestValidate_CopyFromAbsolute(t *testing.T) {
	c := &Config{Worktree: Worktree{Copy: []CopyStep{{From: "/etc/passwd"}}}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for absolute copy from")
	}
}

func TestValidate_CopyOK(t *testing.T) {
	c := &Config{Worktree: Worktree{Copy: []CopyStep{
		{From: ".myapp"},
		{From: ".env.local"},
	}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestLoad_WorktreeCopy(t *testing.T) {
	p := writeTOML(t, `
[[worktree.copy]]
from = ".myapp"

[[worktree.copy]]
from = ".env.local"
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Worktree.Copy) != 2 {
		t.Fatalf("copy len = %d", len(c.Worktree.Copy))
	}
	if c.Worktree.Copy[0].From != ".myapp" || c.Worktree.Copy[1].From != ".env.local" {
		t.Fatalf("copy = %+v", c.Worktree.Copy)
	}
}

func TestLoad_WorktreeInit(t *testing.T) {
	p := writeTOML(t, `
[[worktree.init]]
command = ["pnpm", "install"]
dir = "webapp"

[[worktree.init]]
command = ["cp", ".env.local", ".env"]
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Worktree.Init) != 2 {
		t.Fatalf("init len = %d", len(c.Worktree.Init))
	}
	if c.Worktree.Init[0].Dir != "webapp" || len(c.Worktree.Init[0].Command) != 2 {
		t.Fatalf("init[0] = %+v", c.Worktree.Init[0])
	}
	if c.Worktree.Init[1].Dir != "" || c.Worktree.Init[1].Command[0] != "cp" {
		t.Fatalf("init[1] = %+v", c.Worktree.Init[1])
	}
}

func TestService_Lookup(t *testing.T) {
	c := &Config{Services: map[string]Service{"api": {Name: "myapp-api"}, "web": {Name: "myapp"}}}
	s, ok := c.Service("web")
	if !ok || s.Name != "myapp" {
		t.Fatalf("lookup = %+v ok=%v", s, ok)
	}
	if _, ok := c.Service("nope"); ok {
		t.Fatal("expected miss")
	}
}

func TestServiceKeys_Sorted(t *testing.T) {
	c := &Config{
		Services: map[string]Service{
			"web": {Name: "myapp"},
			"api": {Name: "myapp-api"},
			"db":  {},
		},
	}
	keys := c.ServiceKeys()
	if len(keys) != 3 || keys[0] != "api" || keys[1] != "db" || keys[2] != "web" {
		t.Fatalf("keys = %v", keys)
	}
}
