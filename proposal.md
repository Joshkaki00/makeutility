# Proposal: makeutility - a team repo registry service and concurrent workspace sync CLI

**Author:** Joshua

**Sprint:** Developer Workflow Utilities

**Status:** Draft for review (v5 - stack confirmed, implementation not yet started)

## Problem

Our team's day-to-day work is spread across a growing number of GitHub
repositories (services, shared libraries, infra configs, internal tools).
Every engineer independently re-solves the same annoying chores multiple
times a day:

- Cloning/pulling a dozen-plus repos after switching branches, onboarding,
  or coming back from PTO.
- Manually checking which repos have uncommitted changes, unpushed commits,
  or are behind `main` before starting work or before standup.
- Figuring out which repos actually belong to "our team". That list lives
  in someone's head, a pinned Slack message, or a spreadsheet that is out
  of date half the time.

Individually these are small annoyances, but multiplied across every
engineer, every day, they add up to real lost time and lost focus from
constant context-switching.

## Why this matters

- **Onboarding.** New hires currently spend their first morning manually
  cloning repos one by one from a wiki page. A single command that does
  this in seconds is a much better first impression.
- **Daily hygiene.** "Is everything committed and pushed before I close my
  laptop?" and "what changed on `main` overnight?" are questions asked
  constantly in Slack. This should be a 5-second command, not a mental
  checklist.
- **Team visibility.** A single, always-current, queryable source of
  truth for "which repos are ours", one that any tool (this CLI, a
  dashboard, a future bot) can hit as a secured JSON API, removes an
  entire class of "wait, is that repo still active?" conversations and
  stale-spreadsheet drift.

## Proposed solution

Build `makeutility` as two small, complementary Go components:

1. `makeutility-api`: a lightweight HTTP service that is the
   authoritative registry of the team's repos (name, URL, owner, tags,
   active/archived status). It exposes a small secured JSON REST API
   (`GET /repos`, `POST /repos`, `PATCH /repos/{id}`) backed by a real
   database, replacing the "spreadsheet nobody trusts" with something
   typed, versioned, and testable.
2. `makeutility` CLI: the client engineers actually run day-to-day.
   - `makeutility sync`: fetches the current repo list from the API,
     then uses a bounded pool of goroutines to clone any missing repos
     and fetch/pull all existing ones in parallel. What would take
     minutes sequentially finishes in seconds.
   - `makeutility status`: concurrently walks every local repo and
     reports, in one clean table, the current branch, ahead/behind
     counts vs. upstream, and dirty/clean state.

This mirrors both example patterns from the sprint brief directly:
goroutines doing concurrent, I/O-bound work (`sync`/`status`), and a
small Go service turning a shared data source into a secured JSON API
(`makeutility-api`) that other tools can build on later.

## Finalized technical stack

I researched the current (2026) state of the router and data-access
options under consideration before finalizing this stack.

| Layer | Choice | Why |
|---|---|---|
| HTTP router | `chi` v5 (`github.com/go-chi/chi/v5`, latest v5.3.2, Aug 2026) | 100% `net/http` compatible (`http.Handler` all the way down), about 1000 lines of code, zero external dependencies, and built for composable route groups and middleware chains, exactly the shape of a small internal CRUD API. Production-proven at Cloudflare, Heroku, and 99designs. Chosen over `gin` because we don't need `gin`'s throughput ceiling or its Go 1.25+ minimum and extra dependency surface (BSON binding, etc.) for an internal tool serving a few dozen engineers. Chosen over bare `net/http` because `chi`'s route groups, sub-routers, and middleware ecosystem (request ID, structured logging, `Recoverer`, rate limiting) save real time without sacrificing the stdlib compatibility we would get from Go 1.22+'s improved `ServeMux` (method matching, `{wildcard}` path params). |
| Data access | `sqlc` (`github.com/sqlc-dev/sqlc`, latest v1.31.1, Apr 2026), generating against the `pgx/v5` driver (`sql_package: "pgx/v5"` in `sqlc.yaml`) | We write plain SQL for the repo-registry `schema.sql`/`query.sql`; `sqlc generate` produces fully type-safe Go structs and query methods (via a generated `Queries` struct wrapping a `DBTX` interface) with zero runtime reflection and zero ORM "magic". This keeps the data layer auditable (you can read the exact SQL that runs) while still getting compile-time safety on every query, the right tradeoff for a small, long-lived internal service. |
| Database | PostgreSQL, accessed via `jackc/pgx/v5` and `pgxpool` for connection pooling | Free, robust, easy to run locally or in a container; `sqlc` has first-class Postgres support including schema-aware type inference; `pgxpool.Pool` satisfies the generated `DBTX` interface directly, so `db.New(pool)` is all the wiring needed in `main.go`. |
| CLI concurrency | Go standard library (`sync`, `context`, goroutines plus a bounded worker pool or semaphore channel) | No extra dependency needed for the actual concurrency primitives; this is exactly the kind of I/O-bound fan-out Go's goroutines were built for. |

