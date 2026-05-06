package assets

import _ "embed"

// ParserBundle is regenerated via `make build-parser`.
//
//go:embed parser.bundle.js
var ParserBundle []byte
