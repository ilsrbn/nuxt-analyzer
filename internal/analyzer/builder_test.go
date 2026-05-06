package analyzer

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ilsrbn/nuxt-analyzer/internal/parser"
)

type stubParseBridge struct {
	results []parser.ParsedFile
	err     error
	got     []string
}

func (s *stubParseBridge) Parse(files []string, _ []string) ([]parser.ParsedFile, error) {
	s.got = append([]string(nil), files...)
	if s.err != nil {
		return nil, s.err
	}
	return append([]parser.ParsedFile(nil), s.results...), nil
}

func TestBuilderBuild_CreatesNodesAndResolvesImportEdges(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	buttonAbs := filepath.Join(root, "components", "Button.vue")
	authAbs := filepath.Join(root, "composables", "useAuth.ts")
	indexAbs := filepath.Join(root, "pages", "index.vue")

	files := []FileInfo{
		{AbsPath: buttonAbs, RelPath: "components/Button.vue", Type: NodeTypeComponent},
		{AbsPath: authAbs, RelPath: "composables/useAuth.ts", Type: NodeTypeComposable},
		{AbsPath: indexAbs, RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	bridge := &stubParseBridge{results: []parser.ParsedFile{
		{
			Path:              buttonAbs,
			Type:              "component",
			Imports:           []string{"~/composables/useAuth"},
			TemplateRefs:      []string{},
			DynamicComponents: []string{"resolveComponent(name)"},
		},
		{Path: authAbs, Type: "composable"},
		{Path: indexAbs, Type: "page", TemplateRefs: []string{"Button"}},
	}}

	builder := Builder{ProjectRoot: root}
	graph, buildErrs, err := builder.Build(files, bridge)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(buildErrs) != 0 {
		t.Fatalf("len(buildErrs) = %d, want 0", len(buildErrs))
	}

	wantPaths := []string{buttonAbs, authAbs, indexAbs}
	if len(bridge.got) != len(wantPaths) {
		t.Fatalf("bridge.Parse() got %d paths, want %d", len(bridge.got), len(wantPaths))
	}
	for i, want := range wantPaths {
		if bridge.got[i] != want {
			t.Fatalf("bridge.Parse() path[%d] = %q, want %q", i, bridge.got[i], want)
		}
	}

	buttonID := NodeID("components/Button.vue")
	authID := NodeID("composables/useAuth.ts")
	indexID := NodeID("pages/index.vue")

	if got := graph.Nodes[buttonID]; got == nil {
		t.Fatalf("missing button node %q", buttonID)
	}
	if got := graph.Nodes[authID]; got == nil {
		t.Fatalf("missing auth node %q", authID)
	}
	pageNode := graph.Nodes[indexID]
	if pageNode == nil {
		t.Fatalf("missing page node %q", indexID)
	}
	if pageNode.Route == nil || *pageNode.Route != "/" {
		t.Fatalf("page route = %v, want /", pageNode.Route)
	}

	assertEdge(t, graph, Edge{From: buttonID, To: authID, Kind: EdgeScriptImports, Confidence: ConfHigh})
	assertEdge(t, graph, Edge{From: indexID, To: buttonID, Kind: EdgeTemplateUses, Confidence: ConfHigh})
	assertEdge(t, graph, Edge{From: buttonID, To: buttonID, Kind: EdgeDynamic, Confidence: ConfLow})

	if got := graph.Nodes[buttonID].DependencyCount; got != 2 {
		t.Fatalf("button DependencyCount = %d, want 2", got)
	}
	if got := graph.Nodes[buttonID].DependentCount; got != 2 {
		t.Fatalf("button DependentCount = %d, want 2", got)
	}
	if got := graph.Nodes[authID].DependentCount; got != 1 {
		t.Fatalf("useAuth DependentCount = %d, want 1", got)
	}
}

func TestBuilderBuild_StoresParseErrorsWithoutDroppingNode(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	buttonAbs := filepath.Join(root, "components", "Button.vue")

	files := []FileInfo{{AbsPath: buttonAbs, RelPath: "components/Button.vue", Type: NodeTypeComponent}}
	parseMsg := "unexpected token"
	bridge := &stubParseBridge{results: []parser.ParsedFile{{Path: buttonAbs, Type: "component", Error: &parseMsg}}}

	graph, buildErrs, err := (Builder{ProjectRoot: root}).Build(files, bridge)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, ok := graph.Nodes[NodeID("components/Button.vue")]; !ok {
		t.Fatal("expected node to be created even when parse error is present")
	}
	if len(buildErrs) != 1 {
		t.Fatalf("len(buildErrs) = %d, want 1", len(buildErrs))
	}
	if buildErrs[0] != (BuildError{File: "components/Button.vue", Message: parseMsg, Kind: "parse"}) {
		t.Fatalf("buildErrs[0] = %#v", buildErrs[0])
	}
}

func TestBuilderBuild_PropagatesBridgeErrors(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	buttonAbs := filepath.Join(root, "components", "Button.vue")
	files := []FileInfo{{AbsPath: buttonAbs, RelPath: "components/Button.vue", Type: NodeTypeComponent}}
	wantErr := errors.New("bridge failed")

	_, _, err := (Builder{ProjectRoot: root}).Build(files, &stubParseBridge{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Build() error = %v, want wrapped %v", err, wantErr)
	}
}

func TestBuilderBuild_ReturnsErrorForNilBridge(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	buttonAbs := filepath.Join(root, "components", "Button.vue")
	files := []FileInfo{{AbsPath: buttonAbs, RelPath: "components/Button.vue", Type: NodeTypeComponent}}

	_, _, err := (Builder{ProjectRoot: root}).Build(files, nil)
	if err == nil {
		t.Fatal("Build() error = nil, want non-nil")
	}
}

func TestRelPathToRoute(t *testing.T) {
	cases := []struct {
		relPath string
		want    string
	}{
		{relPath: "pages/index.vue", want: "/"},
		{relPath: "pages/blog/index.vue", want: "/blog"},
		{relPath: "pages/users/[id].vue", want: "/users/:id"},
		{relPath: "pages/docs/[...slug].vue", want: "/docs/*slug"},
		// Nuxt srcDir layout: app/pages/...
		{relPath: "app/pages/index.vue", want: "/"},
		{relPath: "app/pages/blog/index.vue", want: "/blog"},
		{relPath: "app/pages/users/[id].vue", want: "/users/:id"},
		{relPath: "app/pages/fragments/[slug].vue", want: "/fragments/:slug"},
	}

	for _, tc := range cases {
		t.Run(tc.relPath, func(t *testing.T) {
			if got := relPathToRoute(tc.relPath); got != tc.want {
				t.Fatalf("relPathToRoute(%q) = %q, want %q", tc.relPath, got, tc.want)
			}
		})
	}
}

func TestBuilderBuild_AutoImportEdgesFromNuxtMap(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	authAbs := filepath.Join(root, "composables", "useAuth.ts")
	indexAbs := filepath.Join(root, "pages", "index.vue")

	files := []FileInfo{
		{AbsPath: authAbs, RelPath: "composables/useAuth.ts", Type: NodeTypeComposable},
		{AbsPath: indexAbs, RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	autoImportMap := map[string]string{
		"useAuth": "composables/useAuth",
	}

	parsed := []parser.ParsedFile{
		{Path: authAbs, Type: "composable"},
		// index.vue uses useAuth via auto-import (no static import in source)
		{Path: indexAbs, Type: "page", UsedAutoImports: []string{"useAuth"}},
	}

	b := Builder{ProjectRoot: root}
	graph, buildErrs, err := b.buildFromParsed(files, parsed, autoImportMap)
	if err != nil {
		t.Fatalf("buildFromParsed() error = %v", err)
	}
	if len(buildErrs) != 0 {
		t.Fatalf("len(buildErrs) = %d, want 0", len(buildErrs))
	}

	indexID := NodeID("pages/index.vue")
	authID := NodeID("composables/useAuth.ts")
	assertEdge(t, graph, Edge{From: indexID, To: authID, Kind: EdgeAutoImportUses, Confidence: ConfMedium})
}

func TestBuilderBuild_AutoImportDeduplicatesAgainstStaticImport(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	authAbs := filepath.Join(root, "composables", "useAuth.ts")
	indexAbs := filepath.Join(root, "pages", "index.vue")

	files := []FileInfo{
		{AbsPath: authAbs, RelPath: "composables/useAuth.ts", Type: NodeTypeComposable},
		{AbsPath: indexAbs, RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	autoImportMap := map[string]string{
		"useAuth": "composables/useAuth",
	}

	parsed := []parser.ParsedFile{
		{Path: authAbs, Type: "composable"},
		// index.vue both statically imports AND appears in usedAutoImports — one edge only
		{
			Path:            indexAbs,
			Type:            "page",
			Imports:         []string{"~/composables/useAuth"},
			UsedAutoImports: []string{"useAuth"},
		},
	}

	b := Builder{ProjectRoot: root}
	graph, _, err := b.buildFromParsed(files, parsed, autoImportMap)
	if err != nil {
		t.Fatalf("buildFromParsed() error = %v", err)
	}

	indexID := NodeID("pages/index.vue")
	authID := NodeID("composables/useAuth.ts")
	count := 0
	for _, e := range graph.Edges {
		if e.From == indexID && e.To == authID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 edge from index to useAuth, got %d", count)
	}
}

func TestResolveToNodeID(t *testing.T) {
	nodes := map[string]*Node{
		NodeID("components/Button.vue"): {
			ID:   NodeID("components/Button.vue"),
			Path: "components/Button.vue",
		},
	}

	if got, ok := resolveToNodeID(nodes, "components/Button"); !ok || got != NodeID("components/Button.vue") {
		t.Fatalf("resolveToNodeID() = (%q, %v), want (%q, true)", got, ok, NodeID("components/Button.vue"))
	}
	if _, ok := resolveToNodeID(nodes, "components/Missing"); ok {
		t.Fatal("resolveToNodeID() unexpectedly matched missing node")
	}
}

func assertEdge(t *testing.T, graph *Graph, want Edge) {
	t.Helper()
	for _, got := range graph.Edges {
		if got == want {
			return
		}
	}
	t.Fatalf("missing edge %#v in %#v", want, graph.Edges)
}
