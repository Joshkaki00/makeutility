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
- `GET /repos` returns **active** repos only, so deactivating a row
  drops it from the CLI sync/status list without deleting history.
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
- Godoc comments on packages and exported APIs (`Package` / name-first
  style), including `BUG(Joshkaki00)` notes for known limitations.

## Quick start (Docker)

Requirements: Docker and Docker Compose.

```bash
docker compose up -d
```

This starts PostgreSQL 18 (schema applied on first boot via
`internal/db/schema.sql`) and `makeutility-api` on port 8080, using a
default local dev API key of `dev-local-only-key`. Set
`MAKEUTILITY_API_KEY` in your shell before running `docker compose up`
to override it.

Verify it is running:

```bash
curl http://localhost:8080/healthz
```

Stop everything with `docker compose down` (add `-v` to also delete
the PostgreSQL data volume).

## Usage

Create, list, and update repos through the API:

```bash
# List active repos
curl -H "X-API-Key: dev-local-only-key" http://localhost:8080/repos/

# Create a repo
curl -X POST -H "X-API-Key: dev-local-only-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"example-service","url":"https://github.com/example-org/example-service.git","owner":"platform-team"}' \
  http://localhost:8080/repos/

# Deactivate a repo (it disappears from GET /repos and from the CLI list)
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

Repo `name` values must be a single path segment (no `..` or `/`); the
CLI rejects names that would escape the workspace directory.

## Running locally without Docker

Requirements: Go 1.27+, a running PostgreSQL instance, and the `sqlc`
CLI if you plan to change the schema or queries.

1. Apply the schema to your database:

   ```bash
   psql "$DATABASE_URL" -f internal/db/schema.sql
   ```

2. Run the API:

   ```bash
   # Match Compose Postgres credentials when using only that container:
   export DATABASE_URL="postgres://makeutility:makeutility@localhost:5432/makeutility?sslmode=disable"
   # Or use the binary's built-in default (postgres/postgres) on a local install:
   # export DATABASE_URL="postgres://postgres:postgres@localhost:5432/makeutility?sslmode=disable"
   export MAKEUTILITY_API_KEY="dev-local-only-key"
   go run ./cmd/makeutility-api
   ```

3. Run the CLI, as shown in the Usage section above.

## Configuration

| Variable | Used by | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | `makeutility-api` | `postgres://postgres:postgres@localhost:5432/makeutility?sslmode=disable` | PostgreSQL connection string. Compose sets `postgres://makeutility:makeutility@postgres:5432/makeutility?sslmode=disable`. |
| `MAKEUTILITY_API_KEY` | both | none (required) | Shared secret sent as the `X-API-Key` header on every `/repos` request. |
| `MAKEUTILITY_API_ADDR` | `makeutility-api` | `:8080` | Address the API listens on. |
| `MAKEUTILITY_API_URL` | CLI | none | Base URL of `makeutility-api`. If unset, the CLI uses `repos.yaml`. |
| `MAKEUTILITY_WORKSPACE` | CLI | `.` (current directory) | Directory where repos are cloned or checked. |

## Repository layout

```
makeutility/
  cmd/
    makeutility/          CLI entrypoint (sync, status)
    makeutility-api/      API service entrypoint
  internal/
    api/                  chi routes, handlers, middleware + tests
    db/                   schema.sql, query.sql, sqlc output, doc.go
    gitops/               sync/status/config/apiclient + tests
  repos.yaml              local fallback repo list for the CLI
  sqlc.yaml
  Dockerfile
  docker-compose.yml
  .golangci.yml
  RUBRIC.md
```

## Tech stack

| Layer | Choice |
|---|---|
| HTTP router | chi (github.com/go-chi/chi) v5 |
| Data access | sqlc, generating against pgx/v5 |
| Database | PostgreSQL 18 |
| CLI concurrency | goroutines + golang.org/x/sync/errgroup |
| Config fallback | repos.yaml (YAML via go.yaml.in/yaml/v3) |
| Unit/assert helpers | testify |
| Integration tests | testcontainers-go (Postgres module) |

