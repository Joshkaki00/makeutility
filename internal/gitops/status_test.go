package gitops

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

// requireGit skips the test if the git binary isn't on PATH -- Status
// shells out to real git rather than an interface, so these tests
// exercise the real boundary instead of mocking it away.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

// initRepo creates a real git repo at workspaceDir/name and returns its
// path. If dirty is true, an uncommitted change is left in the working
// tree.
func initRepo(t *testing.T, workspaceDir, name string, dirty bool) {
	t.Helper()
	repoPath := filepath.Join(workspaceDir, name)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := exec.Command("mkdir", "-p", repoPath).Run(); err != nil {
		t.Fatalf("mkdir %s: %v", repoPath, err)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")

	readme := filepath.Join(repoPath, "README.md")
	if err := exec.Command("sh", "-c", "echo hello > "+readme).Run(); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-q", "-m", "init")

	if dirty {
		if err := exec.Command("sh", "-c", "echo more >> "+readme).Run(); err != nil {
			t.Fatalf("dirty README: %v", err)
		}
	}
}

func TestStatus(t *testing.T) {
	requireGit(t)
	t.Parallel()

	workspace := t.TempDir()
	initRepo(t, workspace, "clean-repo", false)
	initRepo(t, workspace, "dirty-repo", true)

	specs := []RepoSpec{
		{Name: "clean-repo"},
		{Name: "dirty-repo"},
		{Name: "never-cloned"}, // boundary: not present on disk at all
	}

	states := Status(context.Background(), workspace, specs, 4)

	if len(states) != len(specs) {
		t.Fatalf("got %d states, want %d", len(states), len(specs))
	}

	byName := make(map[string]RepoState, len(states))
	for _, s := range states {
		byName[s.Name] = s
	}

	tests := []struct {
		name      string
		wantErr   bool
		wantDirty bool
	}{
		{name: "clean-repo", wantErr: false, wantDirty: false},
		{name: "dirty-repo", wantErr: false, wantDirty: true},
		{name: "never-cloned", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, ok := byName[tc.name]
			if !ok {
				t.Fatalf("no state returned for %q", tc.name)
			}
			if tc.wantErr {
				if state.Err == nil {
					t.Errorf("state.Err = nil, want an error for %q", tc.name)
				}
				return
			}
			if state.Err != nil {
				t.Fatalf("state.Err = %v, want nil for %q", state.Err, tc.name)
			}
			if state.Branch != "main" {
				t.Errorf("state.Branch = %q, want %q", state.Branch, "main")
			}
			if state.Dirty != tc.wantDirty {
				t.Errorf("state.Dirty = %v, want %v", state.Dirty, tc.wantDirty)
			}
		})
	}
}

// BenchmarkStatus measures the concurrent status-check path -- the part
// of the CLI the README specifically claims finishes "in seconds instead
// of minutes" via bounded goroutines. Repos are set up once outside the
// timed loop.
func BenchmarkStatus(b *testing.B) {
	if _, err := exec.LookPath("git"); err != nil {
		b.Skip("git not found on PATH")
	}

	workspace := b.TempDir()
	const repoCount = 8
	specs := make([]RepoSpec, repoCount)
	for i := range specs {
		name := "repo" + string(rune('a'+i))
		initRepoForBench(b, workspace, name)
		specs[i] = RepoSpec{Name: name}
	}

	ctx := context.Background()

	for b.Loop() {
		Status(ctx, workspace, specs, 4)
	}
}

func initRepoForBench(b *testing.B, workspaceDir, name string) {
	b.Helper()
	repoPath := filepath.Join(workspaceDir, name)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := exec.Command("mkdir", "-p", repoPath).Run(); err != nil {
		b.Fatalf("mkdir %s: %v", repoPath, err)
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "bench@example.com")
	run("config", "user.name", "bench")
	readme := filepath.Join(repoPath, "README.md")
	if err := exec.Command("sh", "-c", "echo hello > "+readme).Run(); err != nil {
		b.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-q", "-m", "init")
}
