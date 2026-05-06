import { compileTemplate, parse } from '@vue/compiler-sfc'
import { readFileSync } from 'node:fs'

interface ParseInput {
  files: string[]
  autoImportNames?: string[]
}

type ParsedType =
  | 'page'
  | 'layout'
  | 'component'
  | 'composable'
  | 'store'
  | 'plugin'
  | 'middleware'
  | 'util'
  | 'unknown'

interface ParsedFile {
  path: string
  type: ParsedType
  imports: string[]
  templateRefs: string[]
  dynamicComponents: string[]
  usedAutoImports: string[]
  error: string | null
}

interface ParseOutput {
  results: ParsedFile[]
}

type UnknownRecord = Record<string, unknown>

const stdinChunks: Buffer[] = []

process.stdin.on('data', (chunk: Buffer) => {
  stdinChunks.push(chunk)
})

process.stdin.on('end', () => {
  try {
    const raw = Buffer.concat(stdinChunks).toString('utf8').trim()
    const parsed = raw === '' ? { files: [] } : (JSON.parse(raw) as ParseInput)
    const files = Array.isArray(parsed.files) ? parsed.files : []
    const autoImportNames = Array.isArray(parsed.autoImportNames) ? parsed.autoImportNames.map(String) : []
    const results = files.map((filePath) => parseFile(String(filePath), autoImportNames))
    writeOutput({ results })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    process.stderr.write(`parser input error: ${message}\n`)
    writeOutput({ results: [] })
    process.exitCode = 1
  }
})

process.stdin.resume()

function writeOutput(output: ParseOutput): void {
  process.stdout.write(JSON.stringify(output))
}

function parseFile(filePath: string, autoImportNames: string[]): ParsedFile {
  const inferredType = inferType(filePath)

  try {
    const content = readFileSync(filePath, 'utf8')
    if (filePath.endsWith('.vue')) {
      return parseVue(filePath, content, autoImportNames)
    }

    return parseTS(filePath, content, autoImportNames)
  } catch (error) {
    return empty(filePath, inferredType, errorMessage(error))
  }
}

function parseVue(filePath: string, content: string, autoImportNames: string[]): ParsedFile {
  const inferredType = inferVueType(filePath)

  try {
    const parsed = parse(content, { filename: filePath })
    const firstError = parsed.errors[0]
    if (firstError) {
      throw firstError
    }

    const imports: string[] = []
    const templateRefs: string[] = []
    const dynamicComponents: string[] = []
    const descriptor = parsed.descriptor
    const scriptContent = [descriptor.script?.content ?? '', descriptor.scriptSetup?.content ?? '']
      .filter((part) => part.length > 0)
      .join('\n')

    extractImports(scriptContent, imports)

    if (descriptor.template?.content) {
      extractTemplateRefs(filePath, descriptor.template.content, templateRefs, dynamicComponents)
    }

    return {
      path: filePath,
      type: inferredType,
      imports: dedupe(imports),
      templateRefs: dedupe(templateRefs),
      dynamicComponents: dedupe(dynamicComponents),
      usedAutoImports: extractAutoImportUsages(scriptContent, autoImportNames),
      error: null,
    }
  } catch (error) {
    return empty(filePath, inferredType, errorMessage(error))
  }
}

function parseTS(filePath: string, content: string, autoImportNames: string[]): ParsedFile {
  const imports: string[] = []
  extractImports(content, imports)

  return {
    path: filePath,
    type: inferType(filePath),
    imports: dedupe(imports),
    templateRefs: [],
    dynamicComponents: [],
    usedAutoImports: extractAutoImportUsages(content, autoImportNames),
    error: null,
  }
}

