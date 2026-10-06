package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type apiRepo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// FetchReposFromAPI calls GET /repos on makeutility-api and returns the
// active repo list as RepoSpecs. baseURL is the API origin without a
// trailing path (for example "http://localhost:8080"); apiKey is sent in
// the X-API-Key header. Non-200 responses and network/decode failures
// return an error.
func FetchReposFromAPI(ctx context.Context, baseURL, apiKey string) ([]RepoSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/repos/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("makeutility-api returned status %d", resp.StatusCode)
	}

	var repos []apiRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	specs := make([]RepoSpec, 0, len(repos))
	for _, r := range repos {
		specs = append(specs, RepoSpec(r))
	}
	return specs, nil
}
