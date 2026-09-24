package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

// Sync clones any repo in specs that is missing from workspaceDir, and
// fetches+pulls every repo that already exists there. Work is fanned out
// across a bounded pool of goroutines (via errgroup.SetLimit) so a large
// repo list finishes in roughly the time of the slowest single repo,
// rather than the sum of all of them.
func Sync(ctx context.Context, workspaceDir string, specs []RepoSpec, maxConcurrent int) []RepoState {
	states := make([]RepoState, len(specs))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i, spec := range specs {
		i, spec := i, spec
		g.Go(func() error {
			states[i] = syncOne(ctx, workspaceDir, spec)
			return nil // per-repo errors are captured on RepoState, not propagated
		})
	}

	_ = g.Wait()
	return states
}

func syncOne(ctx context.Context, workspaceDir string, spec RepoSpec) RepoState {
	state := RepoState{Name: spec.Name}
	repoPath := filepath.Join(workspaceDir, spec.Name)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		if err := runGit(ctx, workspaceDir, "clone", spec.URL, spec.Name); err != nil {
			state.Err = err
			return state
		}
		state.Cloned = true
		return state
	}

	if err := runGit(ctx, repoPath, "fetch", "--all", "--prune"); err != nil {
		state.Err = err
		return state
	}
	if err := runGit(ctx, repoPath, "pull", "--ff-only"); err != nil {
		state.Err = err
		return state
	}
	state.Fetched = true
	return state
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &gitError{args: args, output: string(out), cause: err}
	}
	return nil
}

type gitError struct {
	args   []string
	output string
	cause  error
}

func (e *gitError) Error() string {
	return "git " + joinArgs(e.args) + ": " + e.cause.Error() + "\n" + e.output
}

func (e *gitError) Unwrap() error { return e.cause }

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