// extractAutoImportUsages returns the subset of autoImportNames that appear as
// standalone identifiers in content (not as property accesses like `.name`).
function extractAutoImportUsages(content: string, autoImportNames: string[]): string[] {
  if (autoImportNames.length === 0) {
    return []
  }
  const found: string[] = []
  for (const name of autoImportNames) {
    // Negative lookbehind for dot or word char prevents matching `foo.useFoo` or `notUseFoo`.
    const re = new RegExp(`(?<![.\\w])${escapeRegExp(name)}(?![\\w])`)
    if (re.test(content)) {
      found.push(name)
    }
  }
  return found
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function extractImports(content: string, out: string[]): void {
  const staticImportRe = /(?:^|[^\w$])import\s+(?:type\s+)?(?:[^'";\n]+?\s+from\s+)?['"]([^'"]+)['"]/gm
  const dynamicImportRe = /import\s*\(\s*['"]([^'"]+)['"]\s*\)/g

  collectMatches(staticImportRe, content, out)
  collectMatches(dynamicImportRe, content, out)
}

function collectMatches(pattern: RegExp, content: string, out: string[]): void {
  let match: RegExpExecArray | null

  while ((match = pattern.exec(content)) !== null) {
    if (match[1]) {
      out.push(match[1])
    }
  }
}

function extractTemplateRefs(
  filePath: string,
  templateContent: string,
  refs: string[],
  dynamics: string[]
): void {
  try {
    const compiled = compileTemplate({
      source: templateContent,
      filename: filePath,
      id: filePath,
    })

    const firstError = compiled.errors[0]
    if (firstError) {
      throw firstError
    }

    traverseTemplateAst(compiled.ast as UnknownRecord | undefined, refs, dynamics)
  } catch {
    extractTemplateRefsFallback(templateContent, refs, dynamics)
  }
}

function traverseTemplateAst(node: UnknownRecord | undefined, refs: string[], dynamics: string[]): void {
  if (!node) {
    return
  }

  if (node.type === 1) {
    const tag = typeof node.tag === 'string' ? node.tag : ''
    if (tag === 'component') {
      collectDynamicComponent(node, dynamics)
    } else if (isComponentTag(tag)) {
      refs.push(tag)
    }
  }

  for (const key of ['children', 'branches'] as const) {
    const value = node[key]
    if (Array.isArray(value)) {
      for (const child of value) {
        if (isRecord(child)) {
          traverseTemplateAst(child, refs, dynamics)
        }
      }
    }
  }
}

function collectDynamicComponent(node: UnknownRecord, dynamics: string[]): void {
  const props = Array.isArray(node.props) ? node.props : []

  for (const prop of props) {
    if (!isRecord(prop)) {
      continue
    }

    if (prop.type === 6 && prop.name === 'is') {
      const value = nestedString(prop, ['value', 'content'])
      if (value) {
        dynamics.push(formatStaticDynamicComponent(value))
      }
      continue
    }

    if (prop.type === 7 && prop.name === 'bind' && nestedString(prop, ['arg', 'content']) === 'is') {
      const expression =
        nestedString(prop, ['exp', 'content']) ??
        nestedString(prop, ['exp', 'loc', 'source']) ??
        nestedString(prop, ['loc', 'source'])

      if (expression) {
        dynamics.push(normalizeDynamicComponentExpression(expression))
      }
    }
  }
}

function extractTemplateRefsFallback(templateContent: string, refs: string[], dynamics: string[]): void {
  const componentTagRe = /<\s*([A-Z][A-Za-z0-9]*|[a-z]+(?:-[a-z0-9]+)+)\b/g
  const dynamicBindRe = /<\s*component\b[^>]*\b:is\s*=\s*(["'])(.*?)\1/gis
  const dynamicStaticRe = /<\s*component\b[^>]*\bis\s*=\s*(["'])(.*?)\1/gis

  let match: RegExpExecArray | null

  while ((match = componentTagRe.exec(templateContent)) !== null) {
    refs.push(match[1])
  }

  while ((match = dynamicBindRe.exec(templateContent)) !== null) {
    dynamics.push(normalizeDynamicComponentExpression(match[2]))
  }

  while ((match = dynamicStaticRe.exec(templateContent)) !== null) {
    dynamics.push(formatStaticDynamicComponent(match[2]))
  }
}

function normalizeDynamicComponentExpression(expression: string): string {
  const trimmed = expression.trim().replace(/^_ctx\./, '')
  const quoted = trimmed.match(/^['"]([^'"]+)['"]$/)
  if (quoted) {
    return formatStaticDynamicComponent(quoted[1])
  }

  const unref = trimmed.match(/^_unref\((.+)\)$/)
  if (unref) {
    return unref[1]
  }

  return trimmed
}

function formatStaticDynamicComponent(name: string): string {
  return `resolveComponent('${name.trim()}')`
}

function inferType(filePath: string): ParsedType {
  const normalized = normalizePath(filePath)
  const segments = normalized.split('/').filter((segment) => segment.length > 0 && segment !== '.')
  const baseName = segments[segments.length - 1]

  if (baseName === 'app.vue') {
    return 'component'
  }

  for (const segment of segments) {
    switch (segment) {
      case 'components':
        return 'component'
      case 'pages':
        return 'page'
      case 'layouts':
        return 'layout'
      case 'composables':
        return 'composable'
      case 'stores':
        return 'store'
      case 'plugins':
        return 'plugin'
      case 'middleware':
        return 'middleware'
      case 'utils':
      case 'shared':
        return 'util'
    }
  }

  return 'unknown'
}

function inferVueType(filePath: string): ParsedType {
  const type = inferType(filePath)

  if (type === 'page' || type === 'layout' || type === 'component') {
    return type
  }

  return 'unknown'
}

function empty(filePath: string, type: ParsedType, error: string): ParsedFile {
  return {
    path: filePath,
    type,
    imports: [],
    templateRefs: [],
    dynamicComponents: [],
    usedAutoImports: [],
    error,
  }
}

function dedupe(values: string[]): string[] {
  return [...new Set(values.filter((value) => value.length > 0))]
}

function normalizePath(filePath: string): string {
  return filePath.replace(/\\/g, '/')
}

function isComponentTag(tag: string): boolean {
  return /^[A-Z]/.test(tag) || /^[a-z]+(?:-[a-z0-9]+)+$/.test(tag)
}

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === 'object' && value !== null
}

function nestedString(record: UnknownRecord, path: string[]): string | undefined {
  let current: unknown = record

  for (const key of path) {
    if (!isRecord(current) || !(key in current)) {
      return undefined
    }
    current = current[key]
  }

  return typeof current === 'string' ? current : undefined
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  if (isRecord(error)) {
    const message = error.message
    if (typeof message === 'string') {
      return message
    }
  }

  return String(error)
}
