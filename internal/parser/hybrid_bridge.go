package parser

import (
	"os"
	"path/filepath"
)

type HybridBridge struct {
	node     *Bridge
	ts       *TypeScriptParser
	readFile func(string) ([]byte, error)
}

func NewHybrid() (*HybridBridge, error) {
	node, err := newNodeBridge()
	if err != nil {
		return nil, err
	}
	return &HybridBridge{
		node:     node,
		ts:       NewTypeScriptParser(),
		readFile: os.ReadFile,
	}, nil
}

// New returns the parser bridge used by the analyzer.
func New() (*HybridBridge, error) {
	return NewHybrid()
}

func (h *HybridBridge) Cleanup() {
	if h != nil && h.node != nil {
		h.node.Cleanup()
	}
}

func (h *HybridBridge) Parse(files []string, autoImportNames []string) ([]ParsedFile, error) {
	vueFiles := []string{}
	tsFiles := []string{}
	for _, file := range files {
		switch filepath.Ext(file) {
		case ".vue":
			vueFiles = append(vueFiles, file)
		case ".ts":
			tsFiles = append(tsFiles, file)
		}
	}

	results := make([]ParsedFile, 0, len(files))
	vueByPath := map[string]ParsedFile{}
	if len(vueFiles) > 0 {
		vueResults, err := h.node.Parse(vueFiles, autoImportNames)
		if err != nil {
			return nil, err
		}
		for _, result := range vueResults {
			vueByPath[result.Path] = result
		}
	}

	tsByPath := map[string]ParsedFile{}
	for _, file := range tsFiles {
		source, err := h.readFile(file)
		if err != nil {
			tsByPath[file] = emptyParsedFile(file, inferParsedType(file), err)
			continue
		}
		tsByPath[file] = h.ts.ParseSource(file, source, autoImportNames)
	}

	for _, file := range files {
		if result, ok := vueByPath[file]; ok {
			results = append(results, result)
			continue
		}
		if result, ok := tsByPath[file]; ok {
			results = append(results, result)
			continue
		}
	}

	return results, nil
}
