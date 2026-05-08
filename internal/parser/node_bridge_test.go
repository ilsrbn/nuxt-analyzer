package parser

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestBridgeParseReturnsResults(t *testing.T) {
	b := &Bridge{
		parserPath: "parser.bundle.js",
		runCmd: func(name string, args []string, input []byte) ([]byte, error) {
			return []byte(`{"results":[{"path":"/a.vue","type":"component","imports":["./foo"],"templateRefs":["MyComp"],"dynamicComponents":["resolveComponent(name)"],"usedAutoImports":["useAuth"],"providedInjections":["api","*"],"usedInjections":["api"],"error":null}]}`), nil
		},
	}

	results, err := b.Parse([]string{"/a.vue"}, nil)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	got := results[0]
	if got.Path != "/a.vue" {
		t.Fatalf("Path = %q, want %q", got.Path, "/a.vue")
	}
	if got.Type != "component" {
		t.Fatalf("Type = %q, want %q", got.Type, "component")
	}
	if !reflect.DeepEqual(got.Imports, []string{"./foo"}) {
		t.Fatalf("Imports = %#v, want %#v", got.Imports, []string{"./foo"})
	}
	if !reflect.DeepEqual(got.TemplateRefs, []string{"MyComp"}) {
		t.Fatalf("TemplateRefs = %#v, want %#v", got.TemplateRefs, []string{"MyComp"})
	}
	if !reflect.DeepEqual(got.DynamicComponents, []string{"resolveComponent(name)"}) {
		t.Fatalf("DynamicComponents = %#v, want %#v", got.DynamicComponents, []string{"resolveComponent(name)"})
	}
	if !reflect.DeepEqual(got.ProvidedInjections, []string{"api", "*"}) {
		t.Fatalf("ProvidedInjections = %#v, want %#v", got.ProvidedInjections, []string{"api", "*"})
	}
	if !reflect.DeepEqual(got.UsedInjections, []string{"api"}) {
		t.Fatalf("UsedInjections = %#v, want %#v", got.UsedInjections, []string{"api"})
	}
	if got.Error != nil {
		t.Fatalf("Error = %v, want nil", *got.Error)
	}
}

func TestBridgeParseSendsCorrectInput(t *testing.T) {
	var (
		gotName  string
		gotArgs  []string
		received []byte
	)

	b := &Bridge{
		parserPath: "parser.bundle.js",
		runCmd: func(name string, args []string, input []byte) ([]byte, error) {
			gotName = name
			gotArgs = append([]string(nil), args...)
			received = append([]byte(nil), input...)
			return []byte(`{"results":[]}`), nil
		},
	}

	if _, err := b.Parse([]string{"/a.vue", "/b.vue"}, nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if gotName != "node" {
		t.Fatalf("command name = %q, want %q", gotName, "node")
	}
	if !reflect.DeepEqual(gotArgs, []string{"parser.bundle.js"}) {
		t.Fatalf("command args = %#v, want %#v", gotArgs, []string{"parser.bundle.js"})
	}

	var payload struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(received, &payload); err != nil {
		t.Fatalf("json.Unmarshal(input) error = %v", err)
	}
	if !reflect.DeepEqual(payload.Files, []string{"/a.vue", "/b.vue"}) {
		t.Fatalf("payload.Files = %#v, want %#v", payload.Files, []string{"/a.vue", "/b.vue"})
	}
}

func TestBridgeParsePropagatesCommandErrors(t *testing.T) {
	wantErr := errors.New("node failed")
	b := &Bridge{
		parserPath: "parser.bundle.js",
		runCmd: func(name string, args []string, input []byte) ([]byte, error) {
			return nil, wantErr
		},
	}

	_, err := b.Parse([]string{"/a.vue"}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Parse() error = %v, want wrapped %v", err, wantErr)
	}
}
