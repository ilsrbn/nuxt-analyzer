package nuxt

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var autoImportLineRe = regexp.MustCompile(`export\s*\{([^}]+)\}\s*from\s*["'](\.\./[^"']+)["']`)

// LoadAutoImportMap reads .nuxt/imports.d.ts and returns a map of
// exported name → project-relative path (without extension).
// Returns an empty map (no error) when .nuxt/imports.d.ts does not exist.
func LoadAutoImportMap(projectRoot string) (map[string]string, error) {
	result := make(map[string]string)

	f, err := os.Open(filepath.Join(projectRoot, ".nuxt", "imports.d.ts"))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // lines can be very long
	for scanner.Scan() {
		m := autoImportLineRe.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		rawPath := m[2] // e.g., "../app/composables/useFragments"
		if strings.HasPrefix(rawPath, "../node_modules") {
			continue
		}
		// Convert from .nuxt-relative ("../X") to project-root-relative ("X").
		relPath := filepath.ToSlash(strings.TrimPrefix(rawPath, "../"))

		for _, part := range strings.Split(m[1], ",") {
			name := parseExportedName(strings.TrimSpace(part))
			if name == "" {
				continue
			}
			result[name] = relPath
		}
	}

	return result, scanner.Err()
}

// parseExportedName extracts the exported identifier from a single element of
// an export list. Handles plain names ("useFragments") and re-export forms
// ("default as useFragments", "SeoOptions as SeoOptions").
func parseExportedName(s string) string {
	if idx := strings.LastIndex(s, " as "); idx >= 0 {
		s = strings.TrimSpace(s[idx+4:])
	}
	if !isValidIdentifier(s) {
		return ""
	}
	return s
}

func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_', c == '$':
			// always valid
		case i > 0 && c >= '0' && c <= '9':
			// valid after first char
		default:
			return false
		}
	}
	return true
}
