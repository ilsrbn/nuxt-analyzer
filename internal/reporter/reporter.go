package reporter

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
	"github.com/ilsrbn/nuxt-analyzer/internal/impact"
)

const schemaVersion = "1.0.0"

type Report struct {
	SchemaVersion string        `json:"schemaVersion"`
	Meta          Meta          `json:"meta"`
	Project       Project       `json:"project"`
	Git           Git           `json:"git"`
	Graph         GraphOutput   `json:"graph"`
	Impact        ImpactOutput  `json:"impact"`
	Severity      SeverityOut   `json:"severity"`
	Errors        []ReportError `json:"errors"`
}

type Meta struct {
	AnalyzedAt    string `json:"analyzedAt"`
	DurationMs    int64  `json:"durationMs"`
	TotalNodes    int    `json:"totalNodes"`
	TotalEdges    int    `json:"totalEdges"`
	ChangedCount  int    `json:"changedCount"`
	AffectedCount int    `json:"affectedCount"`
}

type Project struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

type Git struct {
	Base         string   `json:"base"`
	Head         string   `json:"head"`
	ChangedFiles []string `json:"changedFiles"`
}

type NodeOut struct {
	ID              string   `json:"id"`
	Path            string   `json:"path"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Tags            []string `json:"tags"`
	Route           *string  `json:"route"`
	Layout          *string  `json:"layout"`
	IsChanged       bool     `json:"isChanged"`
	IsAffected      bool     `json:"isAffected"`
	DependentCount  int      `json:"dependentCount"`
	DependencyCount int      `json:"dependencyCount"`
	ImpactScore     int      `json:"impactScore"`
}

type EdgeOut struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Confidence string `json:"confidence"`
}

type GraphOutput struct {
	Nodes    []NodeOut  `json:"nodes"`
	Edges    []EdgeOut  `json:"edges"`
	Circular [][]string `json:"circular"`
}

type RouteRefOut struct {
	NodeID string `json:"nodeId"`
	Route  string `json:"route"`
	Layout string `json:"layout"`
}

type PathOut struct {
	Source string   `json:"source"`
	Target string   `json:"target"`
	Nodes  []string `json:"nodes"`
}

type ImpactOutput struct {
	ChangedNodes    []string      `json:"changedNodes"`
	AffectedNodes   []string      `json:"affectedNodes"`
	AffectedPages   []string      `json:"affectedPages"`
	AffectedLayouts []string      `json:"affectedLayouts"`
	AffectedRoutes  []RouteRefOut `json:"affectedRoutes"`
	Paths           []PathOut     `json:"paths"`
}

type SeverityEntry struct {
	NodeID  string   `json:"nodeId"`
	Reasons []string `json:"reasons"`
}

type SeverityOut struct {
	High   []SeverityEntry `json:"high"`
	Medium []SeverityEntry `json:"medium"`
	Low    []SeverityEntry `json:"low"`
}

type ReportError struct {
	File    string `json:"file"`
	Message string `json:"message"`
	Kind    string `json:"kind"`
}

type BuildInput struct {
	ProjectName    string
	ProjectRoot    string
	GitBase        string
	GitHead        string
	ChangedFiles   []string
	Graph          *analyzer.Graph
	ImpactResult   *impact.Result
	SeverityResult *impact.SeverityResult
	Circular       [][]string
	Errors         []ReportError
	StartTime      time.Time
}

func Build(in BuildInput) *Report {
	graph := in.Graph
	if graph == nil {
		graph = analyzer.NewGraph()
	}

	impactResult := in.ImpactResult
	if impactResult == nil {
		impactResult = &impact.Result{}
	}

	severityResult := in.SeverityResult
	if severityResult == nil {
		severityResult = &impact.SeverityResult{}
	}

	startTime := in.StartTime.UTC()
	if startTime.IsZero() {
		startTime = time.Now().UTC()
	}

	nodes := make([]NodeOut, 0, len(graph.Nodes))
	for _, n := range graph.Nodes {
		if n == nil {
			continue
		}
		nodes = append(nodes, NodeOut{
			ID:              n.ID,
			Path:            n.Path,
			Name:            n.Name,
			Type:            string(n.Type),
			Tags:            nonNil(n.Tags),
			Route:           n.Route,
			Layout:          n.Layout,
			IsChanged:       n.IsChanged,
			IsAffected:      n.IsAffected,
			DependentCount:  n.DependentCount,
			DependencyCount: n.DependencyCount,
			ImpactScore:     n.ImpactScore,
		})
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Path == nodes[j].Path {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Path < nodes[j].Path
	})

	edges := make([]EdgeOut, 0, len(graph.Edges))
	for _, e := range graph.Edges {
		edges = append(edges, EdgeOut{
			From:       e.From,
			To:         e.To,
			Kind:       string(e.Kind),
			Confidence: string(e.Confidence),
		})
	}

	routes := make([]RouteRefOut, 0, len(impactResult.AffectedRoutes))
	for _, route := range impactResult.AffectedRoutes {
		routes = append(routes, RouteRefOut{
			NodeID: route.NodeID,
			Route:  route.Route,
			Layout: route.Layout,
		})
	}

	paths := make([]PathOut, 0, len(impactResult.Paths))
	for _, path := range impactResult.Paths {
		paths = append(paths, PathOut{
			Source: path.Source,
			Target: path.Target,
			Nodes:  nonNil(path.Nodes),
		})
	}

	return &Report{
		SchemaVersion: schemaVersion,
		Meta: Meta{
			AnalyzedAt:    startTime.Format(time.RFC3339),
			DurationMs:    durationMilliseconds(startTime),
			TotalNodes:    len(nodes),
			TotalEdges:    len(edges),
			ChangedCount:  len(impactResult.ChangedNodes),
			AffectedCount: len(impactResult.AffectedNodes),
		},
		Project: Project{
			Name: in.ProjectName,
			Root: in.ProjectRoot,
		},
		Git: Git{
			Base:         in.GitBase,
			Head:         in.GitHead,
			ChangedFiles: nonNil(in.ChangedFiles),
		},
		Graph: GraphOutput{
			Nodes:    nodes,
			Edges:    edges,
			Circular: nonNil2D(in.Circular),
		},
		Impact: ImpactOutput{
			ChangedNodes:    nonNil(impactResult.ChangedNodes),
			AffectedNodes:   nonNil(impactResult.AffectedNodes),
			AffectedPages:   nonNil(impactResult.AffectedPages),
			AffectedLayouts: nonNil(impactResult.AffectedLayouts),
			AffectedRoutes:  routes,
			Paths:           paths,
		},
		Severity: SeverityOut{
			High:   toSeverityEntries(severityResult.High),
			Medium: toSeverityEntries(severityResult.Medium),
			Low:    toSeverityEntries(severityResult.Low),
		},
		Errors: nonNilErrors(in.Errors),
	}
}

func Marshal(r *Report, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(r, "", "  ")
	}
	return json.Marshal(r)
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNil2D(values [][]string) [][]string {
	if values == nil {
		return [][]string{}
	}
	return values
}

func nonNilErrors(values []ReportError) []ReportError {
	if values == nil {
		return []ReportError{}
	}
	return values
}

func toSeverityEntries(entries []impact.SeverityEntry) []SeverityEntry {
	if entries == nil {
		return []SeverityEntry{}
	}

	out := make([]SeverityEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, SeverityEntry{
			NodeID:  entry.NodeID,
			Reasons: nonNil(entry.Reasons),
		})
	}
	return out
}

func durationMilliseconds(start time.Time) int64 {
	duration := time.Since(start).Milliseconds()
	if duration < 0 {
		return 0
	}
	return duration
}
