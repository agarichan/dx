# dx

A developer-experience CLI that wraps [portless](https://portless.dev) and Docker to manage local dev services and per-worktree Postgres databases.

- **Stable HTTPS URLs** for every dev service (`https://api.localhost`) via portless — no port juggling
- **One command** (`dx up`) starts your whole stack in the background, idempotently
- **Unified logs** (`dx logs -f`) — every service in one stream, colored per service, with a clean ANSI-free file for grep/AI
- **Git worktree lifecycle** (`dx worktree create/rm`) — each branch gets its own worktree, its own Postgres database (forked from the primary), and its own URL namespace (`api-<branch>.localhost`)
- **Cross-service URL injection** — declare `VITE_API_URL = "api"` in `dx.toml` and every checkout gets the right URL automatically
- **Raycast extension** (optional) to list / open / stop services

## Requirements

- **portless** — `npm install -g portless`; the proxy must be running (`portless proxy start`)
- **docker** — only for `dx db` (per-worktree Postgres)
- **git** — for `dx worktree`
- macOS or Linux (the Raycast extension is macOS-only)

## Install

```
go install github.com/agarichan/dx/cmd/dx@latest
```

Or from source:

```
git clone https://github.com/agarichan/dx && cd dx
mise install      # install the pinned Go toolchain (first time only)
mise run install  # build and copy the binary to ~/.local/bin/dx
```

## Quick Start

```sh
npm install -g portless
portless proxy start        # HTTPS proxy on 443 (once; auto-elevates)
```

Drop a `dx.toml` at your repo root:

```toml
[service.web]
command = ["npm", "run", "dev", "--", "--port", "{port}"]
```

Then:

```sh
dx up               # start everything in the background
open https://web.localhost
dx logs -f          # unified live logs
dx status           # what's running
dx down             # stop this checkout's services
```

`{port}` is replaced with a free port dx allocates; portless routes `https://web.localhost` to it. Add more `[service.<key>]` entries and they all come up with `dx up`.

## Raycast Extension (optional)

dx ships with a Raycast extension (list / open / stop dx services). The source
is embedded in the binary — no repo checkout needed:

```
dx raycast install      # extract + npm ci + import into Raycast
dx raycast uninstall    # remove extracted files (drop the Raycast entry via UI)
```

Requires `npm` on `$PATH` (same prerequisite as portless) and Raycast.app
running. Re-run `dx raycast install` after upgrading dx to update the
extension.

## Environment Variables

With zero configuration dx matches plain portless (`portless proxy start`, HTTPS on 443): every service gets `https://<svc>.localhost`, and `pub` URLs fall back to the internal form. Set these only if you run a custom local TLD, a non-default proxy port, or an externally reachable domain (e.g. via Tailscale or a reverse proxy).

| Variable | Default | Description |
|---|---|---|
| `DX_PUBLIC_DOMAIN` | *(unset)* | Domain suffix for public URLs (`<svc>.<domain>`). Unset: public URLs fall back to the internal URL. |
| `DX_INTERNAL_DOMAIN` | `localhost` | TLD for internal URLs (`<svc>.<domain>`) |
| `DX_PROXY_PORT` | `443` | Port portless listens on. `443` is omitted from generated URLs. |

Example — custom TLD `.lan` on port 1355 plus a public wildcard domain:

```sh
export DX_INTERNAL_DOMAIN=lan      # internal: https://<svc>.lan:1355
export DX_PROXY_PORT=1355
export DX_PUBLIC_DOMAIN=dev.example.com  # pub: https://<svc>.dev.example.com
```

## Configuration — dx.toml

Place `dx.toml` at the **repo root** and commit it. It is the single config source for all `dx` commands. Secrets (DSN values) stay in env vars; only the env var *name* appears in dx.toml.

```toml
# dx.toml (repo root)
[worktree]
dir = ".worktrees"             # where dx worktree create places new worktrees (default: .claude/worktrees)

# copy steps seed files from the primary checkout into a new worktree (before init).
[[worktree.copy]]
from = ".env.local"            # file or directory, relative to primary root; skipped if missing

# init steps run after `dx worktree create` (and DB fork) succeed, in order, fail-fast.
[[worktree.init]]
command = ["pnpm", "install"]
dir     = "webapp"             # relative to the new worktree root (default: root)

[db]
container = "myapp-postgres"   # docker container name
dsn       = "postgres://myapp:pw@localhost:5432/myapp"  # inline base DSN (preferred for dx db url)
# url_env = "DATABASE_URL"     # alternative: env var holding the base DSN (either dsn or url_env required)
image     = "postgres:18"      # image for dx db up (default: postgres:18)
volume    = "myapp-pgdata"     # named volume for dx db up

[service.api]
name    = "myapp-api"          # portless registration base name (defaults to key "api" if omitted)
command = ["uvicorn", "app:app", "--port", "{port}", "--reload"]
db      = true                 # idempotent DB fork for worktrees (requires [db])
dir     = "api"                # run in <repo root>/api
pub     = { API_URL = "self", WEBAPP_URL = "web" }

[service.web]
# name omitted → defaults to key "web" (portless name: web / web-<branch>)
command  = ["vite", "--port", "{port}"]
dir      = "webapp"            # run in <repo root>/webapp
internal = { VITE_API_URL = "api" }   # "api" is the key of the api service above
```

Services are declared as `[service.<key>]` map entries. The key is used:
- as the argument to `dx serve <key>` and `dx logs <key>`,
- as the reference in `pub`/`internal` tables (e.g. `VITE_API_URL = "api"`),
- as the default portless base name when `name` is omitted.

### Schema reference

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `worktree.dir` | string | optional (default `.claude/worktrees`) | Worktree placement dir, relative to primary root |
| `worktree.copy` | array of tables | optional | Files/dirs seeded from the primary checkout into a new worktree, before init. Missing source → skipped; existing destination → skipped (no overwrite); symlinks preserved. |
| `worktree.copy[].from` | string | required | Source relative to primary root; also the destination path in the new worktree. |
| `worktree.init` | array of tables | optional | Commands run after `dx worktree create` succeeds (after DB fork and copy), in order, fail-fast. Child env adds `DX_WORKTREE_BRANCH` / `DX_WORKTREE_PATH` / `DX_PRIMARY_ROOT`. |
| `worktree.init[].command` | string array | required | argv. Shell is not involved. |
| `worktree.init[].dir` | string | optional (default: worktree root) | Working dir relative to the new worktree root. Must be relative and stay inside the worktree (no absolute paths, no `..`). |
| `db` | table | optional | Omit if no managed DB |
| `db.container` | string | required if `[db]` present | Docker container name |
| `db.dsn` | string | `dsn` or `url_env` required | Inline base DSN. Preferred when using `dx db url` with the mise `{{ exec }}` pattern to avoid recursive env evaluation. |
| `db.url_env` | string | `dsn` or `url_env` required | Env var name holding the base DSN. Used as fallback when `dsn` is empty. |
| `db.image` | string | optional (default `postgres:18`) | Image for `dx db up` |
| `db.volume` | string | required for `dx db up` | Named volume for data persistence |
| `service.<key>` | table | — | Declare a service with map key `<key>`. Key is used for `dx serve`/`dx logs` lookup and as the default `name`. |
| `service.<key>.name` | string | optional (defaults to `<key>`) | Portless registration base name (e.g. `myapp-api`). Primary uses this name; linked worktrees get `<name>-<branch>`. Omit to use the key itself. |
| `service.<key>.command` | string array | required | argv. `{port}` is replaced with the port allocated by dx. Shell is not involved. |
| `service.<key>.db` | bool | optional | If true, perform an idempotent DB fork for the worktree (no-op on primary). The child process inherits the DB env as set by mise — `dx serve` does not rewrite it. Requires `[db]`. |
| `service.<key>.dir` | string | optional (default: repo root) | Working directory for the service process, relative to the repo root. Omit to use the repo root. |
| `service.<key>.pub` | table | optional | `ENV = ref`. Injects the public URL of `ref` as `ENV`. `ref` is another service's key, `"self"` for the current service, or a literal portless base name as fallback. |
| `service.<key>.internal` | table | optional | Same as `pub` but injects the internal URL. |

## Commands

### `dx serve <key>`

Start one service from `dx.toml` by its map key (e.g. `dx serve api`). Idempotent — if already running, prints the PID and exits cleanly.

`$PORT` is set by portless and passed to the child process automatically via the `{port}` token in `command`.

### `dx up`

Start **all** `[service.<key>]` entries from `dx.toml` in deterministic (sorted-key) order. This is the primary entry point for bringing up a full dev environment. Idempotent — already-running services are left untouched.

### `dx down`

Stop all running services for the current checkout and remove their registry entries.

### `dx logs [-f] [--no-color] [name]`

Multiplexes the log files of all registered services into a single stream. Each line is prefixed with the service name (padded and aligned), the capture timestamp from the `.color.log` file (e.g. `12:07:23.567`), and a `|` separator:

```
myapp-api 12:07:23.567 | server started on port 8000
myapp-web 12:07:24.012 | ready in 320ms
```

Color is assigned per-service automatically, cycling through a fixed palette — disabled when stdout is not a TTY or when `--no-color`/`--plain` is passed. The timestamp appears only when reading the `.color.log` (i.e. terminal mode); plain mode reads the timestamp-free `.log`.

| Flag/Arg | Description |
|---|---|
| `-f`, `--follow` | Follow logs (equivalent to `tail -F`) |
| `--no-color`, `--plain` | Suppress ANSI color codes |
| `key` | Tail only the service with this config key (e.g. `dx logs api`); resolved to its portless name automatically |

Services are sorted alphabetically before display. The last 10 lines of each log are shown on start.

**Two log files per service**: each service writes two files side by side:

| File | Contents | Read it when |
|---|---|---|
| `<name>.log` | **Plain canonical** — ANSI escape sequences already stripped, no timestamp | You or an AI/grep read the log directly; `dx logs` when piped/`--no-color` |
| `<name>.color.log` | **Raw ANSI** — the app's native colors preserved verbatim, with a line-head capture time (`HH:MM:SS.mmm`) | `dx logs` in a terminal (color display) |

A small `__logpump` process (the dx binary re-invoked) sits between each service and its files: it reads the child's combined stdout+stderr line by line and fans each line out — with a capture timestamp prefix to `.color.log`, ANSI-stripped and timestamp-free to `.log`. So the `.log` is always clean (no escape codes, no timestamps), while `.color.log` keeps the color and records when each line arrived.

`dx logs` picks the file automatically: in a terminal it tails `.color.log` (falling back to `.log` for older services that lack one); when stdout is not a TTY or `--no-color`/`--plain` is given it tails the plain `.log`. **AI/grep should read the `.log` directly.**

**Color preservation**: `dx serve`/`dx up` inject `FORCE_COLOR=1` and `CLICOLOR_FORCE=1` into each service's environment so that tools like Vite, Jest, and Vitest emit their native ANSI colors, which the pump records into `.color.log`.

> **Note for uvicorn**: uvicorn ignores `FORCE_COLOR`/`CLICOLOR_FORCE`. Pass `--use-colors` explicitly in its `command` in `dx.toml` to get colored uvicorn output.

### `dx status`

List all registered services for the current checkout with their running/stopped state, PID, and log path. `--all` spans every checkout; `--json` emits a machine-readable array.

### `dx stop <name>`

Stop one named service.

### `dx db <subcommand>`

Manage the Postgres container and per-worktree databases. Configuration comes from `[db]` in `dx.toml`.

**Subcommands:**

| Subcommand | Description |
|---|---|
| `up` | Start the container if not running (idempotent) |
| `down` | Stop and remove the container (data in the named volume persists) |
| `psql` | Open a `psql` session against the worktree DB |
| `fork` | CREATE DATABASE ... TEMPLATE from the primary DB (worktree only) |
| `drop` | Drop the worktree DB |
| `reset` | Drop then re-fork (worktree only) |
| `list` | List all databases whose names start with the base DB name |
| `url` | Print the per-checkout DSN (worktree-aware) to stdout |

`fork` and `reset` are rejected on a primary checkout — run them from a linked worktree.

#### `dx db url` — per-checkout DSN

Prints the full connection URL for the current checkout:

```
$ dx db url
postgres://myapp:pw@localhost:5432/myapp_feat_x
```

The DB name is derived from the current worktree branch. On the primary checkout it returns the base DSN unchanged.

**mise `{{ exec }}` pattern** — wire `dx db url` into `mise.toml` so every mise task automatically uses the correct worktree DB:

```toml
# mise.toml
[env]
DATABASE_URL = "{{ exec(command='dx db url') }}"
```

**Recursion avoidance**: if `url_env` in `dx.toml` references the same env var that the `{{ exec }}` pattern sets, recursive evaluation will occur. Avoid this by using the inline `dsn` field in `dx.toml` instead of `url_env`. With `dsn`, dx reads the DSN directly from the config file and never touches the env during `dx db url`.

**`dx serve` and DB env**: `dx serve` does **not** rewrite the DB env var in the child process environment. The child process inherits whatever env mise has already set (including the `{{ exec }}`-resolved `DATABASE_URL`). The `service[].db = true` flag only triggers an idempotent DB fork — it does not modify the env.

### `dx worktree <subcommand>`

Manage the full lifecycle of git worktrees: create, remove, and list. All worktree subcommands must be run from the **primary checkout**.

#### `dx worktree create <branch> [--from <base-branch>] [--skip-init]`

Create a new linked worktree at `<worktree.dir>/<branch>`, fork the primary DB into the worktree-specific DB, seed `[[worktree.copy]]` files, then run the `[[worktree.init]]` steps in order (fail-fast).

| Flag | Description |
|------|-------------|
| `--from <branch>` | Start point for a new branch. Only valid when the branch does not yet exist. |
| `--skip-init` | Skip the configured `[[worktree.init]]` steps (copy still runs). |

Each init step runs in the new worktree root (or `<root>/<dir>` when `dir` is set) with the parent env plus `DX_WORKTREE_BRANCH`, `DX_WORKTREE_PATH`, and `DX_PRIMARY_ROOT`. The child's stdout/stderr stream through.

**Exit codes:**
- `0` — worktree created, DB forked, and all init steps succeeded (fully prepared).
- `3` — worktree created but a follow-up step did not finish: DB fork skipped/failed, **or** an init step failed. The worktree is kept; fix and re-run the remaining step manually.
- `1` — fatal error (nothing was created).

After a successful `create`, start services with `dx up` from inside the new worktree (or `dx serve <name>`).

#### `dx worktree rm <branch> [--force] [--keep-db] [--delete-branch]`

Stop services, drop the worktree DB, and remove the git worktree — in that order. All precondition checks run before any destructive action.

| Flag | Description |
|------|-------------|
| `--force` | Remove even if the worktree has uncommitted changes |
| `--keep-db` | Skip DB drop |
| `--delete-branch` | Also run `git branch -d <branch>` after removal |

#### `dx worktree list [--json]`

Show all git worktrees with their DB status and service states side by side.

### `dx raycast <install|uninstall>`

Install or remove the bundled Raycast extension. See [Raycast Extension](#raycast-extension-optional).

### `dx version`

Print the current version string.

### `dx -h` / `dx --help`

Print the built-in help text listing all commands and flags. Also works after any subcommand (e.g. `dx logs -h`).

## State Directory

Runtime state (per-service JSON records and log files) is stored under:

```
$XDG_STATE_HOME/dx/<checkout-hash>/
~/.local/state/dx/<checkout-hash>/     # fallback when XDG_STATE_HOME is unset
```

The checkout hash is derived from the git worktree root, so each linked worktree gets its own isolated registry.

## AI Agent Integration

To make coding-agent worktree skills (e.g. superpowers' using-git-worktrees / finishing-a-development-branch) go through dx, add the following to your project's CLAUDE.md / AGENTS.md — don't edit the skill files themselves (instructions take precedence over skills):

```markdown
## Worktree workflow (dx)
- Create isolated workspaces with `dx worktree create <branch>` (not plain `git worktree add`).
  Exit code 3 means "worktree created, DB needs recovery" (cd into the worktree and run `dx db fork`).
- Clean up with `dx worktree rm <branch>`, including in the cleanup phase of finishing-a-development-branch.
```

With `dx.toml` at the repo root, serve/db/up/worktree all share the same config.

## Development

| Task | What it does |
|---|---|
| `mise run build` | Build `./dx` |
| `mise run test` | Unit tests (fast; docker/network-gated integration excluded) |
| `mise run test:integration` | Integration tests (requires docker/postgres, npm) |
| `mise run lint` | `gofmt -w .` then `go vet ./...` |
| `mise run check` | `lint` + `test` (pre-commit check) |
| `mise run install` | Build and install to `~/.local/bin/dx` |

## License

[MIT](LICENSE)
