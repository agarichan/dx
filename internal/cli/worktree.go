package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agarichan/dx/internal/db"
	"github.com/agarichan/dx/internal/dburl"
	"github.com/agarichan/dx/internal/portless"
	"github.com/agarichan/dx/internal/project"
	"github.com/agarichan/dx/internal/registry"
	"github.com/agarichan/dx/internal/supervisor"
	"github.com/agarichan/dx/internal/worktree"
)

type createOpts struct {
	Branch   string
	From     string
	SkipInit bool
}

type wtDeps struct {
	Cfg              *project.Config
	PrimaryRoot      string
	Existing         []string // existing worktree branch names (for collision)
	Git              func(args ...string) (string, error)
	BranchExists     func(branch string) bool
	ContainerRunning func(name string) bool
	Docker           db.Runner
	Getenv           func(string) string
	RunCopy          func(steps []project.CopyStep, primaryRoot, worktreeRoot string, stdout, stderr io.Writer) error
	RunInit          func(steps []project.InitStep, root, branch, primaryRoot string, stdout, stderr io.Writer) error
	Stdout, Stderr   io.Writer
}

// createWorktree performs `dx worktree create` and returns the exit code.
// 0=ready, 3=worktree created but DB pending, 1=failure.
func createWorktree(o createOpts, d wtDeps) int {
	// collision (DB name) check
	if with, ok := worktree.SlugCollision(o.Branch, d.Existing); ok {
		fmt.Fprintf(d.Stderr, "branch %q collides with existing worktree %q on db/portless name\n", o.Branch, with)
		return 1
	}
	path := filepath.Join(d.PrimaryRoot, d.Cfg.Worktree.Dir, o.Branch)
	// git worktree add
	var args []string
	if d.BranchExists(o.Branch) {
		if o.From != "" {
			fmt.Fprintln(d.Stderr, "cannot pass --from for an existing branch")
			return 1
		}
		args = []string{"worktree", "add", path, o.Branch}
	} else {
		args = []string{"worktree", "add", path, "-b", o.Branch}
		if o.From != "" {
			args = append(args, o.From)
		}
	}
	if _, err := d.Git(args...); err != nil {
		fmt.Fprintln(d.Stderr, "git worktree add:", err)
		return 1
	}
	// DB fork (prepared). Only when [db] declared.
	if d.Cfg.DB.SQLite() {
		s := db.SQLite{Path: d.Cfg.DB.Path}
		if err := s.Seed(d.Docker, d.PrimaryRoot, path); err != nil {
			fmt.Fprintln(d.Stderr, "worktree created but db seed failed:", err)
			return 3
		}
		fmt.Fprintf(d.Stdout, "created %s (branch=%s db=%s)\n", path, o.Branch, d.Cfg.DB.Path)
	} else if d.Cfg.DB != nil {
		raw := d.Cfg.DB.Dsn
		if raw == "" {
			raw = d.Getenv(d.Cfg.DB.URLEnv)
		}
		if raw == "" {
			fmt.Fprintf(d.Stderr, "url_env %q not set; skipping db fork (cd into the worktree and run `dx db fork`)\n", d.Cfg.DB.URLEnv)
			return 3
		}
		dsn, err := dburl.Parse(raw)
		if err != nil {
			fmt.Fprintln(d.Stderr, "parse DSN:", err)
			return 3
		}
		if !d.ContainerRunning(d.Cfg.DB.Container) {
			fmt.Fprintln(d.Stderr, "postgres container not running; run `dx db up` then `dx db fork` in the worktree")
			return 3
		}
		target := worktree.DBName(dsn.Name, o.Branch, false)
		c := db.Container{Name: d.Cfg.DB.Container, Image: d.Cfg.DB.Image, Volume: d.Cfg.DB.Volume, DSN: dsn}
		if err := c.Fork(d.Docker, dsn.Name, target); err != nil {
			fmt.Fprintln(d.Stderr, "worktree created but db fork failed:", err)
			return 3
		}
		fmt.Fprintf(d.Stdout, "created %s (branch=%s db=%s)\n", path, o.Branch, target)
	} else {
		fmt.Fprintf(d.Stdout, "created %s (branch=%s, no db)\n", path, o.Branch)
	}

	// copy steps (after DB fork, before init). fail-fast → rc 3.
	if len(d.Cfg.Worktree.Copy) > 0 && d.RunCopy != nil {
		if err := d.RunCopy(d.Cfg.Worktree.Copy, d.PrimaryRoot, path, d.Stdout, d.Stderr); err != nil {
			fmt.Fprintln(d.Stderr, "worktree created but copy failed:", err)
			return 3
		}
	}

	// init steps (after copy). fail-fast → rc 3 (worktree kept).
	if len(d.Cfg.Worktree.Init) > 0 {
		if o.SkipInit {
			fmt.Fprintln(d.Stderr, "init skipped")
			return 0
		}
		if d.RunInit != nil {
			if err := d.RunInit(d.Cfg.Worktree.Init, path, o.Branch, d.PrimaryRoot, d.Stdout, d.Stderr); err != nil {
				fmt.Fprintln(d.Stderr, "worktree created but init failed:", err)
				return 3
			}
		}
	}
	return 0
}

