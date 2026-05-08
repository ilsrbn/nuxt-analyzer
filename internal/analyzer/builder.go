package analyzer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ilsrbn/nuxt-analyzer/internal/nuxt"
	"github.com/ilsrbn/nuxt-analyzer/internal/parser"
)

type BuildError struct {
	File    string
	Message string
	Kind    string
}

type Builder struct {
	ProjectRoot string
}

const dynamicInjectionProvider = "*"

type parseBridge interface {
	Parse(files []string, autoImportNames []string) ([]parser.ParsedFile, error)
}

func (b Builder) Build(files []FileInfo, bridge parseBridge) (*Graph, []BuildError, error) {
	if bridge == nil {
		return nil, nil, fmt.Errorf("node bridge parse: nil bridge")
	}

	// Load Nuxt auto-import map from .nuxt/imports.d.ts; best-effort, ignored on error.
	autoImportMap, err := nuxt.LoadAutoImportMap(b.ProjectRoot)
	if err != nil {
		autoImportMap = make(map[string]string)
	}
	autoImportNames := make([]string, 0, len(autoImportMap))
	for name := range autoImportMap {
		autoImportNames = append(autoImportNames, name)
	}

	absPaths := make([]string, len(files))
	for i, file := range files {
		absPaths[i] = file.AbsPath
	}

	parsed, err := bridge.Parse(absPaths, autoImportNames)
	if err != nil {
		return nil, nil, fmt.Errorf("node bridge parse: %w", err)
	}

	return b.buildFromParsed(files, parsed, autoImportMap)
}

func (b Builder) buildFromParsed(files []FileInfo, parsed []parser.ParsedFile, autoImportMap map[string]string) (*Graph, []BuildError, error) {
	graph := NewGraph()
	buildErrs := make([]BuildError, 0)
	infoByAbs := make(map[string]FileInfo, len(files))

	for _, file := range files {
		infoByAbs[file.AbsPath] = file

		node := &Node{
			ID:   NodeID(file.RelPath),
			Path: file.RelPath,
			Name: NodeName(file.RelPath),
			Type: file.Type,
			Tags: []string{},
		}
		if file.Type == NodeTypePage {
			route := relPathToRoute(file.RelPath)
			node.Route = &route
		}
		graph.AddNode(node)
	}

	resolver := NewResolver(b.ProjectRoot, graph.Nodes)

	for _, parsedFile := range parsed {
		info, ok := infoByAbs[parsedFile.Path]
		if !ok {
			continue
		}

		if parsedFile.Error != nil {
			buildErrs = append(buildErrs, BuildError{
				File:    info.RelPath,
				Message: *parsedFile.Error,
				Kind:    "parse",
			})
		}

		fromID := NodeID(info.RelPath)

		for _, imp := range parsedFile.Imports {
			resolvedRel, ok := resolver.Resolve(imp, info.RelPath)
			if !ok {
				continue
			}
			toID, ok := resolveToNodeID(graph.Nodes, resolvedRel)
			if !ok {
				continue
			}
			graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeScriptImports, Confidence: ConfHigh})
		}

		for _, ref := range parsedFile.TemplateRefs {
			toID, ok := resolver.MatchTemplateRef(ref)
			if !ok {
				continue
			}
			graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeTemplateUses, Confidence: ConfHigh})
		}

		for range parsedFile.DynamicComponents {
			graph.AddEdge(Edge{From: fromID, To: fromID, Kind: EdgeDynamic, Confidence: ConfLow})
		}
	}

	// Third pass: auto-import edges from .nuxt/imports.d.ts usage detected by the parser.
	existingEdges := make(map[string]struct{}, len(graph.Edges))
	for _, e := range graph.Edges {
		existingEdges[e.From+":"+e.To] = struct{}{}
	}

	for _, parsedFile := range parsed {
		info, ok := infoByAbs[parsedFile.Path]
		if !ok {
			continue
		}
		fromID := NodeID(info.RelPath)
		for _, name := range parsedFile.UsedAutoImports {
			relPath, ok := autoImportMap[name]
			if !ok {
				continue
			}
			toID, ok := resolveToNodeID(graph.Nodes, relPath)
			if !ok {
				continue
			}
			if fromID == toID {
				continue // skip self-loops: composable files reference their own function name
			}
			key := fromID + ":" + toID
			if _, exists := existingEdges[key]; exists {
				continue
			}
			existingEdges[key] = struct{}{}
			graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeAutoImportUses, Confidence: ConfMedium})
		}
	}

	pluginProviders := make(map[string][]string)
	dynamicPluginProviders := make([]string, 0)

	for _, parsedFile := range parsed {
		info, ok := infoByAbs[parsedFile.Path]
		if !ok || info.Type != NodeTypePlugin {
			continue
		}
		pluginID := NodeID(info.RelPath)
		for _, provided := range parsedFile.ProvidedInjections {
			if provided == "" {
				continue
			}
			if provided == dynamicInjectionProvider {
				dynamicPluginProviders = append(dynamicPluginProviders, pluginID)
				continue
			}
			pluginProviders[provided] = append(pluginProviders[provided], pluginID)
		}
	}

	for _, parsedFile := range parsed {
		info, ok := infoByAbs[parsedFile.Path]
		if !ok {
			continue
		}
		fromID := NodeID(info.RelPath)
		for _, used := range parsedFile.UsedInjections {
			for _, toID := range pluginProviders[used] {
				if fromID == toID {
					continue
				}
				key := fromID + ":" + toID
				if _, exists := existingEdges[key]; exists {
					continue
				}
				existingEdges[key] = struct{}{}
				graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeInjects, Confidence: ConfHigh})
			}
			for _, toID := range dynamicPluginProviders {
				if fromID == toID {
					continue
				}
				key := fromID + ":" + toID
				if _, exists := existingEdges[key]; exists {
					continue
				}
				existingEdges[key] = struct{}{}
				graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeInjects, Confidence: ConfLow})
			}
		}
	}

	for id, node := range graph.Nodes {
		node.DependencyCount = len(graph.ForwardDeps[id])
		node.DependentCount = len(graph.ReverseDeps[id])
	}

	return graph, buildErrs, nil
}

