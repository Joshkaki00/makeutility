package gitops

import (
	"os"

	"go.yaml.in/yaml/v3"
)

// LoadReposYAML reads the local fallback repo list used when
// makeutility-api is unreachable (see proposal.md, "in scope" section).
func LoadReposYAML(path string) ([]RepoSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg struct {
		Repos []RepoSpec `yaml:"repos"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg.Repos, nil
}
