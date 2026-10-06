package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshuakaki/makeutility/internal/db"
)

// handlerTestServer builds NewServer with a nil DBTX. Only paths that
// never touch the database (healthz, decode/validation failures) are safe.
func handlerTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewServer(db.New(nil), "unit-test-key"))
	t.Cleanup(srv.Close)
	return srv
}

func TestHealthzUnauthenticated(t *testing.T) {
	t.Parallel()
	srv := handlerTestServer(t)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestCreateRepoValidation(t *testing.T) {
	t.Parallel()
	srv := handlerTestServer(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantSubstr string
	}{
		{
			name:       "malformed json",
			body:       `{not json`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "invalid JSON",
		},
		{
			name:       "empty object missing required fields",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "name, url, and owner are required",
		},
		{
			name:       "missing url",
			body:       `{"name":"x","owner":"y"}`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "name, url, and owner are required",
		},
		{
			name:       "whitespace-only name still empty after decode? no trim — accepted shape but empty string",
			body:       `{"name":"","url":"https://example.com/x.git","owner":"y"}`,
			wantStatus: http.StatusBadRequest,
			wantSubstr: "name, url, and owner are required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/repos/", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "unit-test-key")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			var er errorResponse
			_ = json.NewDecoder(resp.Body).Decode(&er)
			if !strings.Contains(er.Error, tc.wantSubstr) {
				t.Errorf("error = %q, want substring %q", er.Error, tc.wantSubstr)
			}
		})
	}
}

func TestUpdateRepoInvalidID(t *testing.T) {
	t.Parallel()
	srv := handlerTestServer(t)

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/repos/not-an-id", strings.NewReader(`{"url":"https://example.com/x.git"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "unit-test-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
