package reporter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
	"github.com/ilsrbn/nuxt-analyzer/internal/impact"
)

func TestBuildSetsSchemaVersion(t *testing.T) {
	report := Build(newBuildInput())

	if report.SchemaVersion != "1.0.0" {
		t.Fatalf("SchemaVersion = %q, want 1.0.0", report.SchemaVersion)
	}
}

func TestMarshalProducesValidJSONWithRequiredTopLevelKeys(t *testing.T) {
	report := Build(newBuildInput())

	data, err := Marshal(report, false)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	keys := []string{"schemaVersion", "meta", "project", "git", "graph", "impact", "severity", "errors"}
	for _, key := range keys {
		if _, ok := raw[key]; !ok {
			t.Fatalf("top-level key %q missing in %v", key, raw)
		}
	}
}

func TestMarshalPrettyModeContainsNewlines(t *testing.T) {
	report := Build(newBuildInput())

	data, err := Marshal(report, true)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	if string(data) == "" || !containsNewline(string(data)) {
		t.Fatalf("pretty JSON = %q, want output containing newlines", string(data))
	}
}

func TestBuildNormalizesNilSlicesAndSortsNodesByPath(t *testing.T) {
	graph := analyzer.NewGraph()
	graph.AddNode(&analyzer.Node{
		ID:   "b",
		Path: "pages/z.vue",
		Name: "z",
		Type: analyzer.NodeTypePage,
	})
	graph.AddNode(&analyzer.Node{
		ID:   "a",
		Path: "components/a.vue",
		Name: "a",
		Type: analyzer.NodeTypeComponent,
	})

	report := Build(BuildInput{
		ProjectName:    "test-app",
		ProjectRoot:    "/repo",
		GitBase:        "origin/main",
		GitHead:        "HEAD",
		Graph:          graph,
		ImpactResult:   &impact.Result{},
		SeverityResult: &impact.SeverityResult{},
		StartTime:      time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
	})

	if report.Git.ChangedFiles == nil {
		t.Fatal("ChangedFiles = nil, want empty slice")
	}
	if report.Graph.Circular == nil {
		t.Fatal("Circular = nil, want empty slice")
	}
	if report.Errors == nil {
		t.Fatal("Errors = nil, want empty slice")
	}
	if report.Impact.ChangedNodes == nil || report.Impact.AffectedNodes == nil || report.Impact.AffectedPages == nil || report.Impact.AffectedLayouts == nil {
		t.Fatalf("impact slices should be non-nil: %+v", report.Impact)
	}
	if report.Severity.High == nil || report.Severity.Medium == nil || report.Severity.Low == nil {
		t.Fatalf("severity slices should be non-nil: %+v", report.Severity)
	}
	if len(report.Graph.Nodes) != 2 {
		t.Fatalf("len(Graph.Nodes) = %d, want 2", len(report.Graph.Nodes))
	}
	if report.Graph.Nodes[0].Path != "components/a.vue" || report.Graph.Nodes[1].Path != "pages/z.vue" {
		t.Fatalf("Graph.Nodes paths = [%s %s], want sorted by path", report.Graph.Nodes[0].Path, report.Graph.Nodes[1].Path)
	}
}

func newBuildInput() BuildInput {
	graph := analyzer.NewGraph()
	route := "/dashboard"
	layout := "default"

	graph.AddNode(&analyzer.Node{
		ID:              "node-b",
		Path:            "pages/dashboard.vue",
		Name:            "dashboard",
		Type:            analyzer.NodeTypePage,
		Route:           &route,
		Layout:          &layout,
		IsAffected:      true,
		DependentCount:  2,
		DependencyCount: 1,
		ImpactScore:     34,
	})
	graph.AddNode(&analyzer.Node{
		ID:              "node-a",
		Path:            "components/Button.vue",
		Name:            "Button",
		Type:            analyzer.NodeTypeComponent,
		Tags:            []string{"ui-risk"},
		IsChanged:       true,
		IsAffected:      true,
		DependentCount:  5,
		DependencyCount: 2,
		ImpactScore:     67,
	})
	graph.AddEdge(analyzer.Edge{
		From:       "node-b",
		To:         "node-a",
		Kind:       analyzer.EdgeTemplateUses,
		Confidence: analyzer.ConfHigh,
	})

	return BuildInput{
		ProjectName:  "test-app",
		ProjectRoot:  "/repo",
		GitBase:      "origin/main",
		GitHead:      "HEAD",
		ChangedFiles: []string{"components/Button.vue"},
		Graph:        graph,
		ImpactResult: &impact.Result{
			ChangedNodes:    []string{"node-a"},
			AffectedNodes:   []string{"node-b"},
			AffectedPages:   []string{"node-b"},
			AffectedLayouts: []string{},
			AffectedRoutes: []impact.RouteRef{
				{NodeID: "node-b", Route: route, Layout: layout},
			},
			Paths: []impact.ImpactPath{
				{Source: "node-a", Target: "node-b", Nodes: []string{"node-a", "node-b"}},
			},
		},
		SeverityResult: &impact.SeverityResult{
			High: []impact.SeverityEntry{
				{NodeID: "node-a", Reasons: []string{"10+-dependents"}},
			},
			Medium: []impact.SeverityEntry{},
			Low:    []impact.SeverityEntry{},
		},
		Circular:  [][]string{},
		Errors:    []ReportError{},
		StartTime: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
	}
}

func containsNewline(s string) bool {
	for _, r := range s {
		if r == '\n' {
			return true
		}
	}
	return false
}
