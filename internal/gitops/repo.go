// Package gitops implements the concurrent, goroutine-based clone/fetch/
// status logic behind the makeutility CLI's sync and status subcommands.
package gitops

// RepoSpec describes a repo the CLI should manage locally. It is the
// common shape whether the list comes from makeutility-api or from the
// local repos.yaml fallback.
type RepoSpec struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
}

// RepoState is the result of inspecting or syncing one local repo.
type RepoState struct {
	Name    string
	Branch  string
	Ahead   int
	Behind  int
	Dirty   bool
	Cloned  bool
	Fetched bool
	Err     error
}
