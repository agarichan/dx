package project

import (
	"strings"
	"testing"
)

func TestLoad_PopulatesKeyAndOpen(t *testing.T) {
	p := writeTOML(t, `
[service.api]
command = ["a"]

[service.web]
command = ["w"]
open = true
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	api, web := c.Services["api"], c.Services["web"]
	if api.Key != "api" || web.Key != "web" {
		t.Errorf("keys = %q %q", api.Key, web.Key)
	}
	if api.Open || !web.Open {
		t.Errorf("open: api=%v web=%v", api.Open, web.Open)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("single open must validate: %v", err)
	}
}

func TestValidate_RejectsMultipleOpen(t *testing.T) {
	p := writeTOML(t, `
[service.api]
command = ["a"]
open = true

[service.web]
command = ["w"]
open = true
`)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Validate()
	if err == nil || !strings.Contains(err.Error(), "open") {
		t.Fatalf("want multiple-open validation error, got %v", err)
	}
}
