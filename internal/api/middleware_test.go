package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiKeyAuth(t *testing.T) {
	t.Parallel()

	const expectedKey = "correct-key"

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiKeyAuth(expectedKey)(next)

	tests := []struct {
		name       string
		headerKey  string
		sendHeader bool
		wantStatus int
	}{
		{name: "correct key allowed", headerKey: expectedKey, sendHeader: true, wantStatus: http.StatusOK},
		{name: "missing header rejected", sendHeader: false, wantStatus: http.StatusUnauthorized},
		{name: "empty header rejected", headerKey: "", sendHeader: true, wantStatus: http.StatusUnauthorized},
		{name: "wrong key rejected", headerKey: "wrong-key", sendHeader: true, wantStatus: http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/repos", nil)
			if tc.sendHeader {
				req.Header.Set("X-API-Key", tc.headerKey)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusUnauthorized {
				if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}
			}
		})
	}
}
