package project

import (
	"strings"
	"testing"
)

func TestLoad_DBEnv(t *testing.T) {
	p := writeTOML(t, `
[db]
container = "c"
dsn = "postgres://u:p@h:5432/app"

[service.api]
command = ["a"]
db_env  = { name = "APP_DATABASE_URL", scheme = "postgresql+psycopg" }
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	e := c.Services["api"].DBEnv
	if e == nil || e.Name != "APP_DATABASE_URL" || e.Scheme != "postgresql+psycopg" {
		t.Fatalf("db_env = %+v", e)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid db_env rejected: %v", err)
	}
}

func TestValidate_DBEnvRules(t *testing.T) {
	// name required
	c := &Config{
		DB:       &DB{Container: "c", Dsn: "postgres://u@h/app"},
		Services: map[string]Service{"api": {Command: []string{"a"}, DBEnv: &DBEnv{Scheme: "x"}}},
	}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("want name-required error, got %v", err)
	}
	// requires [db]
	c = &Config{
		Services: map[string]Service{"api": {Command: []string{"a"}, DBEnv: &DBEnv{Name: "X"}}},
	}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "[db]") {
		t.Fatalf("want requires-db error, got %v", err)
	}
}
