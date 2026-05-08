package parser

import (
	"os"
	"path/filepath"
	"strings"
)

type HybridBridge struct {
	node     *Bridge
	ts       *TypeScriptParser
	readFile func(string) ([]byte, error)
	CleanupFn func()
}

func NewHybrid() (*HybridBridge, error) {
	node, cleanup, err := newNodeBridge()
	if err != nil {
		cleanup()
		return nil, err
	}
	h := &HybridBridge{
		node:     node,
		ts:       NewTypeScriptParser(),
		readFile: os.ReadFile,
	}
	h.CleanupFn = cleanup
	return h, nil
}

// New returns the parser bridge used by the analyzer.
func New() (*HybridBridge, error) {
	return NewHybrid()
}

func (h *HybridBridge) Cleanup() {
	if h != nil && h.node != nil {
		h.node.Cleanup()
	}
	if h != nil && h.CleanupFn != nil {
		h.CleanupFn()
	}
}

func (h *HybridBridge) Parse(files []string, autoImportNames []string) ([]ParsedFile, error) {
	vueFiles := []string{}
	tsFiles := []string{}
	for _, file := range files {
		switch strings.ToLower(filepath.Ext(file)) {
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
