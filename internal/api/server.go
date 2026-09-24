package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/joshuakaki/makeutility/internal/db"
)

// Server holds the dependencies shared by all HTTP handlers.
type Server struct {
	queries *db.Queries
}

// NewServer builds the chi-routed HTTP handler for makeutility-api.
// apiKey is the shared secret required in the X-API-Key header for every
// request; see proposal.md for why a single API key is enough for this
// sprint.
func NewServer(queries *db.Queries, apiKey string) http.Handler {
	s := &Server{queries: queries}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/repos", func(r chi.Router) {
		r.Use(apiKeyAuth(apiKey))
		r.Get("/", s.listRepos)
		r.Post("/", s.createRepo)
		r.Patch("/{id}", s.updateRepo)
	})

	return r
}