// existingWorktreeBranches parses `git worktree list --porcelain` for branch names
// other than the primary.
func existingWorktreeBranches(git func(args ...string) (string, error)) ([]string, error) {
	out, err := git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			branches = append(branches, strings.TrimPrefix(ref, "refs/heads/"))
		}
	}
	return branches, nil
}

// containerRunning reports whether a docker container with the given name is running.
func containerRunning(name string) bool {
	out, err := dockerRunner("docker", "ps", "-q", "-f", "name=^"+name+"$")
	return err == nil && strings.TrimSpace(out) != ""
}

// branchExists reports whether a local git branch exists.
func branchExists(git func(args ...string) (string, error), branch string) bool {
	_, err := git("rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

type rmOpts struct {
	Branch       string
	Force        bool
	KeepDB       bool
	DeleteBranch bool
}

type rmDeps struct {
	Cfg            *project.Config
	PrimaryRoot    string
	Git            func(args ...string) (string, error)
	Docker         db.Runner
	Getenv         func(string) string
	StopServices   func(toplevel string) error // downRegistry for the worktree's registry
	Dirty          func(path string) bool
	Toplevel       func(path string) (string, error) // canonical toplevel via git (registry key)
	Stdout, Stderr io.Writer
}

type wtRow struct {
	Branch    string   `json:"branch"`
	Path      string   `json:"path"`
	IsPrimary bool     `json:"is_primary"`
	DB        string   `json:"db"`
	Services  []string `json:"services"`
}

// parseWorktreePorcelain extracts branch/path rows from `git worktree list --porcelain`.
// The first entry is the primary checkout.
func parseWorktreePorcelain(porc string) []wtRow {
	var rows []wtRow
	var cur *wtRow
	for _, line := range strings.Split(porc, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			rows = append(rows, wtRow{Path: strings.TrimPrefix(line, "worktree ")})
			cur = &rows[len(rows)-1]
		case strings.HasPrefix(line, "branch ") && cur != nil:
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	if len(rows) > 0 {
		rows[0].IsPrimary = true
	}
	return rows
}

// listRows joins worktree list with DB name and per-service state.
func listRows(cfg *project.Config, baseDB, porc string, serviceState func(svcName string) string) []wtRow {
	rows := parseWorktreePorcelain(porc)
	for i := range rows {
		r := &rows[i]
		if cfg.DB.SQLite() {
			r.DB = cfg.DB.Path // same relative file in every checkout
		} else {
			r.DB = worktree.DBName(baseDB, r.Branch, r.IsPrimary)
		}
		for _, k := range cfg.ServiceKeys() {
			s := cfg.Services[k]
			name := portless.SvcName(s.Name, r.Branch, r.IsPrimary)
			r.Services = append(r.Services, name+":"+serviceState(name))
		}
	}
	return rows
}

// rmWorktree performs `dx worktree rm`.
// All abort-able preconditions run first (no side effects), then destructive actions.
func rmWorktree(o rmOpts, d rmDeps) int {
	path := filepath.Join(d.PrimaryRoot, d.Cfg.Worktree.Dir, o.Branch)

	// --- Preconditions (no side effects) ---
	// 1a. dirty check
	if d.Dirty(path) && !o.Force {
		fmt.Fprintln(d.Stderr, "worktree has changes; use --force")
		return 1
	}
	// 1b. resolve DB drop target before any destructive action
	var (
		pendingDrop   bool
		dropContainer db.Container
		dropTarget    string
	)
	// sqlite: the db file lives inside the worktree and is removed with it.
	if !o.KeepDB && d.Cfg.DB != nil && !d.Cfg.DB.SQLite() {
		raw := d.Cfg.DB.Dsn
		if raw == "" {
			raw = d.Getenv(d.Cfg.DB.URLEnv)
		}
		if raw == "" {
			fmt.Fprintf(d.Stderr, "url_env %q not set; cannot compute DB name. set it and retry, or use --keep-db\n", d.Cfg.DB.URLEnv)
			return 1
		}
		dsn, err := dburl.Parse(raw)
		if err != nil {
			fmt.Fprintln(d.Stderr, "parse DSN:", err)
			return 1
		}
		dropTarget = worktree.DBName(dsn.Name, o.Branch, false) // structurally != base (spec §7.6)
		dropContainer = db.Container{Name: d.Cfg.DB.Container, DSN: dsn}
		pendingDrop = true
	}

	// --- Destructive actions (only after all preconditions passed) ---
	// 2a. stop services registered under the worktree's canonical toplevel
	top, err := d.Toplevel(path)
	if err != nil {
		top = path // best effort; registry may simply be empty
	}
	if err := d.StopServices(top); err != nil {
		fmt.Fprintln(d.Stderr, "warning: stopping services:", err)
	}
	// 2b. DB drop
	if pendingDrop {
		if err := dropContainer.Drop(d.Docker, dropTarget); err != nil {
			fmt.Fprintln(d.Stderr, "db drop failed (worktree kept):", err)
			return 1
		}
	}
	// 2c. git worktree remove
	rmArgs := []string{"worktree", "remove", path}
	if o.Force {
		rmArgs = append(rmArgs, "--force")
	}
	if _, err := d.Git(rmArgs...); err != nil {
		fmt.Fprintln(d.Stderr, "git worktree remove:", err)
		return 1
	}
	// 2d. optional branch delete
	if o.DeleteBranch {
		if _, err := d.Git("branch", "-d", o.Branch); err != nil {
			fmt.Fprintln(d.Stderr, "warning: branch delete:", err)
		}
	}
	fmt.Fprintf(d.Stdout, "removed %s (branch=%s)\n", path, o.Branch)
	return 0
}

// parseWorktreeArgs parses "<branch> [flags...]" from rest, accepting flags
// before OR after the branch positional. Go's flag package stops at the first
// non-flag arg, so we parse once to find the branch, then parse the remainder.
func parseWorktreeArgs(fs *flag.FlagSet, rest []string) (string, error) {
	if err := fs.Parse(rest); err != nil {
		return "", err
	}
	if fs.NArg() < 1 {
		return "", fmt.Errorf("missing <branch>")
	}
	branch := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return "", err
	}
	return branch, nil
}

// runWorktree dispatches `dx worktree <create|rm|list>` with real git/docker runners.
func runWorktree(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: dx worktree <create|rm|list> ...")
		return 2
	}
	sub, rest := args[0], args[1:]

	// All worktree commands run from the primary checkout.
	wt, err := worktree.Detect(".", gitRun("."))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !wt.IsPrimary {
		fmt.Fprintln(stderr, "run from the primary checkout")
		return 1
	}
	cfg, err := project.Load(filepath.Join(wt.Toplevel, "dx.toml"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	primaryGit := gitRun(wt.Toplevel)

	switch sub {
	case "create":
		fs := flag.NewFlagSet("create", flag.ContinueOnError)
		from := fs.String("from", "", "branch start point")
		skipInit := fs.Bool("skip-init", false, "skip [[worktree.init]] steps")
		branch, err := parseWorktreeArgs(fs, rest)
		if err != nil {
			fmt.Fprintln(stderr, "usage: dx worktree create <branch> [--from <base>] [--skip-init]")
			return 2
		}
		existing, _ := existingWorktreeBranches(primaryGit)
		return createWorktree(createOpts{Branch: branch, From: *from, SkipInit: *skipInit}, wtDeps{
			Cfg: cfg, PrimaryRoot: wt.Toplevel, Existing: existing,
			Git:              primaryGit,
			BranchExists:     func(b string) bool { return branchExists(primaryGit, b) },
			ContainerRunning: containerRunning,
			Docker:           dockerRunner,
			Getenv:           os.Getenv,
			RunCopy:          runCopySteps,
			RunInit:          runInitSteps,
			Stdout:           stdout, Stderr: stderr,
		})
	case "rm":
		fs := flag.NewFlagSet("rm", flag.ContinueOnError)
		force := fs.Bool("force", false, "remove even if dirty")
		keepDB := fs.Bool("keep-db", false, "skip DB drop")
		delBranch := fs.Bool("delete-branch", false, "also delete the git branch")
		branch, err := parseWorktreeArgs(fs, rest)
		if err != nil {
			fmt.Fprintln(stderr, "usage: dx worktree rm <branch> [--force] [--keep-db] [--delete-branch]")
			return 2
		}
		return rmWorktree(rmOpts{Branch: branch, Force: *force, KeepDB: *keepDB, DeleteBranch: *delBranch}, rmDeps{
			Cfg: cfg, PrimaryRoot: wt.Toplevel,
			Git:    primaryGit,
			Docker: dockerRunner,
			Getenv: os.Getenv,
			StopServices: func(top string) error {
				reg, err := registry.Open(registry.StateRoot(os.Getenv), top)
				if err != nil {
					return err
				}
				downRegistry(reg, stdout, stderr)
				return nil
			},
			Dirty: func(path string) bool {
				out, _ := gitRun(path)("status", "--porcelain")
				return strings.TrimSpace(out) != ""
			},
			Toplevel: func(path string) (string, error) {
				return gitRun(path)("rev-parse", "--show-toplevel")
			},
			Stdout: stdout, Stderr: stderr,
		})
	case "list":
		asJSON := false
		for _, a := range rest {
			if a == "--json" {
				asJSON = true
			}
		}
		return runWorktreeList(cfg, wt, asJSON, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown worktree subcommand: %s\n", sub)
		return 2
	}
}

// runWorktreeList builds the cross-view and prints it (table or JSON).
func runWorktreeList(cfg *project.Config, wt *worktree.Info, asJSON bool, stdout, stderr io.Writer) int {
	porc, err := gitRun(wt.Toplevel)("worktree", "list", "--porcelain")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	baseDB := ""
	if cfg.DB != nil {
		if dsn, derr := baseDSN(cfg); derr == nil {
			baseDB = dsn.Name
		}
	}
	all, _ := registry.All(registry.StateRoot(os.Getenv))
	state := func(svcName string) string {
		for _, l := range all {
			if l.Service.Name == svcName {
				if supervisor.Alive(l.Service.PID) {
					return "running"
				}
				return "stopped"
			}
		}
		return "stopped"
	}
	rows := listRows(cfg, baseDB, porc, state)
	if asJSON {
		data, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Fprintln(stdout, string(data))
		return 0
	}
	fmt.Fprintf(stdout, "%-18s %-32s %-16s %s\n", "BRANCH", "PATH", "DB", "SERVICES")
	for _, r := range rows {
		label := r.Branch
		if r.IsPrimary {
			label += " (primary)"
		}
		fmt.Fprintf(stdout, "%-18s %-32s %-16s %s\n", label, r.Path, r.DB, strings.Join(r.Services, " "))
	}
	return 0
}
