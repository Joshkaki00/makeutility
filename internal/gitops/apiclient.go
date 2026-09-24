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
// active repo list as RepoSpecs.
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("makeutility-api returned status %d", resp.StatusCode)
	}

	var repos []apiRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}

	specs := make([]RepoSpec, 0, len(repos))
	for _, r := range repos {
		specs = append(specs, RepoSpec{Name: r.Name, URL: r.URL})
	}
	return specs, nil
}
