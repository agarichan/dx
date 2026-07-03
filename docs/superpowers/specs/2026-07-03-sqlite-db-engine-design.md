# SQLite db engine — design

Date: 2026-07-03
Status: approved

## Goal

Support `[db]` with SQLite. SQLite databases are checkout-relative files, so
per-worktree isolation is inherent — dx's job reduces to seeding a new
worktree's file from the primary (`fork`), resolving the per-checkout DSN
(`url`), and small conveniences (`psql` shell, `drop`/`reset`). No Docker.

Reference workload: an app configured with `database_url = "sqlite:///./dev.db"`
(cwd-relative), overridable via an env var — each checkout naturally gets its
own file; only the initial data seed is missing.

## Config

```toml
[db]
engine = "sqlite"   # default: "postgres" (backward compatible)
path   = "dev.db"   # sqlite only — file path relative to the checkout root
```

Validation:
- `engine` ∈ {"", "postgres", "sqlite"} ("" = postgres).
- sqlite: `path` required; must be relative and stay inside the checkout
  (no absolute, no `..` — same rule as worktree dirs). `container` / `dsn` /
  `url_env` / `image` / `volume` must be unset (misconfiguration error).
- postgres: `path` must be unset; existing rules unchanged.

## internal/db — sqlite.go

```go
type SQLite struct{ Path string } // relative to a checkout root

func (s SQLite) File(root string) string   // filepath.Join(root, s.Path)
func (s SQLite) URL(root string) string    // "sqlite:///" + absolute file path
func (s SQLite) Exists(root string) bool
// Seed copies base→target via `sqlite3 <base> ".backup <target>"` (consistent
// snapshot even with active writers). Idempotent: target exists → no-op.
// Base missing → error. Creates the target's parent dir.
func (s SQLite) Seed(run Runner, baseRoot, targetRoot string) error
func (s SQLite) Drop(root string) error    // remove file + -wal/-shm siblings
func (s SQLite) ShellArgs(root string) []string // {"sqlite3", file}
```

Reuses the existing `Runner` func type; requires `sqlite3` on PATH (ships with
macOS; available on CI). Missing binary surfaces as a clear exec error.

## CLI behavior (engine = "sqlite")

| Subcommand | Behavior |
|---|---|
| `url` | print `URL(current checkout root)` |
| `fork` | worktree only: `Seed(primary root, worktree root)` |
| `drop` | worktree only: `Drop(worktree root)` |
| `reset` | `drop` + `fork` |
| `psql` | interactive `sqlite3 <file>` |
| `up` / `down` | no-op with a note (no container to manage) |
| `list` | one line per git worktree: root, file, exists/size |

- `dx serve` with `service.db = true`: on a linked worktree, `Seed` instead of
  the Postgres fork (primary: no-op). Failure = warning, service still starts
  (same as Postgres today).
- `dx worktree create`: sqlite branch skips the container-running check and
  DSN parsing; seeds the new worktree's file. Seed failure → exit 3, worktree
  kept (same contract).
- `dx worktree rm`: no DB action — the file lives inside the worktree and is
  removed with it. `--keep-db` is meaningless for sqlite (ignored).
- `dx worktree list`: DB column shows the file's relative path when it exists
  in that worktree, `-` otherwise.

## Plumbing

`worktree.Info` gains `PrimaryRoot` (= `filepath.Dir(git-common-dir)`, already
computed inside `Detect`) so `dx serve` / `dx db fork` in a linked worktree can
locate the primary checkout without new git calls.

## Testing

- project: validation matrix (engine values, sqlite field rules, postgres
  unchanged).
- db: Seed via fake Runner (command shape) + real `sqlite3` (LookPath-gated)
  for idempotence / missing base / parent-dir creation; Drop removes wal/shm;
  URL absolute-path format.
- cli: `db url` / no-op `up` with a sqlite config in a temp git repo;
  worktree create sqlite-seed via existing Deps injection tests.
- worktree: Detect populates PrimaryRoot (primary == toplevel; linked ≠).

## Out of scope

- WAL-specific handling beyond removing `-wal`/`-shm` on drop.
- MySQL or other engines; engine plugin interface (two engines don't justify
  it yet — CLI branches on `cfg.DB.Engine`).
