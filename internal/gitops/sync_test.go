package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSync(t *testing.T) {
	requireGit(t)
	t.Parallel()

	workspace := t.TempDir()
	remoteRepo := newBareRemote(t)

	// Pre-existing local clone (same history as remote) for fetch/pull path.
	runGitCmd(t, workspace, "clone", "-q", remoteRepo, "already-local")

	specs := []RepoSpec{
		{Name: "fresh-clone", URL: remoteRepo},
		{Name: "already-local", URL: remoteRepo},
		{Name: "bad-remote", URL: filepath.Join(t.TempDir(), "does-not-exist.git")},
		{Name: "../escape-me", URL: remoteRepo},
	}

	states := Sync(context.Background(), workspace, specs, 4)
	if len(states) != len(specs) {
		t.Fatalf("got %d states, want %d", len(states), len(specs))
	}

	byName := map[string]RepoState{}
	for _, s := range states {
		byName[s.Name] = s
	}

	t.Run("clones missing repo", func(t *testing.T) {
		s := byName["fresh-clone"]
		if s.Err != nil {
			t.Fatalf("fresh-clone err = %v", s.Err)
		}
		if !s.Cloned {
			t.Errorf("fresh-clone.Cloned = false, want true")
		}
		if _, err := os.Stat(filepath.Join(workspace, "fresh-clone", ".git")); err != nil {
			t.Errorf("expected clone on disk: %v", err)
		}
	})

	t.Run("fetches existing repo", func(t *testing.T) {
		s := byName["already-local"]
		if s.Err != nil {
			t.Fatalf("already-local err = %v", s.Err)
		}
		if !s.Fetched {
			t.Errorf("already-local.Fetched = false, want true")
		}
		if s.Cloned {
			t.Errorf("already-local.Cloned = true, want false")
		}
	})

	t.Run("bad remote fails only that slot", func(t *testing.T) {
		s := byName["bad-remote"]
		if s.Err == nil {
			t.Fatal("bad-remote err = nil, want error")
		}
	})

	t.Run("path-escaping name is rejected", func(t *testing.T) {
		s := byName["../escape-me"]
		if s.Err == nil {
			t.Fatal("escape name err = nil, want invalid repo name")
		}
		if !strings.Contains(s.Err.Error(), "invalid repo name") {
			t.Errorf("err = %v, want invalid repo name", s.Err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(workspace), "escape-me")); err == nil {
			t.Error("path escape wrote outside workspace")
		}
	})
}

func TestSync_EmptySpecs(t *testing.T) {
	t.Parallel()
	states := Sync(context.Background(), t.TempDir(), nil, 4)
	if len(states) != 0 {
		t.Fatalf("got %d states, want 0", len(states))
	}
}

func TestSync_NonPositiveConcurrencyDoesNotHang(t *testing.T) {
	requireGit(t)
	t.Parallel()

	workspace := t.TempDir()
	remoteRepo := newBareRemote(t)

	done := make(chan []RepoState, 1)
	go func() {
		done <- Sync(context.Background(), workspace, []RepoSpec{{Name: "solo", URL: remoteRepo}}, 0)
	}()

	select {
	case states := <-done:
		if len(states) != 1 {
			t.Fatalf("got %d states, want 1", len(states))
		}
		if states[0].Err != nil {
			t.Fatalf("unexpected err: %v", states[0].Err)
		}
		if !states[0].Cloned {
			t.Error("expected clone with maxConcurrent=0 (limit coerced to 1)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Sync with maxConcurrent=0 hung (errgroup SetLimit(0) deadlock)")
	}
}

func TestSafeRepoPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		{name: "plain", repo: "svc", wantErr: false},
		{name: "dotdot", repo: "../x", wantErr: true},
		{name: "nested", repo: "a/b", wantErr: true},
		{name: "empty", repo: "", wantErr: true},
		{name: "dot", repo: ".", wantErr: true},
		{name: "parent", repo: "..", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := safeRepoPath("/tmp/ws", tc.repo)
			if tc.wantErr && err == nil {
				t.Fatal("want error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}
}

// newBareRemote creates a bare git remote with one commit on main.
func newBareRemote(t *testing.T) string {
	t.Helper()
	remoteDir := t.TempDir()
	remoteRepo := filepath.Join(remoteDir, "seed.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remoteRepo).CombinedOutput(); err != nil {
		t.Fatalf("init bare remote: %v\n%s", err, out)
	}

	seed := filepath.Join(t.TempDir(), "seed-work")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatalf("mkdir seed: %v", err)
	}
	runGitCmd(t, seed, "init", "-q", "-b", "main")
	runGitCmd(t, seed, "config", "user.email", "test@example.com")
	runGitCmd(t, seed, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("hi\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitCmd(t, seed, "add", "README.md")
	runGitCmd(t, seed, "commit", "-q", "-m", "init")
	runGitCmd(t, seed, "remote", "add", "origin", remoteRepo)
	runGitCmd(t, seed, "push", "-q", "origin", "main")
	return remoteRepo
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v (in %s): %v\n%s", args, dir, err, out)
	}
}
