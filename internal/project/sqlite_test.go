package project

import (
	"strings"
	"testing"
)

func TestLoad_SQLiteEngine(t *testing.T) {
	p := writeTOML(t, `
[db]
engine = "sqlite"
path = "dev.db"

[service.api]
command = ["a"]
db = true
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.DB.Engine != "sqlite" || c.DB.Path != "dev.db" {
		t.Fatalf("db = %+v", c.DB)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid sqlite config rejected: %v", err)
	}
}

func TestValidate_SQLiteRules(t *testing.T) {
	cases := map[string]struct {
		db   DB
		want string // substring of the expected error; "" = valid
	}{
		"path required":       {DB{Engine: "sqlite"}, "path"},
		"absolute path":       {DB{Engine: "sqlite", Path: "/tmp/x.db"}, "relative"},
		"escaping path":       {DB{Engine: "sqlite", Path: "../x.db"}, "relative"},
		"postgres field set":  {DB{Engine: "sqlite", Path: "dev.db", Container: "c"}, "container"},
		"dsn set":             {DB{Engine: "sqlite", Path: "dev.db", Dsn: "postgres://x"}, "dsn"},
		"unknown engine":      {DB{Engine: "mysql", Container: "c", Dsn: "d"}, "engine"},
		"ok nested path":      {DB{Engine: "sqlite", Path: "data/dev.db"}, ""},
		"postgres with path":  {DB{Container: "c", Dsn: "d", Path: "dev.db"}, "path"},
		"postgres unaffected": {DB{Container: "c", Dsn: "d"}, ""},
	}
	for name, tc := range cases {
		db := tc.db
		c := &Config{DB: &db, Services: map[string]Service{}}
		err := c.Validate()
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", name, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want error containing %q, got %v", name, tc.want, err)
		}
	}
}
