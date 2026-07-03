package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	r := Load(func(string) string { return "" })
	if r.PublicDomain != "" || r.InternalDomain != "localhost" || r.ProxyPort != "443" {
		t.Fatalf("defaults = %+v", r)
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
