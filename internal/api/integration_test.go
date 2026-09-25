//go:build integration

// Package api integration tests run against a real, disposable
// PostgreSQL container (testcontainers-go), not a mock. Mocks can
// verify handler wiring, but only a real database can confirm the
// UNIQUE constraint, the sqlc query text, and pgx's type mapping
// actually behave the way the handlers assume. Unit tests
// (middleware_test.go, repos_test.go) stay mock-free and fast; this
// file is the slower, DB-backed layer above them.
//
// Run with: go test -tags=integration ./internal/api/...
// Requires a reachable Docker daemon; skips itself otherwise.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/joshuakaki/makeutility/internal/db"
)

const testAPIKey = "integration-test-key"

// newTestServer starts a real, disposable Postgres container, applies
// schema.sql, and returns an httptest.Server backed by the actual
// handlers and the actual database driver.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema, err := os.ReadFile("../db/schema.sql")
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}

	container, err := postgres.Run(ctx,
		"postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722",
		postgres.WithDatabase("makeutility_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		tc.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("Docker not available, skipping integration test: %v", err)
		}
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	queries := db.New(pool)
	handler := NewServer(queries, testAPIKey)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func isDockerUnavailable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Cannot connect to the Docker daemon") ||
		strings.Contains(msg, "docker.sock") ||
		strings.Contains(msg, "Is the docker daemon running")
}

func doJSON(t *testing.T, method, url, apiKey string, body any) *http.Response {
	t.Helper()
	var reqBody *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = strings.NewReader(string(b))
	} else {
		reqBody = strings.NewReader("")
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func TestIntegration_CreateAndListRepos(t *testing.T) {
	srv := newTestServer(t)

	tests := []struct {
		name       string
		body       map[string]any
		wantStatus int
	}{
		{
			name:       "valid repo is created",
			body:       map[string]any{"name": "makeutility", "url": "https://example.com/makeutility.git", "owner": "joshuakaki"},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing owner is rejected",
			body:       map[string]any{"name": "bad-repo", "url": "https://example.com/bad.git"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate name is rejected by the real UNIQUE constraint",
			body:       map[string]any{"name": "makeutility", "url": "https://example.com/other.git", "owner": "someone-else"},
			wantStatus: http.StatusInternalServerError,
		},
	}

	// Cases run in order because the duplicate-name case depends on
	// the first case's row already existing.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, http.MethodPost, srv.URL+"/repos/", testAPIKey, tc.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}

	resp := doJSON(t, http.MethodGet, srv.URL+"/repos/", testAPIKey, nil)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /repos/ status = %d, want 200", resp.StatusCode)
	}
	var got []repo
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(repos) = %d, want 1 (the duplicate insert must not have succeeded)", len(got))
	}
	if got[0].Name != "makeutility" {
		t.Errorf("repo name = %q, want %q", got[0].Name, "makeutility")
	}
}

func TestIntegration_UpdateRepo(t *testing.T) {
	srv := newTestServer(t)

	createResp := doJSON(t, http.MethodPost, srv.URL+"/repos/", testAPIKey, map[string]any{
		"name": "to-update", "url": "https://example.com/orig.git", "owner": "owner-a",
	})
	var created repo
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	_ = createResp.Body.Close()

	tests := []struct {
		name       string
		id         string
		body       map[string]any
		wantStatus int
	}{
		{
			name:       "partial update changes only the given field",
			id:         strconv.FormatInt(created.ID, 10),
			body:       map[string]any{"url": "https://example.com/new.git"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown id returns not found",
			id:         "999999",
			body:       map[string]any{"url": "https://example.com/whatever.git"},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, http.MethodPatch, srv.URL+"/repos/"+tc.id, testAPIKey, tc.body)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				var updated repo
				if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
					t.Fatalf("decode update response: %v", err)
				}
				if updated.URL != "https://example.com/new.git" {
					t.Errorf("updated URL = %q, want %q", updated.URL, "https://example.com/new.git")
				}
				if updated.Owner != "owner-a" {
					t.Errorf("owner should be unchanged, got %q", updated.Owner)
				}
			}
		})
	}
}

func TestIntegration_AuthRequired(t *testing.T) {
	srv := newTestServer(t)

	resp := doJSON(t, http.MethodGet, srv.URL+"/repos/", "", nil)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status without X-API-Key = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
