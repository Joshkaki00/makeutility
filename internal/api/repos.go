package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/joshuakaki/makeutility/internal/db"
)

// repo is the JSON wire representation of a repos row. We map away from the
// sqlc-generated db.Repo (which carries pgtype.Timestamptz) so API consumers
// only ever see plain JSON types.
type repo struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Owner     string    `json:"owner"`
	Tags      []string  `json:"tags"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toRepo(r db.Repo) repo {
	return repo{
		ID:        r.ID,
		Name:      r.Name,
		URL:       r.Url,
		Owner:     r.Owner,
		Tags:      r.Tags,
		Active:    r.Active,
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}

func toRepos(rows []db.Repo) []repo {
	out := make([]repo, 0, len(rows))
	for _, r := range rows {
		out = append(out, toRepo(r))
	}
	return out
}

// listRepos handles GET /repos.
func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	rows, err := s.queries.ListRepos(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repos: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRepos(rows))
}

type createRepoRequest struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Owner  string   `json:"owner"`
	Tags   []string `json:"tags"`
	Active *bool    `json:"active"`
}

// createRepo handles POST /repos.
func (s *Server) createRepo(w http.ResponseWriter, r *http.Request) {
	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.Name == "" || req.URL == "" || req.Owner == "" {
		writeError(w, http.StatusBadRequest, "name, url, and owner are required")
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	created, err := s.queries.CreateRepo(r.Context(), db.CreateRepoParams{
		Name:   req.Name,
		Url:    req.URL,
		Owner:  req.Owner,
		Tags:   req.Tags,
		Active: active,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create repo: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toRepo(created))
}

type updateRepoRequest struct {
	URL    *string  `json:"url"`
	Owner  *string  `json:"owner"`
	Tags   []string `json:"tags"`
	Active *bool    `json:"active"`
}

// updateRepo handles PATCH /repos/{id}.
func (s *Server) updateRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid repo id")
		return
	}

	var req updateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	params := db.UpdateRepoParams{ID: id}
	if req.URL != nil {
		params.Url = req.URL
	}
	if req.Owner != nil {
		params.Owner = req.Owner
	}
	if req.Tags != nil {
		params.Tags = req.Tags
	}
	if req.Active != nil {
		params.Active = req.Active
	}

	updated, err := s.queries.UpdateRepo(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusNotFound, "repo not found or update failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toRepo(updated))
}
