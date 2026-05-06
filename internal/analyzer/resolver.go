package analyzer

import (
	"path/filepath"
	"sort"
	"strings"
)

type Resolver struct {
	projectRoot string
	knownNodes  map[string]*Node
	byName      map[string]*Node
}

func NewResolver(projectRoot string, knownNodes map[string]*Node) *Resolver {
	byName := make(map[string]*Node, len(knownNodes))
	nodes := make([]*Node, 0, len(knownNodes))
	for _, n := range knownNodes {
		if n == nil {
			continue
		}
		nodes = append(nodes, n)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Path == nodes[j].Path {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Path < nodes[j].Path
	})

	for _, n := range nodes {
		if _, exists := byName[n.Name]; exists {
			continue
		}
		byName[n.Name] = n
	}

	return &Resolver{
		projectRoot: filepath.ToSlash(projectRoot),
		knownNodes:  knownNodes,
		byName:      byName,
	}
}

func (r *Resolver) Resolve(raw, fromFile string) (string, bool) {
	if r == nil {
		return "", false
	}

	raw = stripExt(filepath.ToSlash(strings.TrimSpace(raw)))
	fromFile = filepath.ToSlash(fromFile)

	switch {
	case raw == "":
		return "", false
	case strings.HasPrefix(raw, "#imports"):
		return "", false
	case strings.HasPrefix(raw, "~/"), strings.HasPrefix(raw, "@/"):
		return cleanRepoRel(raw[2:]), true
	case strings.HasPrefix(raw, "./"), strings.HasPrefix(raw, "../"):
		base := filepath.Dir(fromFile)
		if base == "." {
			base = ""
		}
		joined := filepath.ToSlash(filepath.Join(base, raw))
		return cleanRepoRel(joined), true
	default:
		return "", false
	}
}

func (r *Resolver) MatchTemplateRef(tag string) (string, bool) {
	if r == nil {
		return "", false
	}

	if n, ok := r.byName[tag]; ok && n != nil && n.Type == NodeTypeComponent {
		return n.ID, true
	}

	if pascal := kebabToPascal(tag); pascal != tag {
		if n, ok := r.byName[pascal]; ok && n != nil && n.Type == NodeTypeComponent {
			return n.ID, true
		}
	}

	if strings.HasPrefix(tag, "Lazy") && len(tag) > len("Lazy") {
		base := tag[len("Lazy"):]
		if n, ok := r.byName[base]; ok && n != nil && n.Type == NodeTypeComponent {
			return n.ID, true
		}
		if pascal := kebabToPascal(base); pascal != base {
			if n, ok := r.byName[pascal]; ok && n != nil && n.Type == NodeTypeComponent {
				return n.ID, true
			}
		}
	}

	return "", false
}

func (r *Resolver) MatchAutoImport(name string) (string, bool) {
	if r == nil {
		return "", false
	}

	if n, ok := r.byName[name]; ok && n != nil && (n.Type == NodeTypeComposable || n.Type == NodeTypeStore) {
		return n.ID, true
	}

	return "", false
}

func stripExt(p string) string {
	p = filepath.ToSlash(p)
	ext := filepath.Ext(p)
	if ext == "" {
		return p
	}
	switch ext {
	case ".vue", ".ts", ".js", ".tsx", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return strings.TrimSuffix(p, ext)
	default:
		return p
	}
}

func kebabToPascal(s string) string {
	if s == "" {
		return s
	}

	parts := strings.Split(s, "-")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	return b.String()
}

func cleanRepoRel(p string) string {
	p = filepath.ToSlash(filepath.Clean(p))
	return strings.TrimPrefix(p, "./")
}
