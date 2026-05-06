package impact

import (
	"reflect"
	"testing"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
)

func buildTestGraph() *analyzer.Graph {
	g := analyzer.NewGraph()

	route := "/dashboard"
	layout := "default"

	nodes := []*analyzer.Node{
		{ID: "util1", Path: "utils/util1.ts", Name: "util1", Type: analyzer.NodeTypeUtil},
		{ID: "comp1", Path: "components/comp1.vue", Name: "comp1", Type: analyzer.NodeTypeComponent},
		{ID: "cmp1", Path: "components/cmp1.vue", Name: "cmp1", Type: analyzer.NodeTypeComponent},
		{ID: "pg1", Path: "pages/dashboard.vue", Name: "dashboard", Type: analyzer.NodeTypePage, Route: &route, Layout: &layout},
		{ID: "layout1", Path: "layouts/default.vue", Name: "default", Type: analyzer.NodeTypeLayout},
	}

	for _, n := range nodes {
		g.AddNode(n)
	}

	g.AddEdge(analyzer.Edge{From: "comp1", To: "util1", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "cmp1", To: "comp1", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "pg1", To: "cmp1", Kind: analyzer.EdgeTemplateUses, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "layout1", To: "util1", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "pg1", To: "layout1", Kind: analyzer.EdgeTemplateUses, Confidence: analyzer.ConfHigh})

	return g
}

func TestEngine_Analyze_TransitiveAffectedNodesPagesRoutes(t *testing.T) {
	g := buildTestGraph()
	e := &Engine{}

	result := e.Analyze(g, []string{"util1"})

	if !containsAll(result.AffectedNodes, []string{"comp1", "cmp1", "layout1", "pg1"}) {
		t.Fatalf("AffectedNodes = %v, want transitive dependents", result.AffectedNodes)
	}
	if !containsAll(result.AffectedPages, []string{"pg1"}) {
		t.Fatalf("AffectedPages = %v, want pg1", result.AffectedPages)
	}
	if !containsAll(result.AffectedLayouts, []string{"layout1"}) {
		t.Fatalf("AffectedLayouts = %v, want layout1", result.AffectedLayouts)
	}
	if len(result.AffectedRoutes) != 1 {
		t.Fatalf("AffectedRoutes len = %d, want 1", len(result.AffectedRoutes))
	}
	if result.AffectedRoutes[0].NodeID != "pg1" || result.AffectedRoutes[0].Route != "/dashboard" || result.AffectedRoutes[0].Layout != "default" {
		t.Fatalf("AffectedRoutes[0] = %+v, want pg1 /dashboard default", result.AffectedRoutes[0])
	}
	if len(result.Paths) != 4 {
		t.Fatalf("Paths len = %d, want 4", len(result.Paths))
	}

	wantPaths := map[string][]string{
		"comp1":   {"util1", "comp1"},
		"cmp1":    {"util1", "comp1", "cmp1"},
		"layout1": {"util1", "layout1"},
	}
	for _, p := range result.Paths {
		if p.Target == "pg1" {
			if p.Source != "util1" || p.Nodes[0] != "util1" || p.Nodes[len(p.Nodes)-1] != "pg1" {
				t.Fatalf("path to pg1 = %v from %q, want a util1->pg1 path", p.Nodes, p.Source)
			}
			continue
		}
		if !reflect.DeepEqual(p.Nodes, wantPaths[p.Target]) {
			t.Fatalf("path to %s = %v, want %v", p.Target, p.Nodes, wantPaths[p.Target])
		}
		if p.Source != "util1" {
			t.Fatalf("path source = %q, want util1", p.Source)
		}
	}
}

func TestEngine_Analyze_ChangedNodeExcludedFromAffectedNodes(t *testing.T) {
	g := buildTestGraph()
	e := &Engine{}

	result := e.Analyze(g, []string{"util1"})

	for _, id := range result.AffectedNodes {
		if id == "util1" {
			t.Fatalf("changed node %q should not be in AffectedNodes", id)
		}
	}
	if !g.Nodes["util1"].IsChanged {
		t.Fatal("changed node should be marked IsChanged")
	}
	if g.Nodes["util1"].IsAffected {
		t.Fatal("changed node should not be marked IsAffected")
	}
}