**Sources consulted:** `go-chi/chi` GitHub releases and changelog
(v5.3.0 through v5.3.2), `pkg.go.dev/github.com/go-chi/chi/v5`;
`gin-gonic/gin` GitHub repo, official Gin docs, and the Gin 1.12.0
release announcement; `sqlc-dev/sqlc` GitHub README/releases and
`docs.sqlc.dev` config/tutorial pages; the Go blog's "Routing
Enhancements for Go 1.22" post and the `golang/go` tracking issue and
discussion for enhanced `ServeMux` routing.

## Project structure

To keep this a one-sprint-sized effort, `makeutility-api` uses a flat,
minimal-ceremony layout rather than a full domain-driven
`internal/{repository,rest,service}` split (the pattern used by larger,
"meant to last for years" enterprise examples like
`MarioCarrion/todo-api-microservice-example`). With one resource
(`repos`) and a handful of endpoints, that much layering would add
indirection without payoff this sprint. It is straightforward to
introduce later if the service grows more resources or domains.

```
makeutility/
  cmd/
    makeutility/        (CLI entrypoint: sync, status)
    makeutility-api/    (API service entrypoint)
  internal/
    api/                (chi routes, handlers, API-key middleware)
    db/                 (sqlc-generated code, schema.sql, query.sql)
    gitops/             (goroutine worker pool: clone/fetch/status logic)
  repos.yaml            (local fallback repo list for the CLI)
  sqlc.yaml
```

## Enterprise patterns researched: adopting now vs. later

I compared our plan against current (2026) enterprise REST conventions
(Stripe/GitHub/Google/Microsoft Azure Architecture Center guidance, plus
real Go examples like `RashadTanjim/golang-enterprise-microservice-system`
and `MarioCarrion/todo-api-microservice-example`). Given this is a
one-sprint internal tool for a few dozen engineers, we are deliberately
staying lean now and flagging the heavier patterns as explicit future
work rather than gold-plating the first version.

**Adopting this sprint:**

- Simple, single shared API-key auth middleware (not JWT/RBAC),
  appropriate for an internal, trusted-network tool.
- Plain JSON responses and standard HTTP status codes (200, 201, 400,
  404, 500), not yet the full `application/problem+json` (RFC 9457)
  error envelope enterprise APIs use.
- Path-based versioning reserved conceptually (`/repos` today, easy to
  prefix with `/v1` later) but not enforced with a formal deprecation
  policy yet.

**Explicitly deferred** (documented here so it is not forgotten, not
because it is wrong):

- RFC 9457 `application/problem+json` error envelopes instead of ad-hoc
  `{"error": "..."}` JSON.
- Cursor-based pagination (`?limit=20&cursor=...` returning `items`,
  `next_cursor`, and `has_more`). This is a non-issue at our current
  scale (dozens of repos), but the right pattern once the registry grows.
- An OpenAPI 3.1 contract with Spectral linting and oasdiff CI checks,
  so the CLI and any future consumers (dashboard, Slack bot) can
  generate clients instead of hand-coding HTTP calls.
- Rate limiting with `RateLimit-*`/`Retry-After` headers, JWT/RBAC, and
  audit logging: all reasonable next steps if this graduates from "one
  sprint utility" to a service other teams depend on.

## Scope for this sprint

**In scope:**

- `makeutility-api`: a `chi`-routed HTTP service with `GET /repos`,
  `POST /repos`, and `PATCH /repos/{id}` endpoints, backed by Postgres
  via `sqlc`-generated queries, with a basic API-key auth middleware
  (secured, not public).
- `makeutility` CLI: `sync` and `status` subcommands, goroutine
  worker-pool based, calling `makeutility-api` for the repo list.
- Local config fallback (`repos.yaml`) so the CLI still works if the
  API is unreachable.
- Clear, readable CLI output (table format) with color-coded status
  (clean, dirty, behind).
- Per-repo error handling so one broken repo does not halt the whole run.
- A README covering setup for both the API service and the CLI, plus a
  short usage demo.

**Stretch goals (time permitting):**

- `POST /repos/import` on the API that ingests from a Google Sheet, so
  the PM or leads can bulk-edit the registry without touching the API
  directly.
- `makeutility open <repo>` to jump straight into a repo's directory or
  open it in the configured editor.
- Structured request logging and simple rate limiting on the API via
  `chi`'s middleware package.

**Out of scope for this sprint:**

- Any write operations beyond `git clone`, `git fetch`, and `git pull`
  (no automated commits, merges, or pushes).
- Cross-platform packaging and distribution (Homebrew tap, Docker image
  registry, etc.). A local `go build`/`go run` is sufficient for the
  retrospective demo.
- Full auth/RBAC beyond a single shared API key. Fine for an internal,
  trusted-network tool this sprint.

## Success criteria

- `makeutility-api` serves the repo registry as a secured JSON API and
  is the single source of truth the CLI reads from.
- Running `makeutility sync` on a workspace missing 10-plus repos
  completes in a few seconds, visibly faster than sequential `git
  clone` calls.
- `makeutility status` gives an at-a-glance, accurate picture of every
  repo's state in one command, replacing the need to manually `cd` into
  each one.
- At least one teammate outside the author adopts the tool for real
  daily use before the retrospective, based on feedback from the
  demo/feedback form.

