package api

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/joshuakaki/makeutility/internal/db"
)

func TestToRepo(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name  string
		input db.Repo
		want  repo
	}{
		{
			name: "typical repo with tags",
			input: db.Repo{
				ID:        1,
				Name:      "makeutility",
				Url:       "https://example.com/makeutility.git",
				Owner:     "joshuakaki",
				Tags:      []string{"go", "cli"},
				Active:    true,
				CreatedAt: pgtype.Timestamptz{Time: fixedTime, Valid: true},
				UpdatedAt: pgtype.Timestamptz{Time: fixedTime, Valid: true},
			},
			want: repo{
				ID:        1,
				Name:      "makeutility",
				URL:       "https://example.com/makeutility.git",
				Owner:     "joshuakaki",
				Tags:      []string{"go", "cli"},
				Active:    true,
				CreatedAt: fixedTime,
				UpdatedAt: fixedTime,
			},
		},
		{
			// Boundary: nil tags (empty TEXT[] column) must survive the
			// conversion as nil, not panic or become a non-nil empty slice
			// that changes the JSON shape.
			name: "nil tags and inactive repo",
			input: db.Repo{
				ID:     2,
				Name:   "archived",
				Url:    "https://example.com/archived.git",
				Owner:  "someone",
				Tags:   nil,
				Active: false,
			},
			want: repo{
				ID:     2,
				Name:   "archived",
				URL:    "https://example.com/archived.git",
				Owner:  "someone",
				Tags:   nil,
				Active: false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := toRepo(tc.input)
			if got.ID != tc.want.ID || got.Name != tc.want.Name || got.URL != tc.want.URL ||
				got.Owner != tc.want.Owner || got.Active != tc.want.Active ||
				!got.CreatedAt.Equal(tc.want.CreatedAt) || !got.UpdatedAt.Equal(tc.want.UpdatedAt) {
				t.Errorf("toRepo() = %+v, want %+v", got, tc.want)
			}
			if len(got.Tags) != len(tc.want.Tags) {
				t.Errorf("toRepo().Tags = %v, want %v", got.Tags, tc.want.Tags)
			}
		})
	}
}

func TestToRepos(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []db.Repo
		want  int
	}{
		{name: "empty slice stays empty, not nil", input: []db.Repo{}, want: 0},
		{name: "nil slice produces empty result", input: nil, want: 0},
		{
			name: "multiple rows preserve order",
			input: []db.Repo{
				{ID: 1, Name: "a"},
				{ID: 2, Name: "b"},
				{ID: 3, Name: "c"},
			},
			want: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := toRepos(tc.input)
			if got == nil {
				t.Fatal("toRepos() returned nil, want a non-nil (possibly empty) slice for JSON encoding")
			}
			if len(got) != tc.want {
				t.Fatalf("toRepos() len = %d, want %d", len(got), tc.want)
			}
			for i, r := range got {
				if r.ID != tc.input[i].ID || r.Name != tc.input[i].Name {
					t.Errorf("toRepos()[%d] = %+v, want ID=%d Name=%q", i, r, tc.input[i].ID, tc.input[i].Name)
				}
			}
		})
	}
}
