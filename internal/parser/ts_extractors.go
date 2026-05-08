package parser

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func extractTSImports(root *tree_sitter.Node, source []byte) []string {
	return queryStringCaptures(root, source, tsImportQuery, "source")
}

func extractTSAutoImportUsages(root *tree_sitter.Node, source []byte, autoImportNames []string) []string {
	if len(autoImportNames) == 0 {
		return []string{}
	}
	allowed := map[string]struct{}{}
	for _, name := range autoImportNames {
		allowed[name] = struct{}{}
	}

	identifiers := queryNodeCaptures(root, source, tsIdentifierQuery, "identifier")
	found := []string{}
	for _, node := range identifiers {
		name := node.Utf8Text(source)
		if _, ok := allowed[name]; !ok {
			continue
		}
		if isPropertyIdentifier(node) {
			continue
		}
		found = append(found, name)
	}
	return dedupeStrings(found)
}

var ignoredInjectionUsageNames = map[string]struct{}{
	"event": {}, "attrs": {}, "slots": {}, "refs": {}, "props": {}, "emit": {},
	"el": {}, "data": {}, "options": {}, "parent": {}, "root": {}, "nextTick": {},
	"forceUpdate": {}, "route": {}, "router": {}, "config": {},
}

func extractTSInjectionUsages(root *tree_sitter.Node, source []byte) []string {
	found := []string{}
	walkTS(root, func(node *tree_sitter.Node) {
		if node.Kind() != "identifier" {
			return
		}
		name := node.Utf8Text(source)
		if len(name) < 2 || name[0] != '$' {
			return
		}
		normalized := strings.TrimLeft(name, "$")
		if _, ignored := ignoredInjectionUsageNames[normalized]; ignored {
			return
		}
		if isObjectPropertyKey(node) {
			return
		}
		found = append(found, normalized)
	})
	return dedupeStrings(found)
}

func extractTSProvidedInjections(root *tree_sitter.Node, source []byte) []string {
	found := []string{}
	walkTS(root, func(node *tree_sitter.Node) {
		if node.Kind() != "call_expression" {
			return
		}
		function := node.ChildByFieldName("function")
		if function == nil || !isProvideFunction(function, source) {
			return
		}
		args := node.ChildByFieldName("arguments")
		firstArg := firstNamedChild(args)
		if firstArg == nil || firstArg.Kind() != "string" {
			found = append(found, DynamicInjectionProvider)
			return
		}
		value := firstStringFragment(firstArg, source)
		if value == "" {
			found = append(found, DynamicInjectionProvider)
			return
		}
		found = append(found, strings.TrimLeft(value, "$"))
	})
	return dedupeStrings(found)
}

func queryStringCaptures(root *tree_sitter.Node, source []byte, querySource string, captureName string) []string {
	nodes := queryNodeCaptures(root, source, querySource, captureName)
	out := []string{}
	for _, node := range nodes {
		out = append(out, node.Utf8Text(source))
	}
	return out
}

func queryNodeCaptures(root *tree_sitter.Node, source []byte, querySource string, captureName string) []*tree_sitter.Node {
	query, err := tree_sitter.NewQuery(root.Language(), querySource)
	if err != nil {
		return nil
	}
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	captureNames := query.CaptureNames()
	matches := cursor.Matches(query, root, source)
	out := []*tree_sitter.Node{}
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		for _, capture := range match.Captures {
			if captureNames[capture.Index] != captureName {
				continue
			}
			node := capture.Node
			out = append(out, &node)
		}
	}
	return out
}

func walkTS(node *tree_sitter.Node, visit func(*tree_sitter.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for i := uint(0); i < node.NamedChildCount(); i++ {
		walkTS(node.NamedChild(i), visit)
	}
}

func isPropertyIdentifier(node *tree_sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	property := parent.ChildByFieldName("property")
	return parent.Kind() == "member_expression" && sameTSNode(property, node)
}

func isObjectPropertyKey(node *tree_sitter.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	key := parent.ChildByFieldName("key")
	return parent.Kind() == "pair" && sameTSNode(key, node)
}

func isProvideFunction(node *tree_sitter.Node, source []byte) bool {
	text := node.Utf8Text(source)
	return text == "nuxtApp.provide" || text == "app.provide" || text == "useNuxtApp().provide"
}

func firstNamedChild(node *tree_sitter.Node) *tree_sitter.Node {
	if node == nil || node.NamedChildCount() == 0 {
		return nil
	}
	return node.NamedChild(0)
}

func firstStringFragment(node *tree_sitter.Node, source []byte) string {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child.Kind() == "string_fragment" {
			return child.Utf8Text(source)
		}
	}
	return ""
}

func sameTSNode(a *tree_sitter.Node, b *tree_sitter.Node) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Id() == b.Id()
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
