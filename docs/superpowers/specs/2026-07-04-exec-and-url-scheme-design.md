# dx exec + dx db url --scheme — design

Date: 2026-07-04
Status: approved

## Problem

Migrations (and other one-off commands) need the same env a service gets from
`db_env`, but mise `[env]` cannot provide it when dx is a mise-managed tool.
Task-level composition needs two things: a way to run a command in a service's
environment, and a scheme-overridable `dx db url`.

## Design

### `dx exec <key> [--] <command...>`
Runs `<command>` in service `<key>`'s environment, foreground:
- env = `buildEnv` (FORCE_COLOR, pub/internal URLs, `db_env` DSN) over the
  current process env.
- working dir = the service's `dir` (repo root when unset).
- Before running: the same idempotent DB fork/seed as `dx serve` when the
  service wants a DB (`db = true` or `db_env`) — a fresh worktree's first
  command is typically the migration itself.
- No portless registration, no `{port}` handling.
- Exit code of the child is propagated; `--` before the command is optional.

### `dx db url --scheme <scheme>`
`url` accepts `--scheme` and prints the per-checkout DSN with the scheme
overridden. Derivation unified with `db_env` via `dbEnvValue` (works for both
engines; postgres also honors `[db].url_env`).

## Changes

- `internal/cli/serve.go`: extract the fork/seed block of `startService` into
  `ensureServiceDB(cfg, svc, wt, stderr)` (shared with exec).
- `internal/cli/exec.go` (new): arg parsing + `execService(cfg, svc, wt,
  cmdArgs, stdout, stderr) int` core (testable without git).
- `internal/cli/db.go`: `url` case handled for both engines via `dbEnvValue`
  with an optional `--scheme` flag (unknown flags error).
- `internal/cli/dispatch.go`: `exec` command + help; `db` help mentions
  `--scheme`.
- README: `dx exec` section; migration guidance switches to
  `dx exec api -- alembic upgrade head`; `dx db url [--scheme]` documented.

## Testing

- exec core: sqlite temp cfg — child sees the injected env var, runs in the
  service dir, exit code propagated, seed runs on a linked worktree.
- db url: `--scheme` flag parsing (error on unknown flag) + scheme-overridden
  output for sqlite and postgres (dbEnvValue already unit-covered; add a
  runDB-level sqlite case).
- E2E in a scratch repo after implementation.

## Out of scope

- `dx exec` without a service key (global env); TTY allocation logic beyond
  plain stdio inheritance.
