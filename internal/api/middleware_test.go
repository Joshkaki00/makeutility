package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApiKeyAuth(t *testing.T) {
	t.Parallel()

	const expectedKey = "correct-key"

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiKeyAuth(expectedKey)(next)

	tests := map[string]struct {
		headerKey  string
		sendHeader bool
		wantStatus int
	}{
		"correct key allowed":     {headerKey: expectedKey, sendHeader: true, wantStatus: http.StatusOK},
		"missing header rejected": {sendHeader: false, wantStatus: http.StatusUnauthorized},
		"empty header rejected":   {headerKey: "", sendHeader: true, wantStatus: http.StatusUnauthorized},
		"wrong key rejected":      {headerKey: "wrong-key", sendHeader: true, wantStatus: http.StatusUnauthorized},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/repos", nil)
			if tc.sendHeader {
				req.Header.Set("X-API-Key", tc.headerKey)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code, "body: %s", rec.Body.String())
			if tc.wantStatus == http.StatusUnauthorized {
				assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			}
		})
	}
}
