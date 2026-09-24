# makeutility

A team repo registry service and a concurrent workspace sync CLI, built to
cut down on the daily busywork of managing a growing list of team GitHub
repos: cloning them all, keeping them up to date, and knowing which ones
are dirty or behind `main`.

`makeutility` has two parts:

- **`makeutility-api`**: a small, secured JSON REST API backed by
  PostgreSQL. It is the single source of truth for "which repos does our
  team own" (name, URL, owner, tags, active status), replacing an
  out-of-date spreadsheet or wiki page.
- **`makeutility`** (the CLI): the tool engineers run day-to-day.
  - `makeutility sync`: fetches the repo list from the API and clones or
    fetches/pulls every repo concurrently, using a bounded pool of
    goroutines.
  - `makeutility status`: concurrently reports each local repo's branch,
    ahead/behind counts, and dirty/clean state in one table.

## Tech stack

| Layer | Choice |
|---|---|
| HTTP router | [`chi`](https://github.com/go-chi/chi) v5 |
| Data access | [`sqlc`](https://sqlc.dev) generating against `pgx/v5` |
| Database | PostgreSQL |
| CLI concurrency | Go standard library (goroutines, `golang.org/x/sync/errgroup`) |
| Config fallback | `repos.yaml` (YAML via `go.yaml.in/yaml/v3`) |

See the "Design notes" section below for why these were chosen, and what
was deliberately deferred to keep this scoped to one sprint.

## Quick start with Docker

The fastest way to run everything (Postgres plus the API) is Docker
Compose:

```bash
docker compose up -d
```

This starts Postgres (with `internal/db/schema.sql` applied
automatically) and `makeutility-api` on port 8080, using a default local
dev API key of `dev-local-only-key`. Override it by setting
`MAKEUTILITY_API_KEY` in your shell before running `docker compose up`.

Check it is healthy:

```bash
curl http://localhost:8080/healthz
```

Try the API:

```bash
# List repos
curl -H "X-API-Key: dev-local-only-key" http://localhost:8080/repos/

# Create a repo
curl -X POST -H "X-API-Key: dev-local-only-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"example-service","url":"https://github.com/example-org/example-service.git","owner":"platform-team"}' \
  http://localhost:8080/repos/

# Update a repo
curl -X PATCH -H "X-API-Key: dev-local-only-key" \
  -H "Content-Type: application/json" \
  -d '{"active":false}' \
  http://localhost:8080/repos/1
```

Stop everything with `docker compose down` (add `-v` to also delete the
Postgres data volume).

Base images in `Dockerfile` and `docker-compose.yml` are pinned to both a
tag and an OCI index digest (e.g. `postgres:18.6@sha256:...`) so builds
are reproducible and not silently affected by an upstream tag being
overwritten.

## Running locally without Docker

Requirements: Go 1.27+, a running PostgreSQL instance, and the `sqlc` CLI
if you plan to change the schema/queries.

1. Apply the schema to your database:

   ```bash
   psql "$DATABASE_URL" -f internal/db/schema.sql
   ```

2. Run the API:

   ```bash
   export DATABASE_URL="postgres://user:pass@localhost:5432/makeutility?sslmode=disable"
   export MAKEUTILITY_API_KEY="dev-local-only-key"
   go run ./cmd/makeutility-api
   ```

3. Run the CLI against it, from the directory where you want repos
   cloned:

   ```bash
   export MAKEUTILITY_API_URL="http://localhost:8080"
   export MAKEUTILITY_API_KEY="dev-local-only-key"
   go run ./cmd/makeutility sync
   go run ./cmd/makeutility status
   ```

   If `MAKEUTILITY_API_URL`/`MAKEUTILITY_API_KEY` are not set, or the API
   is unreachable, the CLI falls back to the local `repos.yaml` file.

## Repository layout

```
makeutility/
  cmd/
    makeutility/        CLI entrypoint (sync, status)
    makeutility-api/    API service entrypoint
  internal/
    api/                chi routes, handlers, API-key middleware
    db/                 sqlc-generated code, schema.sql, query.sql
    gitops/             goroutine worker pool: clone/fetch/status logic
  repos.yaml            local fallback repo list for the CLI
  sqlc.yaml
  Dockerfile
  docker-compose.yml
  .golangci.yml
```

## Development

```bash
go build ./...
go vet ./...
golangci-lint run ./...
```

`.golangci.yml` enables the standard linter set plus `revive` rules
chosen to match
[Google's Go Style Guide](https://google.github.io/styleguide/go/decisions):
required doc comments on exported names and packages, lowercase/
no-punctuation error strings, consistent receiver naming, early return
over nested error handling, and `context.Context` as the first
parameter.

To regenerate the `sqlc` code after editing `internal/db/schema.sql` or
`internal/db/query.sql`:

```bash
sqlc generate
```

## Design notes

This project is intentionally scoped lean for a one-sprint internal
tool serving a few dozen engineers, rather than gold-plated:

**In place now:**

- A single shared API-key middleware (not JWT/RBAC).
- Plain JSON responses and standard HTTP status codes, not the full
  `application/problem+json` (RFC 9457) error envelope.
- A flat `cmd/`/`internal/` layout rather than a domain-driven
  `internal/{repository,rest,service}` split, since there is currently
  one resource (`repos`).

**Deliberately deferred** (reasonable next steps if this grows beyond
one sprint's worth of usage):

- RFC 9457 error envelopes.
- Cursor-based pagination on `GET /repos` (a non-issue at dozens of
  repos, but the right pattern once the registry grows).
- An OpenAPI 3.1 contract so other tools can generate clients.
- Rate limiting, JWT/RBAC, and audit logging.
- `POST /repos/import` (bulk import from a spreadsheet) and
  `makeutility open <repo>`.
