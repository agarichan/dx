package config

import (
	"os"
	"path/filepath"
	"testing"
)

func noFiles(string) ([]byte, error) { return nil, os.ErrNotExist }

func stateFiles(m map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if v, ok := m[filepath.Base(path)]; ok {
			return []byte(v), nil
		}
		return nil, os.ErrNotExist
	}
}

func TestLoad_StaticDefaults(t *testing.T) {
	r := load(func(string) string { return "" }, noFiles)
	if r.PublicDomain != "" || r.InternalDomain != "localhost" || r.ProxyPort != "443" || r.Scheme != "https" {
		t.Fatalf("defaults = %+v", r)
	}
}

func TestLoad_PortlessStateAutodetect(t *testing.T) {
	read := stateFiles(map[string]string{
		"proxy.tld":  "lan\n",
		"proxy.port": "1355\n",
		"proxy.tls":  "1\n",
	})
	r := load(func(k string) string {
		return map[string]string{"HOME": "/Users/u"}[k]
	}, read)
	if r.InternalDomain != "lan" || r.ProxyPort != "1355" || r.Scheme != "https" {
		t.Fatalf("state autodetect = %+v", r)
	}
}

func TestLoad_NoTLSStateGivesHTTP(t *testing.T) {
	read := stateFiles(map[string]string{"proxy.tls": "0"})
	r := load(func(k string) string { return map[string]string{"HOME": "/u"}[k] }, read)
	if r.Scheme != "http" {
		t.Fatalf("scheme = %q", r.Scheme)
	}
}

func TestLoad_EnvWinsOverState(t *testing.T) {
	read := stateFiles(map[string]string{"proxy.tld": "lan", "proxy.port": "1355"})
	env := map[string]string{
		"HOME":               "/u",
		"DX_PUBLIC_DOMAIN":   "example.test",
		"DX_INTERNAL_DOMAIN": "custom",
		"DX_PROXY_PORT":      "8443",
	}
	r := load(func(k string) string { return env[k] }, read)
	if r.PublicDomain != "example.test" || r.InternalDomain != "custom" || r.ProxyPort != "8443" {
		t.Fatalf("override = %+v", r)
	}
}

func TestLoad_EmptyStateFileFallsThrough(t *testing.T) {
	read := stateFiles(map[string]string{"proxy.tld": "  \n", "proxy.port": ""})
	r := load(func(k string) string { return map[string]string{"HOME": "/u"}[k] }, read)
	if r.InternalDomain != "localhost" || r.ProxyPort != "443" {
		t.Fatalf("empty state = %+v", r)
	}
}

func TestLoad_Override(t *testing.T) {
	env := map[string]string{
		"DX_PUBLIC_DOMAIN":   "example.test",
		"DX_INTERNAL_DOMAIN": "lan",
		"DX_PROXY_PORT":      "8443",
	}
	r := Load(func(k string) string { return env[k] })
	if r.PublicDomain != "example.test" || r.InternalDomain != "lan" || r.ProxyPort != "8443" {
		t.Fatalf("override = %+v", r)
	}
}
