package analyzer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestScan_InfersTypeAndSkipsUnknown(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, root, "components/Button.vue")
	mustWriteFile(t, root, "pages/index.vue")
	mustWriteFile(t, root, "composables/useAuth.ts")
	mustWriteFile(t, root, "other/ignore.ts")
	mustWriteFile(t, root, "docs/readme.md")

	got, err := Scan(root, true)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := []FileInfo{
		{AbsPath: filepath.Join(root, "components", "Button.vue"), RelPath: "components/Button.vue", Type: NodeTypeComponent},
		{AbsPath: filepath.Join(root, "composables", "useAuth.ts"), RelPath: "composables/useAuth.ts", Type: NodeTypeComposable},
		{AbsPath: filepath.Join(root, "pages", "index.vue"), RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	if len(got) != len(want) {
		t.Fatalf("Scan() len = %d, want %d; got %#v", len(got), len(want), got)
	}

	sort.Slice(got, func(i, j int) bool { return got[i].RelPath < got[j].RelPath })
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Scan()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestScan_ExcludesTestFilesWhenDisabled(t *testing.T) {
	root := t.TempDir()

	mustWriteFile(t, root, "components/Button.vue")
	mustWriteFile(t, root, "components/Button.test.vue")
	mustWriteFile(t, root, "pages/index.spec.ts")
	mustWriteFile(t, root, "pages/about.vue")

	got, err := Scan(root, false)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := []string{"components/Button.vue", "pages/about.vue"}

	if len(got) != len(want) {
		t.Fatalf("Scan() len = %d, want %d; got %#v", len(got), len(want), got)
	}

	sort.Slice(got, func(i, j int) bool { return got[i].RelPath < got[j].RelPath })
	for i, rel := range want {
		if got[i].RelPath != rel {
			t.Fatalf("Scan()[%d].RelPath = %q, want %q", i, got[i].RelPath, rel)
		}
		if got[i].AbsPath != filepath.Join(root, filepath.FromSlash(rel)) {
			t.Fatalf("Scan()[%d].AbsPath = %q, want %q", i, got[i].AbsPath, filepath.Join(root, filepath.FromSlash(rel)))
		}
	}
}

func mustWriteFile(t *testing.T, root, rel string) {
	t.Helper()

	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", full, err)
	}
	if err := os.WriteFile(full, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}
