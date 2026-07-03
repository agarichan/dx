package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agarichan/dx/internal/config"
	"github.com/agarichan/dx/internal/db"
	"github.com/agarichan/dx/internal/dburl"
	"github.com/agarichan/dx/internal/portless"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/registry"
	"github.com/agarichan/dx/internal/supervisor"
	"github.com/agarichan/dx/internal/worktree"
)

// loadConfig detects the checkout and loads+validates dx.toml from its toplevel.
func loadConfig() (*project.Config, *worktree.Info, error) {
	wt, err := worktree.Detect(".", gitRun("."))
	if err != nil {
		return nil, nil, err
	}
	cfg, err := project.Load(filepath.Join(wt.Toplevel, "dx.toml"))
	if err != nil {
		return nil, nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	return cfg, wt, nil
}

// dbEnvValue computes the per-checkout DSN for db_env injection.
// scheme overrides the DSN scheme when non-empty. lookup resolves [db].url_env
// from the parent env (keeps this pure — no os.Getenv).
func dbEnvValue(cfg *project.Config, wt *worktree.Info, scheme string, lookup func(string) string) (string, error) {
	if cfg.DB.SQLite() {
		s := db.SQLite{Path: cfg.DB.Path}
		if scheme == "" {
			scheme = "sqlite"
		}
		return scheme + ":///" + s.File(wt.Toplevel), nil
	}
	raw := cfg.DB.Dsn
	if raw == "" {
		raw = lookup(cfg.DB.URLEnv)
	}
	if raw == "" {
		return "", fmt.Errorf("[db]: set dsn or env var %q", cfg.DB.URLEnv)
	}
	d, err := dburl.Parse(raw)
	if err != nil {
		return "", err
	}
	d = d.WithName(worktree.DBName(d.Name, wt.Branch, wt.IsPrimary))
	if scheme != "" {
		d = d.WithScheme(scheme)
	}
	return d.String(), nil
}

// buildEnv computes the child env with injected URL vars.
// References in svc.Pub/Internal are resolved as config keys first; if no key
// matches, the value is used as a literal portless base name.
// "self" always resolves to the current service's Name.
// It is a pure function: no git/docker/process side effects.
func buildEnv(parent []string, cfg *project.Config, svc project.Service, wt *worktree.Info, cli portless.Client) ([]string, error) {
	env := append([]string{}, parent...)
	set := func(k, v string) {
		env = append(env, k+"="+v)
	}
	set("FORCE_COLOR", "1")
	set("CLICOLOR_FORCE", "1")
	resolve := func(ref string) string {
		target := svc.Name // default: self
		if ref != "self" {
			if t, ok := cfg.Services[ref]; ok {
				target = t.Name
			} else {
				target = ref // literal portless base name fallback
			}
		}
		return portless.SvcName(target, wt.Branch, wt.IsPrimary)
	}
	for k, ref := range svc.Pub {
		set(k, cli.URLPublic(resolve(ref)))
	}
	for k, ref := range svc.Internal {
		set(k, cli.URLInternal(resolve(ref)))
	}
	if svc.DBEnv != nil {
		lookup := func(k string) string {
			for _, kv := range parent {
				if strings.HasPrefix(kv, k+"=") {
					return kv[len(k)+1:]
				}
			}
			return ""
		}
		v, err := dbEnvValue(cfg, wt, svc.DBEnv.Scheme, lookup)
		if err != nil {
			return nil, fmt.Errorf("service %q db_env: %w", svc.Key, err)
		}
		set(svc.DBEnv.Name, v)
	}
	return env, nil
}

// serviceDir returns the working directory for svc: <repo root>/<svc.Dir>.
// An empty svc.Dir yields the repo root.
func serviceDir(cfg *project.Config, svc project.Service) string {
	return filepath.Join(cfg.Root, svc.Dir)
}

// ensureServiceDB runs the idempotent worktree DB fork/seed for services that
// want a DB (db=true or db_env). Failures are warnings — callers proceed.
// No-op on the primary checkout or when the service declares no DB.
func ensureServiceDB(cfg *project.Config, svc project.Service, wt *worktree.Info, stderr io.Writer) {
	if (!svc.DB && svc.DBEnv == nil) || cfg.DB == nil || wt.IsPrimary {
		return
	}
	if cfg.DB.SQLite() {
		s := db.SQLite{Path: cfg.DB.Path}
		if ferr := s.Seed(dockerRunner, wt.PrimaryRoot, wt.Toplevel); ferr != nil {
			fmt.Fprintln(stderr, "warning: db seed failed:", ferr)
		}
		return
	}
	base, err := baseDSN(cfg)
	if err != nil {
		fmt.Fprintln(stderr, "warning: db fork skipped:", err)
		return
	}
	target := worktree.DBName(base.Name, wt.Branch, wt.IsPrimary)
	c := db.Container{Name: cfg.DB.Container, Image: cfg.DB.Image, Volume: cfg.DB.Volume, DSN: base}
	if ferr := c.Fork(dockerRunner, base.Name, target); ferr != nil {
		fmt.Fprintln(stderr, "warning: db fork failed:", ferr)
	}
}

// startService starts one service (background) and records it in the registry.
// Idempotent: no-op if the portless name is already alive.
func startService(cfg *project.Config, svc project.Service, wt *worktree.Info,
	reg *registry.Registry, pcli portless.Client, stdout, stderr io.Writer) error {
	env, err := buildEnv(os.Environ(), cfg, svc, wt, pcli)
	if err != nil {
		return err
	}
	ensureServiceDB(cfg, svc, wt, stderr)
	name := portless.SvcName(svc.Name, wt.Branch, wt.IsPrimary)
	if existing, ok, _ := reg.Get(name); ok && supervisor.Alive(existing.PID) {
		fmt.Fprintf(stdout, "%s already running (pid %d)\n", name, existing.PID)
		return nil
	}
	appPort, err := portless.FreePort()
	if err != nil {
		return err
	}
	cmd := portless.SubstitutePort(svc.Command, appPort)
	full := append([]string{"portless"}, portless.ServeArgv(name, appPort, cmd)...)
	logPath := reg.Dir + "/" + name + ".log"
	colorPath := reg.Dir + "/" + name + ".color.log"
	pid, err := supervisor.Start(supervisor.Spec{
		Dir: serviceDir(cfg, svc), Env: env, Command: full, LogPath: logPath, ColorLogPath: colorPath,
	})
	if err != nil {
		return err
	}
	dbURLEnv := ""
	if svc.DBEnv != nil {
		dbURLEnv = svc.DBEnv.Name
	} else if svc.DB && cfg.DB != nil {
		dbURLEnv = cfg.DB.URLEnv
	}
	_ = reg.Put(registry.Service{
		Name: name, PID: pid, LogPath: logPath, Dir: serviceDir(cfg, svc),
		Root: wt.Toplevel, URL: pcli.URLPublic(name),
		Command: full, Pub: svc.Pub, Internal: svc.Internal, DB: dbURLEnv,
		Key: svc.Key, Open: svc.Open,
	})
	fmt.Fprintf(stdout, "%s started (pid %d) -> %s\n", name, pid, logPath)
	if (svc.DB || svc.DBEnv != nil) && cfg.DB != nil {
		if cfg.DB.SQLite() {
			fmt.Fprintf(stdout, "  db: %s\n", cfg.DB.Path)
		} else if base, err := baseDSN(cfg); err == nil {
			target := worktree.DBName(base.Name, wt.Branch, wt.IsPrimary)
			fmt.Fprintf(stdout, "  db: %s\n", target)
		}
	}
	return nil
}

// runServe implements `dx serve <key>` (single named service from dx.toml).
// <key> is the map key in [service.<key>].
func runServe(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "" {
		fmt.Fprintln(stderr, "usage: dx serve <key>")
		return 2
	}
	name := args[0]
	cfg, wt, err := loadConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	svc, ok := cfg.Service(name)
	if !ok {
		fmt.Fprintf(stderr, "unknown service: %s\n", name)
		return 1
	}
	pcli := portless.Client{R: config.Load(os.Getenv)}
	reg, err := registry.Open(registry.StateRoot(os.Getenv), wt.Toplevel)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := startService(cfg, svc, wt, reg, pcli, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
