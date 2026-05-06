package git

import (
	"errors"
	"strings"
	"testing"
)

func TestDiffer_ChangedFiles_ParsesOutput(t *testing.T) {
	d := &Differ{
		ProjectRoot: "/repo",
		exec: func(dir, name string, args ...string) ([]byte, error) {
			if dir != "/repo" {
				t.Fatalf("dir = %q, want /repo", dir)
			}
			if name != "git" {
				t.Fatalf("name = %q, want git", name)
			}
			if got := strings.Join(args, " "); got != "diff --name-only origin/main..HEAD" {
				t.Fatalf("args = %q", got)
			}
			return []byte("\ncomponents/Button.vue\r\npages/index.vue\n\n"), nil
		},
	}

	got, err := d.ChangedFiles("origin/main", "HEAD")
	if err != nil {
		t.Fatalf("ChangedFiles() error = %v", err)
	}

	want := []string{"components/Button.vue", "pages/index.vue"}
	if len(got) != len(want) {
		t.Fatalf("ChangedFiles() len = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ChangedFiles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiffer_ChangedFiles_FiltersGeneratedFiles(t *testing.T) {
	d := &Differ{
		ProjectRoot: "/repo",
		exec: func(dir, name string, args ...string) ([]byte, error) {
			return []byte(strings.Join([]string{
				"components/Button.vue",
				"node_modules/pkg/index.ts",
				".nuxt/types.d.ts",
				".output/server/index.mjs",
				"dist/app.js",
				"types/generated.d.ts",
				"",
			}, "\n")), nil
		},
	}

	got, err := d.ChangedFiles("base", "head")
	if err != nil {
		t.Fatalf("ChangedFiles() error = %v", err)
	}

	want := []string{"components/Button.vue"}
	if len(got) != len(want) {
		t.Fatalf("ChangedFiles() len = %d, want %d; got %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ChangedFiles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDiffer_ChangedFiles_PropagatesExecutionError(t *testing.T) {
	d := &Differ{
		ProjectRoot: "/repo",
		exec: func(dir, name string, args ...string) ([]byte, error) {
			return nil, errors.New("git not found")
		},
	}

	_, err := d.ChangedFiles("base", "head")
	if err == nil {
		t.Fatal("ChangedFiles() error = nil, want non-nil")
	}
	if got := err.Error(); !strings.Contains(got, "git not found") {
		t.Fatalf("error = %q, want wrapped execution error", got)
	}
}

func TestIsGeneratedPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"components/Button.vue", false},
		{"node_modules/pkg/index.ts", true},
		{".nuxt/types.d.ts", true},
		{".output/server/index.mjs", true},
		{"dist/app.js", true},
		{"types/generated.d.ts", true},
		{"", true},
		{"src/.nuxt/types.ts", false},
		{"src/dist/app.js", false},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isGeneratedPath(tc.path); got != tc.want {
				t.Fatalf("isGeneratedPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
