package gitops

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReposYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string // "" means: don't create the file at all
		want    []RepoSpec
		wantErr bool
	}{
		{
			name: "valid multi-repo list",
			content: `repos:
  - name: foo
    url: https://example.com/foo.git
  - name: bar
    url: https://example.com/bar.git
`,
			want: []RepoSpec{
				{Name: "foo", URL: "https://example.com/foo.git"},
				{Name: "bar", URL: "https://example.com/bar.git"},
			},
		},
		{
			name:    "empty repos key",
			content: "repos: []\n",
			want:    []RepoSpec{},
		},
		{
			name:    "missing repos key entirely",
			content: "other: value\n",
			want:    nil,
		},
		{
			name:    "malformed yaml",
			content: "repos: [not: valid: yaml",
			wantErr: true,
		},
		{
			name:    "file does not exist",
			content: "", // sentinel: skip writing the file
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "repos.yaml")

			if tc.name != "file does not exist" {
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatalf("failed to write fixture: %v", err)
				}
			} else {
				path = filepath.Join(dir, "does-not-exist.yaml")
			}

			got, err := LoadReposYAML(path)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("LoadReposYAML(%q) = %v, nil; want error", path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadReposYAML(%q) unexpected error: %v", path, err)
			}

			if len(got) != len(tc.want) {
				t.Fatalf("LoadReposYAML(%q) = %#v, want %#v", path, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("repo[%d] = %#v, want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestLoadReposYAML_ErrorIsWrapped(t *testing.T) {
	t.Parallel()

	_, err := LoadReposYAML(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected a wrapped os.ErrNotExist, got: %v", err)
	}
}
