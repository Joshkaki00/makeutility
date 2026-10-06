package gitops

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestFetchReposFromAPI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		wantSpecs  []RepoSpec
		wantErr    bool
		wantStatus int // if non-zero, error text must mention this status code
	}{
		{
			name:   "ok with two repos",
			status: http.StatusOK,
			body:   `[{"name":"a","url":"https://example.com/a.git"},{"name":"b","url":"https://example.com/b.git"}]`,
			wantSpecs: []RepoSpec{
				{Name: "a", URL: "https://example.com/a.git"},
				{Name: "b", URL: "https://example.com/b.git"},
			},
		},
		{
			name:      "ok empty list",
			status:    http.StatusOK,
			body:      `[]`,
			wantSpecs: []RepoSpec{},
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			body:       `{"error":"nope"}`,
			wantErr:    true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "server error",
			status:     http.StatusInternalServerError,
			body:       `{"error":"db down"}`,
			wantErr:    true,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:    "malformed json",
			status:  http.StatusOK,
			body:    `{not-json`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var sawKey string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sawKey = r.Header.Get("X-API-Key")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			got, err := FetchReposFromAPI(t.Context(), srv.URL, "test-key")
			if sawKey != "test-key" {
				t.Errorf("X-API-Key = %q, want %q", sawKey, "test-key")
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("FetchReposFromAPI = %#v, nil; want error", got)
				}
				if tc.wantStatus != 0 && !strings.Contains(err.Error(), strconv.Itoa(tc.wantStatus)) {
					t.Errorf("error %q does not mention status %d", err, tc.wantStatus)
				}
				return
			}
			if err != nil {
				t.Fatalf("FetchReposFromAPI unexpected error: %v", err)
			}
			if len(got) != len(tc.wantSpecs) {
				t.Fatalf("got %d specs, want %d: %#v", len(got), len(tc.wantSpecs), got)
			}
			for i := range got {
				if got[i] != tc.wantSpecs[i] {
					t.Errorf("spec[%d] = %#v, want %#v", i, got[i], tc.wantSpecs[i])
				}
			}
		})
	}
}
