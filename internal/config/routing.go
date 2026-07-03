package config

// Routing holds machine-global portless URL conventions.
type Routing struct {
	PublicDomain   string
	InternalDomain string
	ProxyPort      string
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Load reads routing constants from env, falling back to plain-portless defaults
// (proxy on 443, .localhost domains). An empty PublicDomain means "no public
// domain": public URLs fall back to the internal form.
func Load(getenv func(string) string) Routing {
	return Routing{
		PublicDomain:   getenv("DX_PUBLIC_DOMAIN"),
		InternalDomain: or(getenv("DX_INTERNAL_DOMAIN"), "localhost"),
		ProxyPort:      or(getenv("DX_PROXY_PORT"), "443"),
	}
}