func TestEngine_Analyze_ResetsFlagsBetweenRuns(t *testing.T) {
	g := buildTestGraph()
	e := &Engine{}

	first := e.Analyze(g, []string{"util1"})
	if len(first.AffectedNodes) == 0 {
		t.Fatal("expected first run to mark affected nodes")
	}
	if !g.Nodes["util1"].IsChanged || !g.Nodes["pg1"].IsAffected {
		t.Fatal("first run should set flags on changed and affected nodes")
	}

	second := e.Analyze(g, []string{"cmp1"})
	if len(second.AffectedNodes) == 0 {
		t.Fatal("expected second run to mark affected nodes")
	}
	if g.Nodes["util1"].IsChanged {
		t.Fatal("util1 IsChanged should be reset before second run")
	}
	if g.Nodes["util1"].IsAffected {
		t.Fatal("util1 IsAffected should be reset before second run")
	}
	if g.Nodes["layout1"].IsAffected {
		t.Fatal("layout1 IsAffected should be reset before second run")
	}
	if !g.Nodes["cmp1"].IsChanged {
		t.Fatal("cmp1 should be marked changed on second run")
	}
}

func TestEngine_Analyze_PreservesPathsForMultipleSources(t *testing.T) {
	g := analyzer.NewGraph()

	g.AddNode(&analyzer.Node{ID: "a", Path: "utils/a.ts", Name: "a", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "b", Path: "utils/b.ts", Name: "b", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "shared", Path: "components/shared.vue", Name: "shared", Type: analyzer.NodeTypeComponent})
	g.AddNode(&analyzer.Node{ID: "leaf", Path: "pages/leaf.vue", Name: "leaf", Type: analyzer.NodeTypePage})

	g.AddEdge(analyzer.Edge{From: "shared", To: "a", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "shared", To: "b", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "leaf", To: "shared", Kind: analyzer.EdgeTemplateUses, Confidence: analyzer.ConfHigh})

	e := &Engine{}
	result := e.Analyze(g, []string{"a", "b"})

	if !containsAll(result.AffectedNodes, []string{"shared", "leaf"}) {
		t.Fatalf("AffectedNodes = %v, want shared/leaf", result.AffectedNodes)
	}

	want := map[[2]string]bool{
		{"a", "shared"}: false,
		{"a", "leaf"}:   false,
		{"b", "shared"}: false,
		{"b", "leaf"}:   false,
	}
	for _, path := range result.Paths {
		key := [2]string{path.Source, path.Target}
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}

	for key, seen := range want {
		if !seen {
			t.Fatalf("missing path from %q to %q in %v", key[0], key[1], result.Paths)
		}
	}
}

func TestFindCircular_DetectsCycle(t *testing.T) {
	g := analyzer.NewGraph()
	g.AddNode(&analyzer.Node{ID: "c", Path: "c.ts", Name: "c", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "a", Path: "a.ts", Name: "a", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "b", Path: "b.ts", Name: "b", Type: analyzer.NodeTypeUtil})
	g.AddEdge(analyzer.Edge{From: "a", To: "b", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "b", To: "c", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "c", To: "a", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})

	cycles := FindCircular(g)
	if !reflect.DeepEqual(cycles, [][]string{{"a", "b", "c", "a"}}) {
		t.Fatalf("cycles = %v, want [[a b c a]]", cycles)
	}
}

func TestFindCircular_DeterministicOrder(t *testing.T) {
	g := analyzer.NewGraph()
	g.AddNode(&analyzer.Node{ID: "z", Path: "z.ts", Name: "z", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "b", Path: "b.ts", Name: "b", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "a", Path: "a.ts", Name: "a", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "y", Path: "y.ts", Name: "y", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "x", Path: "x.ts", Name: "x", Type: analyzer.NodeTypeUtil})

	g.AddEdge(analyzer.Edge{From: "b", To: "z", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "z", To: "a", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "a", To: "b", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})

	g.AddEdge(analyzer.Edge{From: "x", To: "y", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "y", To: "x", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})

	cycles := FindCircular(g)
	want := [][]string{
		{"a", "b", "z", "a"},
		{"x", "y", "x"},
	}
	if !reflect.DeepEqual(cycles, want) {
		t.Fatalf("cycles = %v, want %v", cycles, want)
	}
}

func TestFindCircular_NoCycles(t *testing.T) {
	g := analyzer.NewGraph()
	g.AddNode(&analyzer.Node{ID: "a", Path: "a.ts", Name: "a", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "b", Path: "b.ts", Name: "b", Type: analyzer.NodeTypeUtil})
	g.AddNode(&analyzer.Node{ID: "c", Path: "c.ts", Name: "c", Type: analyzer.NodeTypeUtil})
	g.AddEdge(analyzer.Edge{From: "a", To: "b", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
	g.AddEdge(analyzer.Edge{From: "b", To: "c", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})

	cycles := FindCircular(g)
	if len(cycles) != 0 {
		t.Fatalf("cycles = %v, want none", cycles)
	}
}

func containsAll(got []string, want []string) bool {
	set := make(map[string]struct{}, len(got))
	for _, v := range got {
		set[v] = struct{}{}
	}
	for _, v := range want {
		if _, ok := set[v]; !ok {
			return false
		}
	}
	return true
}