func resolveToNodeID(nodes map[string]*Node, relPath string) (string, bool) {
	candidates := []string{
		relPath + ".vue",
		relPath + ".ts",
		relPath,
		relPath + "/index.vue",
		relPath + "/index.ts",
		relPath + ".js",
		relPath + ".mjs",
		relPath + ".cjs",
		relPath + ".tsx",
		relPath + ".jsx",
		relPath + "/index.js",
		relPath + "/index.mjs",
		relPath + "/index.cjs",
		relPath + "/index.tsx",
		relPath + "/index.jsx",
	}

	for _, candidate := range candidates {
		id := NodeID(candidate)
		if _, ok := nodes[id]; ok {
			return id, true
		}
	}

	return "", false
}

func relPathToRoute(relPath string) string {
	route := filepath.ToSlash(relPath)
	pagesIdx := strings.Index(route, "pages/")
	if pagesIdx >= 0 {
		route = route[pagesIdx+len("pages/"):]
	}
	route = strings.TrimSuffix(route, filepath.Ext(route))
	if route == "index" {
		return "/"
	}
	route = strings.TrimSuffix(route, "/index")

	parts := strings.Split(route, "/")
	for i, part := range parts {
		switch {
		case strings.HasPrefix(part, "[[...") && strings.HasSuffix(part, "]]"):
			parts[i] = "*" + strings.TrimSuffix(strings.TrimPrefix(part, "[[..."), "]]")
		case strings.HasPrefix(part, "[...") && strings.HasSuffix(part, "]"):
			parts[i] = "*" + strings.TrimSuffix(strings.TrimPrefix(part, "[..."), "]")
		case strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]"):
			parts[i] = ":" + strings.TrimSuffix(strings.TrimPrefix(part, "["), "]")
		}
	}

	joined := strings.Join(parts, "/")
	joined = strings.Trim(joined, "/")
	if joined == "" {
		return "/"
	}
	return "/" + joined
}
