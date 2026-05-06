package main

import (
	"testing"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
)

func TestReportGraphAndCyclesChangedFilesOnlyFiltersCyclesToKeptNodes(t *testing.T) {
	g := analyzer.NewGraph()
	nodeA := &analyzer.Node{ID: "a", Path: "components/A.vue"}
	nodeB := &analyzer.Node{ID: "b", Path: "components/B.vue"}
	nodeC := &analyzer.Node{ID: "c", Path: "components/C.vue"}
	nodeD := &analyzer.Node{ID: "d", Path: "components/D.vue"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddNode(nodeC)
	g.AddNode(nodeD)

	g.AddEdge(analyzer.Edge{From: "a", To: "b"})
	g.AddEdge(analyzer.Edge{From: "b", To: "a"})
	g.AddEdge(analyzer.Edge{From: "c", To: "d"})
	g.AddEdge(analyzer.Edge{From: "d", To: "c"})

	reportGraph, circular := reportGraphAndCycles(g, true, []string{"a"}, []string{"b"})

	if len(reportGraph.Nodes) != 2 {
		t.Fatalf("filtered graph node count = %d, want 2", len(reportGraph.Nodes))
	}
	if _, ok := reportGraph.Nodes["c"]; ok {
		t.Fatal("filtered graph contains unexpected node c")
	}
	if len(circular) != 1 {
		t.Fatalf("circular len = %d, want 1; got %#v", len(circular), circular)
	}
	for _, id := range circular[0] {
		if _, ok := reportGraph.Nodes[id]; !ok {
			t.Fatalf("cycle references node %q missing from filtered graph", id)
		}
	}
}

func TestReportGraphAndCyclesFullGraphKeepsOriginalCycles(t *testing.T) {
	g := analyzer.NewGraph()
	nodeA := &analyzer.Node{ID: "a", Path: "components/A.vue"}
	nodeB := &analyzer.Node{ID: "b", Path: "components/B.vue"}

	g.AddNode(nodeA)
	g.AddNode(nodeB)
	g.AddEdge(analyzer.Edge{From: "a", To: "b"})
	g.AddEdge(analyzer.Edge{From: "b", To: "a"})

	reportGraph, circular := reportGraphAndCycles(g, false, []string{"a"}, []string{"b"})

	if reportGraph != g {
		t.Fatal("full report graph should reuse original graph")
	}
	if len(circular) != 1 {
		t.Fatalf("circular len = %d, want 1", len(circular))
	}
}
