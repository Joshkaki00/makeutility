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
| 8 | +1.0 | 2 or more table-driven tests | |
| 9 | +0.5 | 1 or more benchmark tests | |
| 0 | +1.5 | All tests pass | |
| - | +0.0 | Academic dishonesty: code copied from another student | n/a |

## Notes on current status (as of this commit)

- **#1**: `go build ./...`, `go vet ./...`, and `golangci-lint run
  ./...` all pass with zero issues. `docker compose up` was run and
  verified end to end: `GET /healthz`, `POST /repos`, `GET /repos`,
  and `PATCH /repos/{id}` all work against a real PostgreSQL instance.
- **#2**: not yet submitted to goreportcard.com (requires a public
  repo URL). `golangci-lint`, which covers the same underlying
  checks (`gofmt`, `go vet`, `ineffassign`, `staticcheck`, plus
  `revive` in place of the retired `golint`), reports zero issues, so
  this is expected to score well once submitted, but is left
  unchecked until actually verified on the real site.
- **#3**: `chi`, `pgx/v5`, `sqlc`-generated code, and
  `go.yaml.in/yaml/v3` are all real external dependencies in active
  use, not stubs.
- **#4**: PostgreSQL via `makeutility-api`, plus `repos.yaml` as a
  local file-based fallback for the CLI.
- **#5, #6, #7**: see `README.md` (description, Docker/local install
  instructions, and `curl`/CLI usage examples).
- **#8, #9, #0**: no test files exist yet in the repository. This is
  the main outstanding gap.
