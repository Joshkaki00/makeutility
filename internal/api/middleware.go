package api

import "net/http"

// apiKeyAuth is a minimal shared-secret auth middleware, appropriate for an
// internal tool on a trusted network. See proposal.md for the deliberate
// decision to defer JWT/RBAC to a later sprint.
func apiKeyAuth(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" || key != expectedKey {
				writeError(w, http.StatusUnauthorized, "missing or invalid X-API-Key header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
