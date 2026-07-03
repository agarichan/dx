package portless

import (
	"testing"

	"github.com/agarichan/dx/internal/config"
)

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"ws-category": "ws-category",
		"Feat/Foo":    "feat-foo",
		"--a__b--":    "a-b",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSvcName(t *testing.T) {
	if got := SvcName("myapp-api", "main", true); got != "myapp-api" {
		t.Errorf("primary = %q", got)
	}
	if got := SvcName("myapp-api", "ws-category", false); got != "myapp-api-ws-category" {
		t.Errorf("worktree = %q", got)
	}
}

func TestSvcName_Cap63(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += "x"
	}
	got := SvcName("myapp-api", long, false)
	if len(got) > 63 {
		t.Fatalf("len = %d: %q", len(got), got)
	}
}

func TestURLs_Defaults(t *testing.T) {
	// zero config: proxy on 443 -> port omitted; no public domain -> pub falls back to internal
	c := Client{R: config.Load(func(string) string { return "" })}
	if got := c.URLInternal("myapp-api"); got != "https://myapp-api.localhost" {
		t.Errorf("internal = %q", got)
	}
	if got := c.URLPublic("myapp-api"); got != "https://myapp-api.localhost" {
		t.Errorf("public = %q", got)
	}
}

func TestURLs_CustomRouting(t *testing.T) {
	env := map[string]string{
		"DX_PUBLIC_DOMAIN":   "dev.example.com",
		"DX_INTERNAL_DOMAIN": "lan",
		"DX_PROXY_PORT":      "1355",
	}
	c := Client{R: config.Load(func(k string) string { return env[k] })}
	if got := c.URLPublic("myapp-api"); got != "https://myapp-api.dev.example.com" {
		t.Errorf("public = %q", got)
	}
	if got := c.URLInternal("myapp-api"); got != "https://myapp-api.lan:1355" {
		t.Errorf("internal = %q", got)
	}
}

func TestURLInternal_ExplicitDefaultPort(t *testing.T) {
	// explicitly setting 443 also omits the port suffix
	env := map[string]string{"DX_PROXY_PORT": "443"}
	c := Client{R: config.Load(func(k string) string { return env[k] })}
	if got := c.URLInternal("app"); got != "https://app.localhost" {
		t.Errorf("internal = %q", got)
	}
}

func TestServeArgv(t *testing.T) {
	got := ServeArgv("myapp", 4321, []string{"pnpm", "dev"})
	want := []string{"myapp", "--app-port", "4321", "--", "pnpm", "dev"}
	if len(got) != len(want) {
		t.Fatalf("argv = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv = %v", got)
		}
	}
}

func TestSubstitutePort(t *testing.T) {
	got := SubstitutePort([]string{"uvicorn", "--port", "{port}", "--host", "127.0.0.1"}, 5050)
	if got[2] != "5050" {
		t.Fatalf("substitute = %v", got)
	}
	// original token-free args untouched
	if got[0] != "uvicorn" || got[3] != "--host" {
		t.Fatalf("unexpected mutation: %v", got)
	}
}

func TestFreePort(t *testing.T) {
	p, err := FreePort()
	if err != nil {
		t.Fatal(err)
	}
	if p <= 0 || p > 65535 {
		t.Fatalf("port out of range: %d", p)
	}
}
