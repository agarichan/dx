package portless

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/agarichan/dx/internal/config"
)

// PortToken is the placeholder users put in a serve command where the allocated
// port should go (e.g. `--port {port}`). dx substitutes it before exec, so the
// command needs no shell to expand $PORT.
const PortToken = "{port}"

// FreePort asks the OS for an unused TCP port on the loopback interface.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("alloc free port: %w", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// SubstitutePort returns a copy of cmd with every PortToken replaced by port.
func SubstitutePort(cmd []string, port int) []string {
	out := make([]string, len(cmd))
	p := strconv.Itoa(port)
	for i, a := range cmd {
		out[i] = strings.ReplaceAll(a, PortToken, p)
	}
	return out
}

// Sanitize normalizes a string into a DNS-label-safe token (a-z0-9, single dashes).
func Sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	return strings.Trim(out, "-")
}

// SvcName returns the portless registration name (flat single label).
func SvcName(base, branch string, isPrimary bool) string {
	if isPrimary {
		return base
	}
	suffix := Sanitize(branch)
	if suffix == "" {
		suffix = "wt"
	}
	name := base + "-" + suffix
	if len(name) <= 63 {
		return name
	}
	sum := sha256.Sum256([]byte(branch))
	hash := hex.EncodeToString(sum[:])[:6]
	maxSuf := 63 - len(base) - 1 - len(hash) - 1
	if maxSuf < 1 {
		maxSuf = 1
	}
	if len(suffix) > maxSuf {
		suffix = suffix[:maxSuf]
	}
	return base + "-" + suffix + "-" + hash
}

// Client builds portless URLs from routing constants.
type Client struct {
	R config.Routing
}

// URLPublic returns the public URL for name. With no PublicDomain configured
// (plain portless, no externally reachable domain) it falls back to the
// internal URL so dx.toml pub entries still resolve.
func (c Client) URLPublic(name string) string {
	if c.R.PublicDomain == "" {
		return c.URLInternal(name)
	}
	return "https://" + name + "." + c.R.PublicDomain
}

func (c Client) URLInternal(name string) string {
	scheme := c.R.Scheme
	if scheme == "" {
		scheme = "https"
	}
	u := scheme + "://" + name + "." + c.R.InternalDomain
	if (scheme == "https" && c.R.ProxyPort == "443") || (scheme == "http" && c.R.ProxyPort == "80") {
		return u
	}
	return u + ":" + c.R.ProxyPort
}

// ServeArgv builds the argv for `portless <name> --app-port <port> -- <cmd...>`.
// dx allocates the port itself (--app-port pins portless to it) so it can also
// substitute it into the command via {port}; portless still exports PORT to the child.
func ServeArgv(name string, appPort int, cmd []string) []string {
	return append([]string{name, "--app-port", strconv.Itoa(appPort), "--"}, cmd...)
}
