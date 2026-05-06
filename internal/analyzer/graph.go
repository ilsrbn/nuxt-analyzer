package analyzer

import (
	"crypto/sha1"
	"encoding/hex"
	"path/filepath"
	"strings"
)

type NodeType string

const (
	NodeTypeComponent  NodeType = "component"
	NodeTypePage       NodeType = "page"
	NodeTypeLayout     NodeType = "layout"
	NodeTypeComposable NodeType = "composable"
	NodeTypeStore      NodeType = "store"
	NodeTypePlugin     NodeType = "plugin"
	NodeTypeMiddleware NodeType = "middleware"
	NodeTypeUtil       NodeType = "util"
	NodeTypeConfig     NodeType = "config"
	NodeTypeUnknown    NodeType = "unknown"
)

type EdgeKind string

const (
	EdgeTemplateUses      EdgeKind = "template-uses"
	EdgeScriptImports     EdgeKind = "script-imports"
	EdgeAutoImportUses    EdgeKind = "auto-import-uses"
	EdgeCalls             EdgeKind = "calls"
	EdgeInjects           EdgeKind = "injects"
	EdgeMiddlewareApplies EdgeKind = "middleware-applies"
	EdgeDynamic           EdgeKind = "dynamic"
	EdgeUncertain         EdgeKind = "uncertain"
)

type Confidence string

const (
	ConfHigh   Confidence = "high"
	ConfMedium Confidence = "medium"
	ConfLow    Confidence = "low"
)

type Node struct {
	ID              string
	Path            string
	Name            string
	Type            NodeType
	Tags            []string
	Route           *string
	Layout          *string
	IsChanged       bool
	IsAffected      bool
	DependentCount  int
	DependencyCount int
	ImpactScore     int
}

type Edge struct {
	From       string
	To         string
	Kind       EdgeKind
	Confidence Confidence
}

type Graph struct {
	Nodes       map[string]*Node
	Edges       []Edge
	ForwardDeps map[string][]string
	ReverseDeps map[string][]string
}

func NewGraph() *Graph {
	return &Graph{
		Nodes:       make(map[string]*Node),
		Edges:       make([]Edge, 0),
		ForwardDeps: make(map[string][]string),
		ReverseDeps: make(map[string][]string),
	}
}

func NodeID(repoRelPath string) string {
	sum := sha1.Sum([]byte(filepath.ToSlash(repoRelPath)))
	return hex.EncodeToString(sum[:])[:12]
}

func InferType(relPath string) NodeType {
	path := filepath.ToSlash(relPath)
	base := filepath.Base(path)

	if base == "app.vue" {
		return NodeTypeComponent
	}

	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "components":
			return NodeTypeComponent
		case "pages":
			return NodeTypePage
		case "layouts":
			return NodeTypeLayout
		case "composables":
			return NodeTypeComposable
		case "stores":
			return NodeTypeStore
		case "plugins":
			return NodeTypePlugin
		case "middleware":
			return NodeTypeMiddleware
		case "utils", "shared":
			return NodeTypeUtil
		}
	}

	return NodeTypeUnknown
}

func NodeName(relPath string) string {
	base := filepath.Base(filepath.ToSlash(relPath))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (g *Graph) AddNode(n *Node) {
	if g == nil || n == nil {
		return
	}
	if g.Nodes == nil {
		g.Nodes = make(map[string]*Node)
	}
	if g.ForwardDeps == nil {
		g.ForwardDeps = make(map[string][]string)
	}
	if g.ReverseDeps == nil {
		g.ReverseDeps = make(map[string][]string)
	}

	g.Nodes[n.ID] = n
	if _, ok := g.ForwardDeps[n.ID]; !ok {
		g.ForwardDeps[n.ID] = []string{}
	}
	if _, ok := g.ReverseDeps[n.ID]; !ok {
		g.ReverseDeps[n.ID] = []string{}
	}
}

func (g *Graph) AddEdge(e Edge) {
	if g == nil {
		return
	}
	if g.Edges == nil {
		g.Edges = make([]Edge, 0)
	}
	if g.ForwardDeps == nil {
		g.ForwardDeps = make(map[string][]string)
	}
	if g.ReverseDeps == nil {
		g.ReverseDeps = make(map[string][]string)
	}

	g.Edges = append(g.Edges, e)
	g.ForwardDeps[e.From] = append(g.ForwardDeps[e.From], e.To)
	g.ReverseDeps[e.To] = append(g.ReverseDeps[e.To], e.From)
}
