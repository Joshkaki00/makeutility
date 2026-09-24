package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// Status concurrently inspects every repo in specs that exists locally
// under workspaceDir and reports its branch, ahead/behind counts, and
// dirty/clean state.
func Status(ctx context.Context, workspaceDir string, specs []RepoSpec, maxConcurrent int) []RepoState {
	states := make([]RepoState, len(specs))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i, spec := range specs {
		i, spec := i, spec
		g.Go(func() error {
			states[i] = statusOne(ctx, workspaceDir, spec)
			return nil
		})
	}

	_ = g.Wait()
	return states
}

func statusOne(ctx context.Context, workspaceDir string, spec RepoSpec) RepoState {
	state := RepoState{Name: spec.Name}
	repoPath := filepath.Join(workspaceDir, spec.Name)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		state.Err = errNotCloned
		return state
	}

	branch, err := gitOutput(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		state.Err = err
		return state
	}
	state.Branch = strings.TrimSpace(branch)

	counts, err := gitOutput(ctx, repoPath, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		fields := strings.Fields(strings.TrimSpace(counts))
		if len(fields) == 2 {
			state.Ahead, _ = strconv.Atoi(fields[0])
			state.Behind, _ = strconv.Atoi(fields[1])
		}
	}

	dirty, err := gitOutput(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		state.Err = err
		return state
	}
	state.Dirty = strings.TrimSpace(dirty) != ""

	return state
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

var errNotCloned = &notClonedError{}

type notClonedError struct{}

func (e *notClonedError) Error() string { return "repo not cloned locally" }
