package nuxt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAutoImportMap_ParsesLocalImports(t *testing.T) {
	dir := t.TempDir()
	nuxtDir := filepath.Join(dir, ".nuxt")
	if err := os.MkdirAll(nuxtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `export { useScriptNpm } from '#app/composables/script-stubs';
export { ref, computed } from 'vue';
export { useRoute, useRouter } from '#app/composables/router';
export { useFragments } from '../app/composables/useFragments';
export { useSeoHead, SeoOptions } from '../app/composables/useSeoHead';
export { useSystems } from '../app/composables/useSystems';
export { useNuxtDevTools } from '../node_modules/.pnpm/@nuxt+devtools/dist/runtime/use';
`
	if err := os.WriteFile(filepath.Join(nuxtDir, "imports.d.ts"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadAutoImportMap(dir)
	if err != nil {
		t.Fatalf("LoadAutoImportMap() error = %v", err)
	}

	wantPresent := map[string]string{
		"useFragments": "app/composables/useFragments",
		"useSeoHead":   "app/composables/useSeoHead",
		"SeoOptions":   "app/composables/useSeoHead",
		"useSystems":   "app/composables/useSystems",
	}
	for name, wantPath := range wantPresent {
		if got, ok := m[name]; !ok || got != wantPath {
			t.Errorf("m[%q] = %q (ok=%v), want %q", name, got, ok, wantPath)
		}
	}

	wantAbsent := []string{"ref", "computed", "useRoute", "useRouter", "useNuxtDevTools", "useScriptNpm"}
	for _, name := range wantAbsent {
		if _, ok := m[name]; ok {
			t.Errorf("m[%q] should be absent", name)
		}
	}
}

func TestLoadAutoImportMap_MissingFileReturnsEmpty(t *testing.T) {
	m, err := LoadAutoImportMap(t.TempDir())
	if err != nil {
		t.Fatalf("LoadAutoImportMap() error = %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %v", m)
	}
}

func TestLoadAutoImportMap_ReExportAs(t *testing.T) {
	dir := t.TempDir()
	nuxtDir := filepath.Join(dir, ".nuxt")
	if err := os.MkdirAll(nuxtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `export { default as useMyHelper } from '../app/composables/myHelper';` + "\n"
	if err := os.WriteFile(filepath.Join(nuxtDir, "imports.d.ts"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadAutoImportMap(dir)
	if err != nil {
		t.Fatalf("LoadAutoImportMap() error = %v", err)
	}
	if got, ok := m["useMyHelper"]; !ok || got != "app/composables/myHelper" {
		t.Errorf("m[useMyHelper] = %q (ok=%v), want app/composables/myHelper", got, ok)
	}
	if _, ok := m["default"]; ok {
		t.Error("m[default] should be absent")
	}
}
