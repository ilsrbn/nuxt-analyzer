package parser

const tsImportQuery = `
(import_statement
  source: (string
    (string_fragment) @source))

(call_expression
  function: (import)
  arguments: (arguments
    (string
      (string_fragment) @source)))
`

const tsIdentifierQuery = `
(identifier) @identifier
`
