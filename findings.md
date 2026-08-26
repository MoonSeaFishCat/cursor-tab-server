# Findings

## Current repository

- The existing project is a non-Git Go module named `cursor-tab-server`.
- `main.go` contains all proxy behavior in one file and has no test files.
- The only existing dependency is `gopkg.in/yaml.v3`.
- Docker currently uses a static Go build and a `scratch` runtime image, but copies `config.yaml` into the runtime image.

## Confirmed product requirements

- All proxy requests require a managed calling API Key.
- `/admin/*` requires an administrator session; normal calling keys have no management access.
- SQLite persists calling keys, opaque administrator session hashes, and audit records.
- Audit records retain metadata only, expire after 30 days, and never contain secrets or bodies.
- Administration is public-facing only through an HTTPS reverse proxy; the Go server binds to loopback by default.
- The UI is a React + Vite app using shadcn/ui, embedded in the Go binary.
- The selected UI is a fixed-sidebar, neutral, spacious administration console.

## Visual decisions

- Navigation: fixed desktop sidebar; mobile navigation becomes a drawer.
- Pages: Login, Overview, API Keys, Audit Logs, Service Status, Settings.
- Visual style: neutral light surfaces, restrained semantic status colors, tables optimized with filtering and pagination.
