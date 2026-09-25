# Project rubric

Copied per the rubric's own instructions, with a checkmark column to
track completed requirements. Must score higher than 70% to pass.
Items marked with a star earn bonus points.

| # | Points | Requirement | Done |
|---|---|---|---|
| 1 | +9.0 | Builds, installs, and executes successfully | x |
| 2 | +10.0 | B or higher on Go Report Card | |
| 3 | +4.0 | Incorporates an external API or package | x |
| 4 | +2.5 | Persists data in a file or database | x |
| 5 | +0.5 | README contains description | x |
| 6 | +0.5 | README contains screenshot OR install instructions | x |
| 7 | +0.5 | README contains example of how to use the program | x |
| 8 | +1.0 | 2 or more table-driven tests | x |
| 9 | +0.5 | 1 or more benchmark tests | x |
| 0 | +1.5 | All tests pass | x |
| - | +0.0 | Academic dishonesty: code copied from another student | n/a |

## Notes on current status (as of this commit)

- **#1**: `go build ./...`, `go vet ./...`, and `golangci-lint run
  ./...` all pass with zero issues. `docker compose up` was run and
  verified end to end: `GET /healthz`, `POST /repos`, `GET /repos`,
  and `PATCH /repos/{id}` all work against a real PostgreSQL instance.
- **#2**: goreportcard.com has been **sunset** (confirmed by fetching
  both the homepage and this repo's report URL as of this commit --
  both show a shutdown notice, not a grading error). It can no longer
  be submitted to at all, through no fault of this repo. The site's
  own shutdown notice names `golangci-lint` as "the de-facto standard
  for Go code quality today... the spiritual successor to the
  metalinter that powered Go Report Card" -- and `golangci-lint run
  ./...` reports zero issues here. Left unchecked pending guidance
  from whoever owns the rubric on how to substitute for a
  now-nonexistent external service; self-hosting
  github.com/gojp/goreportcard (the still-open-source engine) is a
  fallback if an actual letter grade is required.
- **#3**: `chi`, `pgx/v5`, `sqlc`-generated code, and
  `go.yaml.in/yaml/v3` are all real external dependencies in active
  use, not stubs.
- **#4**: PostgreSQL via `makeutility-api`, plus `repos.yaml` as a
  local file-based fallback for the CLI.
- **#5, #6, #7**: see `README.md` (description, Docker/local install
  instructions, and `curl`/CLI usage examples).
- **#8**: table-driven tests in `internal/gitops/config_test.go`
  (`TestLoadReposYAML`, 5 cases), `internal/gitops/status_test.go`
  (`TestStatus`, against real temp git repos), `internal/api/
  middleware_test.go` (`TestApiKeyAuth`, 4 cases), and
  `internal/api/repos_test.go` (`TestToRepo`, `TestToRepos`).
- **#9**: `BenchmarkStatus` in `internal/gitops/status_test.go`,
  measuring the concurrent (`errgroup`-bounded) repo status-check path
  against 8 real local git repos.
- **#0**: `go test ./...` and `go test ./... -race` both pass with no
  failures.
