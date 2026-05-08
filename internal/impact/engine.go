package impact

import (
	"sort"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
)

// Engine computes impact from changed nodes and detects dependency cycles.
type Engine struct{}

// RouteRef describes a page node with its resolved route and layout.
type RouteRef struct {
	NodeID string
	Route  string
	Layout string
}

// ImpactPath records the traversal path from a changed node to an affected node.
type ImpactPath struct {
	Source string
	Target string
	Nodes  []string
}

// Result holds the full impact analysis output.
type Result struct {
	ChangedNodes    []string
	AffectedNodes   []string
	AffectedPages   []string
	AffectedLayouts []string
	AffectedRoutes  []RouteRef
	Paths           []ImpactPath
}

// Analyze performs a reverse BFS from each changed node.
func (e *Engine) Analyze(g *analyzer.Graph, changedIDs []string) *Result {
	result := &Result{
		ChangedNodes:    uniqueStrings(changedIDs),
		AffectedNodes:   []string{},
		AffectedPages:   []string{},
		AffectedLayouts: []string{},
		AffectedRoutes:  []RouteRef{},
		Paths:           []ImpactPath{},
	}
	if g == nil {
		return result
	}

	for _, n := range g.Nodes {
		if n == nil {
			continue
		}
		n.IsChanged = false
		n.IsAffected = false
	}

	changedSet := make(map[string]struct{}, len(changedIDs))
	for _, id := range changedIDs {
		changedSet[id] = struct{}{}
	}

	affectedNodes := make(map[string]struct{}, len(g.Nodes))
	affectedPages := make(map[string]struct{}, len(g.Nodes))
	affectedLayouts := make(map[string]struct{}, len(g.Nodes))
	affectedRoutes := make(map[string]struct{}, len(g.Nodes))

	type queueItem struct {
		id   string
		path []string
	}

	type queueState struct {
		items []queueItem
		head  int
	}

	for _, id := range result.ChangedNodes {
		if n := g.Nodes[id]; n != nil {
			n.IsChanged = true
		}
	}

	for _, sourceID := range result.ChangedNodes {
		sourceVisited := make(map[string]struct{}, len(g.Nodes))
		sourceVisited[sourceID] = struct{}{}

		q := &queueState{items: make([]queueItem, 0, len(g.Nodes)+1)}
		q.items = append(q.items, queueItem{id: sourceID, path: []string{sourceID}})
		for q.head < len(q.items) {
			curr := q.items[q.head]
			q.head++

			for _, depID := range g.ReverseDeps[curr.id] {
				if _, ok := sourceVisited[depID]; ok {
					continue
				}
				sourceVisited[depID] = struct{}{}

				if _, ok := changedSet[depID]; ok {
					continue
				}

				nextPath := make([]string, len(curr.path)+1)
				copy(nextPath, curr.path)
				nextPath[len(curr.path)] = depID

				result.Paths = append(result.Paths, ImpactPath{
					Source: sourceID,
					Target: depID,
					Nodes:  nextPath,
				})

				if _, ok := affectedNodes[depID]; !ok {
					affectedNodes[depID] = struct{}{}
					result.AffectedNodes = append(result.AffectedNodes, depID)
				}

				if n := g.Nodes[depID]; n != nil {
					n.IsAffected = true
					switch n.Type {
					case analyzer.NodeTypePage:
						if _, ok := affectedPages[depID]; !ok {
							affectedPages[depID] = struct{}{}
							result.AffectedPages = append(result.AffectedPages, depID)
						}
						if n.Route != nil {
							if _, ok := affectedRoutes[depID]; !ok {
								affectedRoutes[depID] = struct{}{}
								layout := ""
								if n.Layout != nil {
									layout = *n.Layout
								}
								result.AffectedRoutes = append(result.AffectedRoutes, RouteRef{
									NodeID: depID,
									Route:  *n.Route,
									Layout: layout,
								})
							}
						}
					case analyzer.NodeTypeLayout:
						if _, ok := affectedLayouts[depID]; !ok {
							affectedLayouts[depID] = struct{}{}
							result.AffectedLayouts = append(result.AffectedLayouts, depID)
						}
					}
				}

				q.items = append(q.items, queueItem{id: depID, path: nextPath})
			}
		}
	}

	return result
}

// FindCircular detects cycles in the forward dependency graph via DFS.
func FindCircular(g *analyzer.Graph) [][]string {
	if g == nil {
		return nil
	}

	visited := make(map[string]bool, len(g.Nodes))
	inStack := make(map[string]int, len(g.Nodes))
	stack := make([]string, 0, len(g.Nodes))
	var cycles [][]string

	startNodes := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		startNodes = append(startNodes, id)
	}
	sort.Strings(startNodes)

	var dfs func(string)
	dfs = func(id string) {
		if idx, ok := inStack[id]; ok {
			cycle := append([]string{}, stack[idx:]...)
			cycle = append(cycle, id)
			cycles = append(cycles, canonicalizeCycle(cycle))
			return
		}
		if visited[id] {
			return
		}

		visited[id] = true
		inStack[id] = len(stack)
		stack = append(stack, id)

		deps := append([]string(nil), g.ForwardDeps[id]...)
		sort.Strings(deps)
		for _, dep := range deps {
			dfs(dep)
		}

		stack = stack[:len(stack)-1]
		delete(inStack, id)
	}

	for _, id := range startNodes {
		if visited[id] {
			continue
		}
		dfs(id)
	}

	sort.Slice(cycles, func(i, j int) bool {
		ci := cycles[i]
		cj := cycles[j]
		for k := 0; k < len(ci) && k < len(cj); k++ {
			if ci[k] == cj[k] {
				continue
			}
			return ci[k] < cj[k]
		}
		return len(ci) < len(cj)
	})

	return cycles
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func canonicalizeCycle(cycle []string) []string {
	if len(cycle) < 2 {
		return append([]string{}, cycle...)
	}

	body := cycle[:len(cycle)-1]
	if len(body) == 0 {
		return append([]string{}, cycle...)
	}

	minIdx := 0
	for i := 1; i < len(body); i++ {
		if body[i] < body[minIdx] {
			minIdx = i
		}
	}

	out := make([]string, 0, len(cycle))
	out = append(out, body[minIdx:]...)
	out = append(out, body[:minIdx]...)
	out = append(out, out[0])
	return out
}
