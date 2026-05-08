package analyzer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileInfo struct {
	AbsPath string
	RelPath string
	Type    NodeType
}

func Scan(projectRoot string, includeTests bool) ([]FileInfo, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}

	files := make([]FileInfo, 0)
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) || os.IsNotExist(walkErr) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return walkErr
		}

		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if !isSourceFile(path) {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return fmt.Errorf("rel path for %q: %w", path, err)
		}
		relPath = filepath.ToSlash(relPath)

		if !includeTests && isTestFile(relPath) {
			return nil
		}

		typ := InferType(relPath)
		if typ == NodeTypeUnknown {
			return nil
		}

		files = append(files, FileInfo{
			AbsPath: path,
			RelPath: relPath,
			Type:    typ,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].RelPath < files[j].RelPath
	})

	return files, nil
}

func shouldSkipDir(name string) bool {
	switch name {
	case "node_modules", ".nuxt", ".output", "dist", ".git", ".worktrees":
		return true
	default:
		return false
	}
}

func isSourceFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".vue", ".ts", ".js", ".mjs", ".cjs", ".tsx", ".jsx":
		return true
	default:
		return false
	}
}

func isTestFile(relPath string) bool {
	base := filepath.Base(relPath)
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
}
