// Package project loads dx.toml, the single source of project configuration
// for worktree / serve / db / up commands.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

type Worktree struct {
	Dir  string     `toml:"dir"`
	Copy []CopyStep `toml:"copy"`
	Init []InitStep `toml:"init"`
}

// InitStep is one command run after `dx worktree create` succeeds.
type InitStep struct {
	Command []string `toml:"command"`
	Dir     string   `toml:"dir"` // relative to the new worktree root; empty = root
}

// CopyStep seeds a file or directory from the primary worktree into the new
// worktree at the same relative path. Missing sources and existing destinations
// are silently skipped.
type CopyStep struct {
	From string `toml:"from"` // relative to the primary root; also the destination in the new worktree
}

type DB struct {
	Engine    string `toml:"engine"` // "" | "postgres" | "sqlite" ("" = postgres)
	Container string `toml:"container"`
	Dsn       string `toml:"dsn"`
	URLEnv    string `toml:"url_env"`
	Image     string `toml:"image"`
	Volume    string `toml:"volume"`
	Path      string `toml:"path"` // sqlite only: db file relative to the checkout root
}

// SQLite reports whether the managed DB is the sqlite engine.
func (d *DB) SQLite() bool { return d != nil && d.Engine == "sqlite" }

type Service struct {
	Name     string            `toml:"name"`
	Command  []string          `toml:"command"`
	DB       bool              `toml:"db"`
	Dir      string            `toml:"dir"`
	Pub      map[string]string `toml:"pub"`
	Internal map[string]string `toml:"internal"`
	Open     bool              `toml:"open"` // this service's URL is the one UIs open (max 1 per config)
	Key      string            `toml:"-"`    // map key, populated by Load
}

type Config struct {
	Worktree Worktree           `toml:"worktree"`
	DB       *DB                `toml:"db"`
	Services map[string]Service `toml:"service"`
	Root     string             `toml:"-"` // directory containing dx.toml
}

const (
	defaultWorktreeDir = ".claude/worktrees"
	defaultImage       = "postgres:18"
)

// Validate checks required fields after Load.
func (c *Config) Validate() error {
	if c.DB != nil {
		switch c.DB.Engine {
		case "", "postgres":
			if c.DB.Path != "" {
				return fmt.Errorf("[db].path is sqlite-only (engine = %q)", c.DB.Engine)
			}
			if c.DB.Container == "" {
				return fmt.Errorf("[db].container is required when [db] is present")
			}
			if c.DB.Dsn == "" && c.DB.URLEnv == "" {
				return fmt.Errorf("[db]: set dsn or url_env")
			}
		case "sqlite":
			if c.DB.Path == "" {
				return fmt.Errorf("[db].path is required for engine = \"sqlite\"")
			}
			if cleaned := filepath.Clean(c.DB.Path); filepath.IsAbs(cleaned) || cleaned == ".." ||
				strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
				return fmt.Errorf("[db].path %q must be relative and stay inside the checkout", c.DB.Path)
			}
			if c.DB.Container != "" || c.DB.Image != "" || c.DB.Volume != "" {
				return fmt.Errorf("[db]: container/image/volume are postgres-only (engine = \"sqlite\")")
			}
			if c.DB.Dsn != "" || c.DB.URLEnv != "" {
				return fmt.Errorf("[db]: dsn/url_env are postgres-only (engine = \"sqlite\"; the DSN derives from path)")
			}
		default:
			return fmt.Errorf("[db].engine %q is not supported (use \"postgres\" or \"sqlite\")", c.DB.Engine)
		}
	}
	for i, step := range c.Worktree.Init {
		if len(step.Command) == 0 {
			return fmt.Errorf("worktree.init[%d]: command is required", i)
		}
		if cleaned := filepath.Clean(step.Dir); filepath.IsAbs(cleaned) || cleaned == ".." ||
			strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return fmt.Errorf("worktree.init[%d]: dir %q must be relative and stay inside the worktree", i, step.Dir)
		}
	}
	for i, step := range c.Worktree.Copy {
		if step.From == "" {
			return fmt.Errorf("worktree.copy[%d]: from is required", i)
		}
		cleaned := filepath.Clean(step.From)
		if filepath.IsAbs(cleaned) || cleaned == ".." ||
			strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return fmt.Errorf("worktree.copy[%d]: from %q must be relative and stay inside the worktree", i, step.From)
		}
	}
	openKey := ""
	for key, s := range c.Services {
		if len(s.Command) == 0 {
			return fmt.Errorf("service %q: command is required", key)
		}
		if s.DB && c.DB == nil {
			return fmt.Errorf("service %q: db=true but no [db] table", key)
		}
		if s.Open {
			if openKey != "" {
				return fmt.Errorf("services %q and %q both set open=true (max 1)", openKey, key)
			}
			openKey = key
		}
	}
	return nil
}

// Service returns the service with the given key.
func (c *Config) Service(key string) (Service, bool) {
	s, ok := c.Services[key]
	return s, ok
}

// ServiceKeys returns the service keys in deterministic (sorted) order.
func (c *Config) ServiceKeys() []string {
	keys := make([]string, 0, len(c.Services))
	for k := range c.Services {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Load reads and decodes dx.toml at path, applies defaults, and records Root.
func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("dx.toml not found at %s", filepath.Dir(path))
	}
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, fmt.Errorf("dx.toml: %w", err)
	}
	if c.Worktree.Dir == "" {
		c.Worktree.Dir = defaultWorktreeDir
	}
	if c.DB != nil && c.DB.Image == "" {
		c.DB.Image = defaultImage
	}
	// Record each service's map key and default its Name to it.
	for k, s := range c.Services {
		s.Key = k
		if s.Name == "" {
			s.Name = k
		}
		c.Services[k] = s
	}
	c.Root = filepath.Dir(path)
	return &c, nil
}