## Implementation status

- Confirmed: router (`chi` v5), data access (`sqlc` plus `pgx/v5`),
  database (PostgreSQL), project layout (flat `cmd/`/`internal/`
  structure), and auth approach (single API key).
- Environment: the local dev machine has Go 1.27.1. The `sqlc` CLI
  (v1.31.1) is installed and verified working.
- Dependencies installed one at a time and confirmed in `go.mod`:
  `github.com/go-chi/chi/v5@v5.3.2`, `github.com/jackc/pgx/v5@v5.11.0`
  (with `pgxpool`), `golang.org/x/sync/errgroup@v0.17.0` (bounded
  worker pool), and `go.yaml.in/yaml/v3@v3.0.5` (the actively
  maintained fork, since the original `gopkg.in/yaml.v3`/`go-yaml/yaml`
  repo was archived in 2025).
- Scaffolded and building cleanly (`go build ./...`, `go vet ./...`,
  and `go mod tidy` all pass with no errors):
  - `sqlc.yaml`, `internal/db/schema.sql`, `internal/db/query.sql`, and
    `sqlc`-generated `db.go`/`models.go`/`query.sql.go` for the `repos`
    table (`ListRepos`, `GetRepo`, `GetRepoByName`, `CreateRepo`,
    `UpdateRepo`, `DeleteRepo`).
  - `internal/api`: `chi` router (`Server.NewServer`), API-key
    middleware, and handlers for `GET /repos`, `POST /repos`, and
    `PATCH /repos/{id}`.
  - `cmd/makeutility-api`: service entrypoint wiring `pgxpool` to the
    `chi` handler, with graceful shutdown on `SIGINT`/`SIGTERM`.
  - `internal/gitops`: goroutine worker pool (via
    `errgroup.SetLimit`) for concurrent `Sync` (clone/fetch/pull) and
    `Status` (branch, ahead/behind, dirty check), plus the
    `repos.yaml` loader and the `makeutility-api` HTTP client.
  - `cmd/makeutility`: CLI entrypoint with `sync` and `status`
    subcommands, table output via `text/tabwriter`.
  - `repos.yaml`: example local fallback repo list.
- Linting: `golangci-lint` v2.13.2 is installed, configured in
  `.golangci.yml` with the standard linter set (`errcheck`, `govet`,
  `ineffassign`, `staticcheck`, `unused`) plus `revive` rules chosen
  to match Google's Go Style Guide
  (`google.github.io/styleguide/go/decisions`): required doc comments
  on exported names and packages, lowercase/no-punctuation error
  strings, consistent receiver naming, early return over nested error
  handling, `context.Context` as the first parameter, and MixedCaps
  naming. `golangci-lint run ./...` currently reports 0 issues; fixes
  made along the way included removing the deprecated, spoofable
  `middleware.RealIP` (flagged by `staticcheck`), checking previously
  ignored `Fprintf`/`Flush` errors (`errcheck`), and adding the
  missing `internal/api` package doc comment.
- Docker: `Dockerfile` (multi-stage `golang` build, `alpine` runtime,
  non-root user) and `docker-compose.yml` (Postgres plus
  `makeutility-api`) are added. Every base image is pinned to both a
  tag and its OCI index digest (`postgres:18.6@sha256:...`,
  `golang:1.27@sha256:...`, `alpine:3.22@sha256:...`), per current
  (2026) container supply-chain guidance: tags are mutable pointers,
  digests are content-addressed and immutable. Digests were obtained
  with `docker buildx imagetools inspect` and verified to be the
  multi-arch index digest, not a per-architecture manifest digest, so
  the same Dockerfile resolves correctly on both amd64 and arm64.
- Validated end to end: `docker compose up` brings up Postgres (with
  `internal/db/schema.sql` auto-applied via
  `docker-entrypoint-initdb.d`) and `makeutility-api` together.
  Verified live: `GET /healthz`, `POST /repos` (create), `GET /repos`
  (list), `PATCH /repos/{id}` (update), and that requests without
  `X-API-Key` are correctly rejected with 401. One real-world fix
  needed along the way: Postgres 18+ changed its expected volume
  mount point from `/var/lib/postgresql/data` to `/var/lib/postgresql`
  (pre-18 images used the former); the compose file mounts the
  correct path for 18.x.
- Not yet started: README and the stretch goals (`POST /repos/import`,
  `makeutility open`, rate limiting).
- Next step: write the README (setup instructions for both the
  Docker-based and local-Go-toolchain workflows), then run the CLI's
  `sync`/`status` subcommands against the containerized API.

## Demo plan for retrospective

1. Show `makeutility-api` running locally, and hit `GET /repos` to show
   the current registry as JSON.
2. On a fresh, empty workspace directory, run `makeutility sync`; watch
   repos clone concurrently with live progress output.
3. Make a change in one repo, then run `makeutility status`; show it
   correctly flags the dirty repo among the clean ones.
4. `POST` a new repo to the API, re-run `makeutility sync`, and show it
   picks up the new repo automatically, demonstrating the
   registry-as-API pattern end to end.
