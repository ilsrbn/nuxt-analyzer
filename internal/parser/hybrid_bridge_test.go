package parser

import (
	"reflect"
	"testing"
)

func TestHybridBridgeRoutesVueToNodeAndTsToGo(t *testing.T) {
	var nodeFiles []string
	h := &HybridBridge{
		node: &Bridge{
			parserPath: "parser.bundle.js",
			runCmd: func(name string, args []string, input []byte) ([]byte, error) {
				nodeFiles = []string{"/project/components/AppButton.vue"}
				return []byte(`{"results":[{"path":"/project/components/AppButton.vue","type":"component","imports":[],"templateRefs":["OtherButton"],"dynamicComponents":[],"usedAutoImports":[],"providedInjections":[],"usedInjections":[],"error":null}]}`), nil
			},
		},
		ts: NewTypeScriptParser(),
		readFile: func(path string) ([]byte, error) {
			return []byte(`import thing from './thing'`), nil
		},
	}

	got, err := h.Parse([]string{"/project/components/AppButton.vue", "/project/composables/useThing.ts"}, nil)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !reflect.DeepEqual(nodeFiles, []string{"/project/components/AppButton.vue"}) {
		t.Fatalf("nodeFiles = %#v, want only vue file", nodeFiles)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(got))
	}
	if got[0].Path != "/project/components/AppButton.vue" {
		t.Fatalf("first result path = %q, want vue path", got[0].Path)
	}
	if got[1].Path != "/project/composables/useThing.ts" {
		t.Fatalf("second result path = %q, want ts path", got[1].Path)
	}
	if !reflect.DeepEqual(got[1].Imports, []string{"./thing"}) {
		t.Fatalf("ts imports = %#v, want %#v", got[1].Imports, []string{"./thing"})
	}
}

func TestHybridBridgeRoutesUppercaseExtensions(t *testing.T) {
	var nodeFiles []string
	h := &HybridBridge{
		node: &Bridge{
			parserPath: "parser.bundle.js",
			runCmd: func(name string, args []string, input []byte) ([]byte, error) {
				nodeFiles = []string{"/project/components/AppButton.VUE"}
				return []byte(`{"results":[{"path":"/project/components/AppButton.VUE","type":"component","imports":[],"templateRefs":[],"dynamicComponents":[],"usedAutoImports":[],"providedInjections":[],"usedInjections":[],"error":null}]}`), nil
			},
		},
		ts: NewTypeScriptParser(),
		readFile: func(path string) ([]byte, error) {
			return []byte(`import thing from './thing'`), nil
		},
	}

	got, err := h.Parse([]string{"/project/components/AppButton.VUE", "/project/composables/useThing.TS"}, nil)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !reflect.DeepEqual(nodeFiles, []string{"/project/components/AppButton.VUE"}) {
		t.Fatalf("nodeFiles = %#v, want uppercase vue file", nodeFiles)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(got))
	}
	if got[1].Path != "/project/composables/useThing.TS" {
		t.Fatalf("second result path = %q, want uppercase ts path", got[1].Path)
	}
	if !reflect.DeepEqual(got[1].Imports, []string{"./thing"}) {
		t.Fatalf("ts imports = %#v, want %#v", got[1].Imports, []string{"./thing"})
	}
}
