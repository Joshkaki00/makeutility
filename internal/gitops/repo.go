// Package gitops implements the concurrent, goroutine-based clone/fetch/
// status logic behind the makeutility CLI's sync and status subcommands.
package gitops

import (
	"fmt"
	"path/filepath"
)

// RepoSpec describes a repo the CLI should manage locally. It is the
// common shape whether the list comes from makeutility-api or from the
// local repos.yaml fallback.
type RepoSpec struct {
	// Name is the local directory name under the workspace (also the
	// unique key in makeutility-api).
	Name string `yaml:"name" json:"name"`
	// URL is the git remote to clone or fetch from.
	URL string `yaml:"url" json:"url"`
}

// RepoState is the result of inspecting or syncing one local repo.
// Per-repo failures are reported on Err rather than aborting the batch.
type RepoState struct {
	// Name matches the RepoSpec that produced this result.
	Name string
	// Branch is the currently checked-out branch, if known.
	Branch string
	// Ahead is how many local commits are not on the upstream.
	Ahead int
	// Behind is how many upstream commits are not in the local branch.
	Behind int
	// Dirty is true when the working tree has uncommitted changes.
	Dirty bool
	// Cloned is true when Sync created this repo in the workspace.
	Cloned bool
	// Fetched is true when Sync refreshed an existing local clone.
	Fetched bool
	// Err is set when clone, fetch, or status inspection failed.
	Err error
}

// safeRepoPath joins workspaceDir and name, rejecting names that would
// escape the workspace via "..", absolute paths, or path separators.
func safeRepoPath(workspaceDir, name string) (string, error) {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		return "", fmt.Errorf("invalid repo name %q", name)
	}
	return filepath.Join(workspaceDir, name), nil
}

// concurrencyLimit returns a positive worker limit. errgroup.SetLimit(0)
// never schedules work, so a non-positive value would deadlock Sync/Status.
func concurrencyLimit(maxConcurrent int) int {
	if maxConcurrent < 1 {
		return 1
	}
	return maxConcurrent
}
