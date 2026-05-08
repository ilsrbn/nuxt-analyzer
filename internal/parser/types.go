package parser

const DynamicInjectionProvider = "*"

type ParsedType string

const (
	ParsedTypePage       ParsedType = "page"
	ParsedTypeLayout     ParsedType = "layout"
	ParsedTypeComponent  ParsedType = "component"
	ParsedTypeComposable ParsedType = "composable"
	ParsedTypeStore      ParsedType = "store"
	ParsedTypePlugin     ParsedType = "plugin"
	ParsedTypeMiddleware ParsedType = "middleware"
	ParsedTypeUtil       ParsedType = "util"
	ParsedTypeUnknown    ParsedType = "unknown"
)

// ParsedFile is the result of parsing one source file.
type ParsedFile struct {
	Path               string   `json:"path"`
	Type               string   `json:"type"`
	Imports            []string `json:"imports"`
	TemplateRefs       []string `json:"templateRefs"`
	DynamicComponents  []string `json:"dynamicComponents"`
	UsedAutoImports    []string `json:"usedAutoImports"`
	ProvidedInjections []string `json:"providedInjections"`
	UsedInjections     []string `json:"usedInjections"`
	Error              *string  `json:"error"`
}

func emptyParsedFile(path string, typ ParsedType, err error) ParsedFile {
	var msg *string
	if err != nil {
		text := err.Error()
		msg = &text
	}
	return ParsedFile{
		Path:               path,
		Type:               string(typ),
		Imports:            []string{},
		TemplateRefs:       []string{},
		DynamicComponents:  []string{},
		UsedAutoImports:    []string{},
		ProvidedInjections: []string{},
		UsedInjections:     []string{},
		Error:              msg,
	}
}
