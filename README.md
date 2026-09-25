# makeutility

A team repo registry API and a concurrent workspace sync CLI for Go
teams managing many GitHub repos.

Most engineering teams end up with a growing pile of repos (services,
shared libraries, infra configs) and no reliable answer to two
questions: "which repos are actually ours" and "are they all up to
date and clean right now". `makeutility` answers both. A small,
secured JSON API is the single source of truth for the repo list, and
a CLI uses concurrent goroutines to clone, fetch, pull, and status-check
every repo in seconds instead of minutes.

## Features

- Secured JSON REST API (`makeutility-api`) backed by PostgreSQL,
  replacing an out-of-date spreadsheet or wiki page as the source of
  truth for the team's repo list.
- Concurrent `sync`: clones missing repos and fetches/pulls existing
  ones in parallel via a bounded goroutine worker pool.
- Concurrent `status`: reports branch, ahead/behind counts, and
  dirty/clean state for every local repo in one table.
- Works offline: the CLI falls back to a local `repos.yaml` file if
  the API is unreachable.
- Type-safe data access generated from plain SQL via `sqlc` (no ORM,
  no runtime reflection).
- Fully containerized with digest-pinned base images for reproducible
  builds.

## Quick start (Docker)

Requirements: Docker and Docker Compose.

```bash
docker compose up -d
```

This starts PostgreSQL (with the schema applied automatically) and
`makeutility-api` on port 8080, using a default local dev API key of
`dev-local-only-key`. Set `MAKEUTILITY_API_KEY` in your shell before
running `docker compose up` to override it.

Verify it is running:

```bash
curl http://localhost:8080/healthz
```

Stop everything with `docker compose down` (add `-v` to also delete
the PostgreSQL data volume).

## Usage

Create, list, and update repos through the API:

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

Then run the CLI from the directory where you want repos managed:

```bash
export MAKEUTILITY_API_URL="http://localhost:8080"
export MAKEUTILITY_API_KEY="dev-local-only-key"

go run ./cmd/makeutility sync
go run ./cmd/makeutility status
```

If `MAKEUTILITY_API_URL`/`MAKEUTILITY_API_KEY` are unset, or the API is
unreachable, the CLI falls back to the local `repos.yaml` file.

## Running locally without Docker

Requirements: Go 1.27+, a running PostgreSQL instance, and the `sqlc`
CLI if you plan to change the schema or queries.

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

3. Run the CLI, as shown in the Usage section above.

## Configuration

| Variable | Used by | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | `makeutility-api` | `postgres://postgres:postgres@localhost:5432/makeutility?sslmode=disable` | PostgreSQL connection string. |
| `MAKEUTILITY_API_KEY` | both | none (required) | Shared secret sent as the `X-API-Key` header on every `/repos` request. |
| `MAKEUTILITY_API_ADDR` | `makeutility-api` | `:8080` | Address the API listens on. |
| `MAKEUTILITY_API_URL` | CLI | none | Base URL of `makeutility-api`. If unset, the CLI uses `repos.yaml`. |
| `MAKEUTILITY_WORKSPACE` | CLI | `.` (current directory) | Directory where repos are cloned or checked. |

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

## Tech stack

| Layer | Choice |
|---|---|
| HTTP router | chi (github.com/go-chi/chi) v5 |
| Data access | sqlc, generating against pgx/v5 |
| Database | PostgreSQL |
| CLI concurrency | Go standard library (goroutines, golang.org/x/sync/errgroup) |
| Config fallback | repos.yaml (YAML via go.yaml.in/yaml/v3) |

Base images in `Dockerfile` and `docker-compose.yml` are pinned to both
a tag and an OCI index digest (for example `postgres:18.6@sha256:...`)
so builds are reproducible and not silently affected by an upstream
tag being overwritten.

## Development

```bash
go build ./...
go vet ./...
golangci-lint run ./...
```

`.golangci.yml` enables the standard linter set plus revive rules
chosen to match Google's Go Style Guide
(google.github.io/styleguide/go/decisions): required doc comments on
exported names and packages, lowercase and no-punctuation error
strings, consistent receiver naming, early return over nested error
handling, and `context.Context` as the first parameter.

To regenerate the sqlc code after editing `internal/db/schema.sql` or
`internal/db/query.sql`:

```bash
sqlc generate
```

### Testing

Unit tests are fast and dependency-free:

```bash
go test ./...
go test ./... -race
```

Handler tests that need a real database are integration tests, kept
behind a build tag so `go test ./...` never needs Docker. They start a
disposable PostgreSQL container (via testcontainers-go), apply the
real schema, and exercise the actual HTTP handlers -- including the
database's own UNIQUE constraint, not a mocked version of it:

```bash
go test -tags=integration ./internal/api/... -v
```

Requires a running Docker daemon; there is no Docker-less fallback for
this tier, unlike the CLI's own repos.yaml fallback.

## Design notes

This project is intentionally scoped lean for a one-sprint internal
tool serving a few dozen engineers, rather than gold-plated.

In place now:

- A single shared API-key middleware (not JWT/RBAC).
- Plain JSON responses and standard HTTP status codes, not the full
  `application/problem+json` (RFC 9457) error envelope.
- A flat `cmd`/`internal` layout rather than a domain-driven
  `internal/repository,rest,service` split, since there is currently
  one resource (`repos`).

Deliberately deferred, as reasonable next steps if this grows beyond
one sprint's worth of usage:

- RFC 9457 error envelopes.
- Cursor-based pagination on `GET /repos` (a non-issue at dozens of
  repos, but the right pattern once the registry grows).
- An OpenAPI 3.1 contract so other tools can generate clients.
- Rate limiting, JWT/RBAC, and audit logging.
- `POST /repos/import` (bulk import from a spreadsheet) and
  `makeutility open <repo>`.

## Contributing

Run `go build ./...`, `go vet ./...`, `golangci-lint run ./...`, and
`go test ./... -race` before submitting changes. If you touched
`internal/api`, also run the integration suite (see Testing above).

## License

MIT. See [LICENSE](LICENSE).
