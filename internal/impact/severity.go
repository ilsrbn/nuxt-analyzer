package impact

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
)

// SeverityBand represents the risk level of a node.
type SeverityBand string

const (
	SeverityHigh   SeverityBand = "high"
	SeverityMedium SeverityBand = "medium"
	SeverityLow    SeverityBand = "low"
)

// SeverityEntry pairs a node ID with the reasons it received its band.
type SeverityEntry struct {
	NodeID  string
	Reasons []string
}

// SeverityResult holds scored nodes grouped by band.
type SeverityResult struct {
	High   []SeverityEntry
	Medium []SeverityEntry
	Low    []SeverityEntry
}

// Score assigns severity bands and tags to all nodes in the impact result.
func Score(g *analyzer.Graph, result *Result) *SeverityResult {
	sr := &SeverityResult{}
	if g == nil || result == nil {
		return sr
	}

	allIDs := make([]string, 0, len(result.ChangedNodes)+len(result.AffectedNodes))
	allIDs = append(allIDs, result.ChangedNodes...)
	allIDs = append(allIDs, result.AffectedNodes...)

	seen := make(map[string]bool, len(allIDs))
	for _, id := range allIDs {
		if seen[id] {
			continue
		}
		seen[id] = true

		n, ok := g.Nodes[id]
		if !ok || n == nil {
			continue
		}

		applyTags(n)
		routes := pagesReached(g, id)
		band, reasons := scoreBand(n, routes)
		n.ImpactScore = computeScore(band, n.DependentCount, routes)

		entry := SeverityEntry{NodeID: id, Reasons: reasons}
		switch band {
		case SeverityHigh:
			sr.High = append(sr.High, entry)
		case SeverityMedium:
			sr.Medium = append(sr.Medium, entry)
		default:
			sr.Low = append(sr.Low, entry)
		}
	}

	return sr
}

func scoreBand(n *analyzer.Node, routes int) (SeverityBand, []string) {
	var reasons []string

	switch n.Type {
	case analyzer.NodeTypePlugin:
		reasons = append(reasons, "is-plugin")
	case analyzer.NodeTypeLayout:
		reasons = append(reasons, "is-layout")
	case analyzer.NodeTypeMiddleware:
		reasons = append(reasons, "is-middleware")
	}
	if n.DependentCount >= 10 {
		reasons = append(reasons, "10+-dependents")
	}
	if routes >= 5 {
		reasons = append(reasons, fmt.Sprintf("reaches-%d-routes", routes))
	}
	if len(reasons) > 0 {
		return SeverityHigh, reasons
	}

	if n.DependentCount >= 2 {
		reasons = append(reasons, fmt.Sprintf("%d-dependents", n.DependentCount))
		return SeverityMedium, reasons
	}
	switch n.Type {
	case analyzer.NodeTypeComposable, analyzer.NodeTypeStore:
		reasons = append(reasons, "shared-"+string(n.Type))
		return SeverityMedium, reasons
	}

	reasons = append(reasons, "local-"+string(n.Type))
	return SeverityLow, reasons
}

func computeScore(band SeverityBand, dependents, routes int) int {
	base := map[SeverityBand]int{
		SeverityLow:    1,
		SeverityMedium: 34,
		SeverityHigh:   67,
	}[band]
	max := map[SeverityBand]int{
		SeverityLow:    33,
		SeverityMedium: 66,
		SeverityHigh:   100,
	}[band]

	score := base + (dependents * 2) + (routes * 3)
	if score > max {
		score = max
	}
	return score
}

func applyTags(n *analyzer.Node) {
	tags := make(map[string]bool, 4)

	p := filepath.ToSlash(n.Path)

	if n.Type == analyzer.NodeTypePlugin || n.Type == analyzer.NodeTypeLayout || p == "app.vue" {
		tags["global-risk"] = true
	}
	if n.Type == analyzer.NodeTypeMiddleware {
		tags["global-risk"] = true
		tags["navigation-risk"] = true
	}
	if strings.Contains(p, "auth") {
		tags["auth-risk"] = true
	}
	if strings.Contains(p, "payment") {
		tags["payments-risk"] = true
	}
	if n.Type == analyzer.NodeTypeComponent && n.DependentCount >= 5 {
		tags["ui-risk"] = true
	}

	n.Tags = make([]string, 0, len(tags))
	for t := range tags {
		n.Tags = append(n.Tags, t)
	}
	sort.Strings(n.Tags)
}

// pagesReached counts pages reachable from nodeID via reverse deps (BFS).
func pagesReached(g *analyzer.Graph, nodeID string) int {
	if g == nil {
		return 0
	}

	visited := make(map[string]bool)
	visited[nodeID] = true
	count := 0
	queue := []string{nodeID}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, dep := range g.ReverseDeps[curr] {
			if visited[dep] {
				continue
			}
			visited[dep] = true
			if n, ok := g.Nodes[dep]; ok && n != nil && n.Type == analyzer.NodeTypePage {
				count++
			}
			queue = append(queue, dep)
		}
	}
	return count
}
