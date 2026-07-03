package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Routing holds machine-global portless URL conventions.
type Routing struct {
	PublicDomain   string
	InternalDomain string
	ProxyPort      string
	Scheme         string // "https" (default) or "http" (portless --no-tls)
}

func or(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Load resolves routing with a 3-tier fallback: env, the running portless
// proxy's state files (~/.portless/proxy.{tld,port,tls}), static defaults
// (plain portless: localhost, 443, https). PublicDomain is env-only; empty
// means "no public domain" (public URLs fall back to the internal form).
func Load(getenv func(string) string) Routing {
	return load(getenv, os.ReadFile)
}

func load(getenv func(string) string, readFile func(string) ([]byte, error)) Routing {
	state := func(name string) string {
		b, err := readFile(filepath.Join(getenv("HOME"), ".portless", name))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	scheme := "https"
	if state("proxy.tls") == "0" {
		scheme = "http"
	}
	return Routing{
		PublicDomain:   getenv("DX_PUBLIC_DOMAIN"),
		InternalDomain: or(getenv("DX_INTERNAL_DOMAIN"), state("proxy.tld"), "localhost"),
		ProxyPort:      or(getenv("DX_PROXY_PORT"), state("proxy.port"), "443"),
		Scheme:         scheme,
	}
}
