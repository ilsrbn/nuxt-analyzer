package parser

import (
	"path/filepath"
	"strings"
)

func inferParsedType(path string) ParsedType {
	normalized := filepath.ToSlash(path)
	segments := strings.Split(normalized, "/")
	base := segments[len(segments)-1]

	if base == "app.vue" {
		return ParsedTypeComponent
	}

	for _, segment := range segments {
		switch segment {
		case "components":
			return ParsedTypeComponent
		case "pages":
			return ParsedTypePage
		case "layouts":
			return ParsedTypeLayout
		case "composables":
			return ParsedTypeComposable
		case "stores":
			return ParsedTypeStore
		case "plugins":
			return ParsedTypePlugin
		case "middleware":
			return ParsedTypeMiddleware
		case "utils", "shared":
			return ParsedTypeUtil
		}
	}

	return ParsedTypeUnknown
}

func inferVueParsedType(path string) ParsedType {
	typ := inferParsedType(path)
	if typ == ParsedTypePage || typ == ParsedTypeLayout || typ == ParsedTypeComponent {
		return typ
	}
	return ParsedTypeUnknown
}
