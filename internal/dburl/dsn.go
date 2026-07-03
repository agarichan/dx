package dburl

import (
	"fmt"
	"net/url"
	"strings"
)

// DSN is a parsed postgres connection URL (sqlalchemy-style scheme+driver supported).
type DSN struct {
	Scheme   string
	Driver   string
	User     string
	Password string
	Host     string
	Port     string
	Name     string
	RawQuery string
}

// Parse parses a connection URL such as postgresql+asyncpg://u:p@h:5432/db?x=y.
func Parse(raw string) (*DSN, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	scheme, driver := u.Scheme, ""
	if i := strings.IndexByte(scheme, '+'); i >= 0 {
		scheme, driver = scheme[:i], scheme[i+1:]
	}
	pw, _ := u.User.Password()
	return &DSN{
		Scheme:   scheme,
		Driver:   driver,
		User:     u.User.Username(),
		Password: pw,
		Host:     u.Hostname(),
		Port:     u.Port(),
		Name:     strings.TrimPrefix(u.Path, "/"),
		RawQuery: u.RawQuery,
	}, nil
}

// WithScheme returns a copy with Scheme (and Driver, when s contains
// "scheme+driver") replaced.
func (d *DSN) WithScheme(s string) *DSN {
	c := *d
	c.Scheme, c.Driver = s, ""
	if i := strings.IndexByte(s, '+'); i >= 0 {
		c.Scheme, c.Driver = s[:i], s[i+1:]
	}
	return &c
}

// WithName returns a copy with Name replaced.
func (d *DSN) WithName(name string) *DSN {
	c := *d
	c.Name = name
	return &c
}

// String reconstructs the connection URL.
func (d *DSN) String() string {
	scheme := d.Scheme
	if d.Driver != "" {
		scheme += "+" + d.Driver
	}
	var b strings.Builder
	b.WriteString(scheme)
	b.WriteString("://")
	if d.User != "" {
		b.WriteString(d.User)
		if d.Password != "" {
			b.WriteString(":" + d.Password)
		}
		b.WriteByte('@')
	}
	b.WriteString(d.Host)
	if d.Port != "" {
		b.WriteString(":" + d.Port)
	}
	b.WriteString("/" + d.Name)
	if d.RawQuery != "" {
		b.WriteString("?" + d.RawQuery)
	}
	return b.String()
}
