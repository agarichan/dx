package registry

import (
	"testing"
)

func TestPutGetListRemove(t *testing.T) {
	dir := t.TempDir()
	r, err := Open(dir, "/home/u/work/myapp")
	if err != nil {
		t.Fatal(err)
	}
	svc := Service{
		Name:    "myapp-api",
		PID:     4242,
		LogPath: "/tmp/myapp-api.log",
		Dir:     "/home/u/work/myapp/api",
		Command: []string{"uvicorn", "app:app"},
		Pub:     map[string]string{"APP_API_URL": "self"},
		DB:      "APP_DATABASE_URL",
	}
	if err := r.Put(svc); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.Get("myapp-api")
	if err != nil || !ok {
		t.Fatalf("Get ok=%v err=%v", ok, err)
	}
	if got.PID != 4242 || got.Pub["APP_API_URL"] != "self" {
		t.Fatalf("got = %+v", got)
	}
	list, err := r.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v err=%v", list, err)
	}
	if err := r.Remove("myapp-api"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = r.Get("myapp-api")
	if ok {
		t.Fatal("expected removed")
	}
}

func TestStateRoot(t *testing.T) {
	got := StateRoot(func(k string) string {
		if k == "XDG_STATE_HOME" {
			return "/xdg/state"
		}
		return ""
	})
	if got != "/xdg/state/dx" {
		t.Fatalf("StateRoot = %q", got)
	}
}

func TestRootURLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r, _ := Open(dir, "/home/u/work/myapp")
	if err := r.Put(Service{Name: "myapp-api", PID: 1, Root: "/home/u/work/myapp", URL: "https://myapp-api.dev.example.com"}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.Get("myapp-api")
	if err != nil || !ok {
		t.Fatalf("get ok=%v err=%v", ok, err)
	}
	if got.Root != "/home/u/work/myapp" || got.URL != "https://myapp-api.dev.example.com" {
		t.Fatalf("root/url = %q / %q", got.Root, got.URL)
	}
}

func TestAll(t *testing.T) {
	root := t.TempDir()
	ra, _ := Open(root, "/home/u/work/myapp")
	rb, _ := Open(root, "/home/u/work/myapp/.claude/worktrees/x")
	ra.Put(Service{Name: "myapp", PID: 10})
	ra.Put(Service{Name: "myapp-api", PID: 11})
	rb.Put(Service{Name: "myapp-x", PID: 12})

	all, err := All(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("All len = %d, want 3", len(all))
	}
	// 各 Located は Remove 可能な Registry を持つ
	names := map[string]bool{}
	for _, l := range all {
		names[l.Service.Name] = true
		if l.Registry == nil {
			t.Fatalf("nil registry for %s", l.Service.Name)
		}
	}
	for _, n := range []string{"myapp", "myapp-api", "myapp-x"} {
		if !names[n] {
			t.Fatalf("missing %s in %v", n, names)
		}
	}
}

func TestAll_MissingRoot(t *testing.T) {
	all, err := All(t.TempDir() + "/does-not-exist")
	if err != nil {
		t.Fatalf("All on missing root should not error: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("want empty, got %d", len(all))
	}
}