Base images in `Dockerfile` and `docker-compose.yml` are pinned to both
a tag and an OCI index digest (for example `postgres:18.6@sha256:...`)
so builds are reproducible and not silently affected by an upstream
tag being overwritten. Postgres 18 stores data under
`/var/lib/postgresql` (not `/var/lib/postgresql/data`).

## Development

```bash
go build ./...
go vet ./...
golangci-lint run ./...
go test ./... -race
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

Browse package docs locally (classic `godoc`; use a free port if the
API already owns `:8080`):

```bash
go install golang.org/x/tools/cmd/godoc@v0.25.0
godoc -http=:6060
# open http://localhost:6060/pkg/github.com/Joshkaki00/makeutility/
```

### Testing

Unit tests live under `internal/gitops` and `internal/api`. They are
table-driven where it helps and stay fast — no Docker required:

- **api**: API-key middleware, JSON mappers, `/healthz`, create/update
  validation (bad JSON, missing fields, non-numeric id)
- **gitops**: YAML load, `FetchReposFromAPI` (httptest status/JSON
  edges), `Status` against real temp git repos, `Sync` (clone, fetch,
  bad remote, path-escaping names, `maxConcurrent=0`)

```bash
go test ./...
go test ./... -race
```

`BenchmarkStatus` measures the concurrent status path across several
repo-count / concurrency shapes:

```bash
go test ./internal/gitops/... -bench=BenchmarkStatus -benchtime=1x -run '^$'
```

Handler tests that need a real database are integration tests, kept
behind a build tag so `go test ./...` never needs Docker. They start a
disposable PostgreSQL container (via testcontainers-go), apply the
real schema, and exercise the actual HTTP handlers — including UNIQUE
constraint behavior, partial PATCH, inactive-repo exclusion from
`GET /repos`, and auth rejection:

```bash
go test -tags=integration ./internal/api/... -v -count=1
```

`-count=1` disables the test cache so a prior "Docker unavailable"
skip is not reused after you start Docker. Requires a running Docker
daemon; there is no Docker-less fallback for this tier, unlike the
CLI's own repos.yaml fallback.

## Design notes

This project is intentionally scoped lean for a one-sprint internal
tool serving a few dozen engineers, rather than gold-plated.

In place now:

- A single shared API-key middleware (not JWT/RBAC).
- Plain JSON responses and standard HTTP status codes, not the full
  `application/problem+json` (RFC 9457) error envelope.
- Active-only listing via sqlc `ListActiveRepos` (deactivate with
  `PATCH`, do not delete).
- A flat `cmd`/`internal` layout rather than a domain-driven
  `internal/repository,rest,service` split, since there is currently
  one resource (`repos`).
- Workspace path safety for repo names (`safeRepoPath`) and a
  positive concurrency floor so `errgroup.SetLimit(0)` cannot hang.

Known limitations (also tagged `BUG(Joshkaki00)` in source for godoc):

- Duplicate repo names currently surface as HTTP 500, not 409.
- PATCH with `"url":""` / `"owner":""` clears the field; omit the key
  to leave a value unchanged.
- Repos with no upstream report Ahead/Behind as 0 (same as in-sync).

Deliberately deferred, as reasonable next steps if this grows beyond
one sprint's worth of usage:

- RFC 9457 error envelopes.
- Keyset pagination on `GET /repos` (a non-issue at dozens of
  repos, but the right pattern once the registry grows).
- An OpenAPI 3.1 contract so other tools can generate clients.
- Rate limiting, JWT/RBAC, and audit logging.
- `POST /repos/import` (bulk import from a spreadsheet) and
  `makeutility open <repo>`.

## Contributing

This project is not accepting external contributions. Issues and pull
requests will not be reviewed.

## License

MIT. See [LICENSE](LICENSE).
