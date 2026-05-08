package parser

import (
	"context"
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type TypeScriptParser struct{}

func NewTypeScriptParser() *TypeScriptParser {
	return &TypeScriptParser{}
}

func (p *TypeScriptParser) ParseSource(path string, source []byte, autoImportNames []string) ParsedFile {
	result := emptyParsedFile(path, inferParsedType(path), nil)

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())); err != nil {
		return emptyParsedFile(path, inferParsedType(path), fmt.Errorf("set typescript grammar: %w", err))
	}

	tree := parser.ParseCtx(context.Background(), source, nil)
	if tree == nil || tree.RootNode() == nil {
		return emptyParsedFile(path, inferParsedType(path), fmt.Errorf("parse typescript source"))
	}
	defer tree.Close()

	root := tree.RootNode()
	result.Imports = dedupeStrings(extractTSImports(root, source))
	result.UsedAutoImports = dedupeStrings(extractTSAutoImportUsages(root, source, autoImportNames))
	result.ProvidedInjections = dedupeStrings(extractTSProvidedInjections(root, source))
	result.UsedInjections = dedupeStrings(extractTSInjectionUsages(root, source))
	return result
}
