# Raycast: environment rows + `open` flag — design

Date: 2026-07-03
Status: approved

## Goal

The Raycast extension currently renders one row per service, exposing every URL
(including backend APIs nobody opens in a browser). Change it to one row per
**environment** (checkout root): pressing Enter opens the designated front-end
URL, while every service's aliveness stays visible as compact status dots.

## Decisions

- **`open` flag in dx.toml** designates the service whose URL the row opens:
  `[service.web] open = true`. Explicit and portable; at most one service may
  set it (validation error otherwise).
- **Fallback** when no service has `open` (or registry records predate the
  field): a single-service group opens that service; multi-service groups open
  the alphabetically-first name.
- API URLs are no longer shown in the list; per-service state dots replace them.

## Changes

### dx (Go)

1. `internal/project`:
   - `Service.Key string` (`toml:"-"`) — the map key, populated by `Load`
     alongside the existing Name-defaulting loop.
   - `Service.Open bool` (`toml:"open"`).
   - `Validate()`: error if more than one service sets `open = true`.
2. `internal/registry`:
   - `Service.Key string` (`json:"key,omitempty"`), `Service.Open bool`
     (`json:"open,omitempty"`) — recorded at serve time.
3. `internal/cli`:
   - `startService` copies `svc.Key` / `svc.Open` into the registry record.
   - `statusEntry` gains `Key string` (`json:"key"`) and `Open bool`
     (`json:"open"`), sourced from the registry record. Old records simply
     yield `"" / false`.

### Raycast extension (raycast/)

- `src/dx.ts`: `Service` gains `key?: string; open?: boolean`. Add
  `groupByRoot(services): Env[]` where
  `Env = { root, services, openSvc, state: "running"|"partial"|"stopped" }`.
  `openSvc` = the service with `open`, else the single service, else
  alphabetically-first by name. `state` = all running / some / none.
- `src/list-services.tsx`: one `List.Item` per Env:
  - icon: green (all running) / yellow (partial) / gray (stopped)
  - title: `openSvc.name`; subtitle: root with `$HOME` abbreviated to `~`
  - accessories: per service, a colored dot + short label (`key`, falling back
    to `name`)
  - actions: Open (openSvc.url) / Copy URL / Stop All (⌘X, stops every service
    in the env sequentially) / submenu with per-service Stop / Refresh (⌘R)

### Docs

- README schema table: add `service.<key>.open` row.

## Testing

- project: Load populates Key; Open decodes; Validate rejects two `open`.
- registry: Key/Open round-trip through Put/Get.
- cli: status --json includes key/open (via a Put + runStatus test).
- extension: `groupByRoot` unit-testable pure function; UI verified via
  `ray build` (integration test already covers compile) and manual check.

## Out of scope

- Starting environments from Raycast, log viewing, per-env `dx down` wiring.
