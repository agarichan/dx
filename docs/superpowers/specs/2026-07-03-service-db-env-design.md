# service db_env — inject the per-checkout DSN into the service process

Date: 2026-07-03
Status: approved

## Problem

The documented pattern for pointing an app at the per-worktree DB was mise's
`[env]` + `{{exec(command='dx db url')}}`. That breaks when dx itself is
managed as a mise tool (`github:agarichan/dx`): mise evaluates `[env]` before
tool shims are on PATH, so `dx` is not yet resolvable.

## Design

`dx serve` injects the DSN directly into the service's environment:

```toml
[service.api]
command = ["uvicorn", "app:app", "--port", "{port}"]
db_env  = { name = "APP_DATABASE_URL", scheme = "postgresql+psycopg" }
```

- `name` (required): env var to set.
- `scheme` (optional): overrides the DSN scheme, e.g. `postgresql+psycopg`,
  `sqlite+aiosqlite`. Default: the `[db].dsn` scheme as configured (postgres)
  or `sqlite` (sqlite engine).
- Value: postgres → base DSN with the worktree-aware DB name (same derivation
  as `dx db url`); sqlite → `<scheme>:///<abs file path>` for the checkout.
- `db_env` implies the fork/seed behavior of `db = true` (a URL pointing at a
  DB that doesn't exist is useless). `db = true` stays as fork-only for
  backward compatibility.
- Validation: `db_env.name` required when the table is present; `db_env`
  requires `[db]`.

## Changes

- `internal/dburl`: `WithScheme(s)` — sets Scheme/Driver from a possibly
  `scheme+driver` string.
- `internal/project`: `Service.DBEnv *DBEnv` (`name`, `scheme`) + validation.
- `internal/cli/serve.go`:
  - `dbEnvValue(cfg, svc, wt, getenv) (value string, err error)` — pure DSN
    computation (postgres via baseDSN-with-getenv, sqlite via db.SQLite.URL).
  - `buildEnv` appends `name=value`.
  - fork/seed condition becomes `svc.DB || svc.DBEnv != nil`.
  - registry `DB` field records the injected env name.
- README: schema row for `db_env`; the mise `{{ exec }}` guidance gains a
  warning (unusable when dx is a mise-managed tool) and points services to
  `db_env`; ad-hoc task usage can do `DATABASE_URL=$(dx db url) <cmd>`.

## Testing

- dburl: WithScheme with and without `+driver`.
- project: validation (name required, requires [db]).
- cli: buildEnv injects the postgres DSN with overridden scheme and worktree
  name; sqlite value; db_env-only service triggers seed (serve fork condition).

## Out of scope

- Multiple db_env entries per service; non-DSN formats (host/port pieces).
