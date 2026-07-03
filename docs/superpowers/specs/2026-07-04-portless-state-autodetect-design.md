# Routing autodetect from portless state — design

Date: 2026-07-04
Status: approved

## Problem

dx's zero-config routing assumes the portless proxy on 443. Users who run
`portless proxy start -p 1355` (no sudo / 443 taken) get URLs without the
port. portless writes the live proxy config to `~/.portless/` — dx can follow
it automatically.

## Design

Routing resolution becomes a 3-tier fallback (env always wins):

| Field | 1. env | 2. portless state | 3. static |
|---|---|---|---|
| InternalDomain | `DX_INTERNAL_DOMAIN` | `~/.portless/proxy.tld` | `localhost` |
| ProxyPort | `DX_PROXY_PORT` | `~/.portless/proxy.port` | `443` |
| Scheme | — | `~/.portless/proxy.tls` (`0` → `http`) | `https` |
| PublicDomain | `DX_PUBLIC_DOMAIN` | — (no state exists) | unset → fallback to internal |

- `Routing` gains `Scheme`.
- `URLInternal`: `<scheme>://<name>.<domain>[:port]`; the port is omitted for
  `https`+443 and `http`+80.
- `URLPublic`: unchanged (`https://<name>.<PublicDomain>`, public domains
  imply TLS at the edge); empty PublicDomain still falls back to URLInternal.
- State files are read via an injected reader (`os.ReadFile` in production);
  missing/empty files fall through to static defaults. Stale files (proxy
  stopped) still reflect the last-used config — a better guess than 443.

## Testing

- config: env wins over state; state wins over static; missing files → static;
  tls=0 → http scheme; whitespace trimmed.
- portless: http scheme URL with port-80 omission; https:443 omission
  unchanged; custom port keeps suffix.
- E2E on this machine: with DX_* env unset, URLs must resolve to `.nao:1355`
  purely from `~/.portless/` state.

## Out of scope

- Public-domain autodetection (no portless state exists for it).
- Live proxy health checks.
