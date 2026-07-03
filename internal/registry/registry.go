package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agarichan/dx/internal/worktree"
)

type Service struct {
	Name     string            `json:"name"`
	PID      int               `json:"pid"`
	LogPath  string            `json:"log_path"`
	Dir      string            `json:"dir"`
	Root     string            `json:"root,omitempty"`
	URL      string            `json:"url,omitempty"`
	Command  []string          `json:"command"`
	Pub      map[string]string `json:"pub,omitempty"`
	Internal map[string]string `json:"internal,omitempty"`
	DB       string            `json:"db,omitempty"`
	Key      string            `json:"key,omitempty"`  // dx.toml service map key
	Open     bool              `json:"open,omitempty"` // this service's URL is the one UIs open
}

type Registry struct {
	Dir string
}

// StateRoot returns the base dir for dx runtime state.
func StateRoot(getenv func(string) string) string {
	if x := getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "dx")
	}
	return filepath.Join(getenv("HOME"), ".local", "state", "dx")
}

// Open returns the per-checkout registry under stateRoot, creating it if needed.
func Open(stateRoot, toplevel string) (*Registry, error) {
	dir := filepath.Join(stateRoot, worktree.CheckoutHash(toplevel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir state: %w", err)
	}
	return &Registry{Dir: dir}, nil
}

func (r *Registry) path(name string) string {
	return filepath.Join(r.Dir, name+".json")
}

func (r *Registry) Put(s Service) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path(s.Name), data, 0o644)
}

func (r *Registry) Get(name string) (*Service, bool, error) {
	data, err := os.ReadFile(r.path(name))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var s Service
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, false, err
	}
	return &s, true, nil
}

func (r *Registry) List() ([]Service, error) {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil, err
	}
	var out []Service
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		s, ok, err := r.Get(name)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (r *Registry) Remove(name string) error {
	err := os.Remove(r.path(name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Located is a Service paired with the Registry it lives in (for Remove).
type Located struct {
	Service  Service
	Registry *Registry
}

// All returns every service across all per-checkout registries under stateRoot.
func All(stateRoot string) ([]Located, error) {
	entries, err := os.ReadDir(stateRoot)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state root: %w", err)
	}
	var out []Located
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		reg := &Registry{Dir: filepath.Join(stateRoot, e.Name())}
		svcs, err := reg.List()
		if err != nil {
			return nil, err
		}
		for _, s := range svcs {
			out = append(out, Located{Service: s, Registry: reg})
		}
	}
	return out, nil
}
