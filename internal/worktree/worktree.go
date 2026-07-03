package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type Info struct {
	Toplevel  string
	Branch    string
	IsPrimary bool
}

// Slug normalizes a branch name into a postgres-identifier-safe token.
func Slug(branch string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(branch) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := strings.Trim(b.String(), "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	if s == "" {
		return "wt"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "_" + s
	}
	return s
}

// DBName derives the database name for a checkout. Primary uses base verbatim.
func DBName(base, branch string, isPrimary bool) string {
	if isPrimary {
		return base
	}
	name := base + "_" + Slug(branch)
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "_")
	}
	return name
}

// CheckoutHash is a stable short id derived from the checkout root path.
func CheckoutHash(toplevel string) string {
	sum := sha256.Sum256([]byte(toplevel))
	return hex.EncodeToString(sum[:])[:12]
}

// SlugCollision reports whether newBranch's Slug equals any existing branch's
// Slug. Since DBName derives from Slug, a collision means two distinct branches
// would map to the same database (which Fork shares idempotently — dangerous on
// drop). Returns the first colliding existing branch and true.
func SlugCollision(newBranch string, existing []string) (string, bool) {
	want := Slug(newBranch)
	for _, e := range existing {
		if Slug(e) == want {
			return e, true
		}
	}
	return "", false
}

// Detect resolves the current checkout via git. run executes `git <args...>`.
func Detect(dir string, run func(args ...string) (string, error)) (*Info, error) {
	top, err := run("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("git toplevel: %w", err)
	}
	// --path-format=absolute makes BOTH paths absolute. Without it, from a subdirectory
	// git returns --git-dir absolute but --git-common-dir relative (e.g. "../.git"), so a
	// plain string compare wrongly flags a primary checkout as a linked worktree. dx serve
	// runs with dir="api" (a subdir), so this matters. (git >= 2.31 supports --path-format.)
	gout, err := run("rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("git-dir: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(gout), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected git rev-parse output: %q", gout)
	}
	gd := strings.TrimSpace(lines[0])
	gc := strings.TrimSpace(lines[1])
	branch, err := run("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" {
		branch = "wt"
	}
	return &Info{
		Toplevel:  strings.TrimSpace(top),
		Branch:    strings.TrimSpace(branch),
		IsPrimary: gd == gc,
	}, nil
}
