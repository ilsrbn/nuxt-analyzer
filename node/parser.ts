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
  providedInjections: string[]
  usedInjections: string[]
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
    const normalizedPath = normalizePath(filePath)
    if (normalizedPath.endsWith('.vue')) {
      return parseVue(filePath, content, autoImportNames)
    }

    return empty(filePath, inferredType, `unsupported file extension for Vue parser: ${filePath}`)
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
    const templateContent = descriptor.template?.content ?? ''

    extractImports(scriptContent, imports)

    if (templateContent) {
      extractTemplateRefs(filePath, templateContent, templateRefs, dynamicComponents)
    }

    return {
      path: filePath,
      type: inferredType,
      imports: dedupe(imports),
      templateRefs: dedupe(templateRefs),
      dynamicComponents: dedupe(dynamicComponents),
      usedAutoImports: extractAutoImportUsages(scriptContent, autoImportNames),
      providedInjections: extractProvidedInjections(scriptContent),
      usedInjections: dedupe([
        ...extractInjectionUsages(scriptContent, { ignoreJsCommentsAndStrings: true }),
        ...extractTemplateInjectionUsages(filePath, templateContent),
      ]),
      error: null,
    }
  } catch (error) {
    return empty(filePath, inferredType, errorMessage(error))
  }
}

const dynamicInjectionProvider = '*'
const ignoredInjectionUsageNames = new Set([
  'event',
  'attrs',
  'slots',
  'refs',
  'props',
  'emit',
  'el',
  'data',
  'options',
  'parent',
  'root',
  'nextTick',
  'forceUpdate',
  'route',
  'router',
  'config',
])

// extractAutoImportUsages returns the subset of autoImportNames that appear as
// standalone identifiers in content (not as property accesses like `.name`).
function extractAutoImportUsages(content: string, autoImportNames: string[]): string[] {
  if (autoImportNames.length === 0) {
    return []
  }
  const searchable = stripJsCommentsAndStrings(content)
  const found: string[] = []
  for (const name of autoImportNames) {
    const re = new RegExp(`(?<![.\\w])${escapeRegExp(name)}(?![\\w])`)
    if (re.test(searchable)) {
      found.push(name)
    }
  }
  return found
}

function extractProvidedInjections(content: string): string[] {
  const found: string[] = []
  const searchable = stripJsCommentsAndStrings(content)
  const provideObjectRe = /\bprovide\s*:\s*\{/g
  let match: RegExpExecArray | null

  while ((match = provideObjectRe.exec(searchable)) !== null) {
    const openBrace = searchable.indexOf('{', match.index)
    const closeBrace = findMatchingBrace(searchable, openBrace)
    if (closeBrace === -1) {
      continue
    }
    collectProvideObjectKeys(content.slice(openBrace + 1, closeBrace), found)
    provideObjectRe.lastIndex = closeBrace + 1
  }

  const provideCallRe = /(?<![\w$.])(?:(?:nuxtApp|app)|useNuxtApp\(\))\s*\.\s*provide\s*\(/g
  while ((match = provideCallRe.exec(searchable)) !== null) {
    if (!isRootLikeProviderMatch(searchable, match.index)) {
      continue
    }

    const openParen = match.index + match[0].length - 1
    const closeParen = findMatchingParen(searchable, openParen)
    if (closeParen === -1) {
      found.push(dynamicInjectionProvider)
      continue
    }

    const args = splitTopLevel(stripJsComments(content.slice(openParen + 1, closeParen)), ',')
    const firstArg = args[0]?.trim()
    const staticName = firstArg ? staticStringLiteralContent(firstArg) : undefined
    found.push(staticName === undefined ? dynamicInjectionProvider : normalizeInjectionName(staticName))
    provideCallRe.lastIndex = closeParen + 1
  }

  return dedupe(found)
}

function isRootLikeProviderMatch(content: string, index: number): boolean {
  for (let i = index - 1; i >= 0; i--) {
    if (/\s/.test(content[i])) {
      continue
    }

    return content[i] !== '.' && !/[\w$]/.test(content[i])
  }

  return true
}

function extractInjectionUsages(
  content: string,
  options: { ignoreJsCommentsAndStrings?: boolean } = {}
): string[] {
  const found: string[] = []
  const searchable = options.ignoreJsCommentsAndStrings ? stripJsCommentsAndStrings(content) : content
  const injectionRe = /(?<![\w$])\$([A-Za-z_][\w$]*)\b/g
  let match: RegExpExecArray | null

  while ((match = injectionRe.exec(searchable)) !== null) {
    if (isQuotedObjectKeyMatch(searchable, match.index, match[0].length)) {
      continue
    }
    const name = normalizeInjectionName(match[0])
    if (ignoredInjectionUsageNames.has(name)) {
      continue
    }
    found.push(name)
  }

  return dedupe(found)
}

function stripHtmlComments(content: string): string {
  return content.replace(/<!--[\s\S]*?-->/g, (comment) => comment.replace(/[^\n]/g, ' '))
}

function extractTemplateInjectionUsages(filePath: string, templateContent: string): string[] {
  if (!templateContent) {
    return []
  }

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

    return extractInjectionUsages(compiled.code, { ignoreJsCommentsAndStrings: true })
  } catch {
    return extractInjectionUsages(stripHtmlComments(templateContent))
  }
}

function stripJsCommentsAndStrings(content: string): string {
  let stripped = ''
  let quote: string | null = null
  let escaped = false

  for (let i = 0; i < content.length; i++) {
    const char = content[i]
    const next = content[i + 1]

    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = null
      } else if (quote === '`' && char === '$' && next === '{') {
        stripped += char + next
        i++
        continue
      }
      stripped += char === '\n' ? '\n' : ' '
      continue
    }

    if (char === '/' && next === '/') {
      stripped += '  '
      i += 2
      while (i < content.length && content[i] !== '\n') {
        stripped += ' '
        i++
      }
      if (i < content.length) {
        stripped += '\n'
      }
      continue
    }

    if (char === '/' && next === '*') {
      stripped += '  '
      i += 2
      while (i < content.length && !(content[i] === '*' && content[i + 1] === '/')) {
        stripped += content[i] === '\n' ? '\n' : ' '
        i++
      }
      if (i < content.length) {
        stripped += '  '
        i++
      }
      continue
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char
      stripped += ' '
      continue
    }

    stripped += char
  }

  return stripped
}

function stripJsComments(content: string): string {
  let stripped = ''
  let quote: string | null = null
  let escaped = false

  for (let i = 0; i < content.length; i++) {
    const char = content[i]
    const next = content[i + 1]

    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = null
      }
      if (quote === '`' && char === '`' && next === '$') {
        stripped += char
        continue
      }
      stripped += char
      continue
    }

    if (char === '/' && next === '/') {
      stripped += '  '
      i += 2
      while (i < content.length && content[i] !== '\n') {
        stripped += ' '
        i++
      }
      if (i < content.length) {
        stripped += '\n'
      }
      continue
    }

    if (char === '/' && next === '*') {
      stripped += '  '
      i += 2
      while (i < content.length && !(content[i] === '*' && content[i + 1] === '/')) {
        stripped += content[i] === '\n' ? '\n' : ' '
        i++
      }
      if (i < content.length) {
        stripped += '  '
        i++
      }
      continue
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char
    }

    stripped += char
  }

  return stripped
}

function isQuotedObjectKeyMatch(content: string, index: number, length: number): boolean {
  const before = content[index - 1]
  const after = content[index + length]
  if ((before !== '"' && before !== "'") || after !== before) {
    return false
  }

  return content.slice(index + length + 1).trimStart().startsWith(':')
}

function normalizeInjectionName(name: string): string {
  return name.trim().replace(/^\$+/, '')
}

function collectProvideObjectKeys(body: string, out: string[]): void {
  for (const property of splitTopLevel(stripJsComments(body), ',')) {
    const trimmed = property.trim()
    if (trimmed.length === 0 || trimmed.startsWith('[')) {
      continue
    }

    const colonIndex = topLevelIndexOf(property, ':')
    const rawKey = colonIndex === -1 ? staticMethodOrShorthandKey(trimmed) : property.slice(0, colonIndex).trim()

    if (rawKey === undefined || rawKey.startsWith('[')) {
      continue
    }

    const staticKey = staticStringLiteralContent(rawKey) ?? rawKey.match(/^[$A-Za-z_][\w$]*$/)?.[0]
    if (staticKey !== undefined) {
      out.push(normalizeInjectionName(staticKey))
    }
  }
}

function staticMethodOrShorthandKey(property: string): string | undefined {
  const shorthand = property.match(/^[$A-Za-z_][\w$]*$/)?.[0]
  if (shorthand) {
    return shorthand
  }

  const method = property.match(/^((?:[$A-Za-z_][\w$]*)|(?:(['"`])[\s\S]*?\2))\s*\(/)?.[1]
  if (method) {
    return method
  }

  return undefined
}

function findMatchingBrace(content: string, openIndex: number): number {
  return findMatchingDelimiter(content, openIndex, '{', '}')
}

function findMatchingParen(content: string, openIndex: number): number {
  return findMatchingDelimiter(content, openIndex, '(', ')')
}

function findMatchingDelimiter(content: string, openIndex: number, open: string, close: string): number {
  if (openIndex < 0 || content[openIndex] !== open) {
    return -1
  }

  let depth = 0
  let quote: string | null = null
  let escaped = false

  for (let i = openIndex; i < content.length; i++) {
    const char = content[i]
    const next = content[i + 1]

    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = null
      }
      continue
    }

    if (char === '/' && next === '/') {
      const newline = content.indexOf('\n', i + 2)
      i = newline === -1 ? content.length : newline
      continue
    }
    if (char === '/' && next === '*') {
      const commentEnd = content.indexOf('*/', i + 2)
      i = commentEnd === -1 ? content.length : commentEnd + 1
      continue
    }
    if (char === '"' || char === "'" || char === '`') {
      quote = char
      continue
    }
    if (char === open) {
      depth++
      continue
    }
    if (char === close) {
      depth--
      if (depth === 0) {
        return i
      }
    }
  }

  return -1
}

function splitTopLevel(content: string, delimiter: string): string[] {
  const parts: string[] = []
  let start = 0
  let parenDepth = 0
  let braceDepth = 0
  let bracketDepth = 0
  let quote: string | null = null
  let escaped = false

  for (let i = 0; i < content.length; i++) {
    const char = content[i]

    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = null
      }
      continue
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char
      continue
    }
    if (char === '(') parenDepth++
    if (char === ')') parenDepth--
    if (char === '{') braceDepth++
    if (char === '}') braceDepth--
    if (char === '[') bracketDepth++
    if (char === ']') bracketDepth--

    if (char === delimiter && parenDepth === 0 && braceDepth === 0 && bracketDepth === 0) {
      parts.push(content.slice(start, i))
      start = i + 1
    }
  }

  parts.push(content.slice(start))
  return parts
}

function topLevelIndexOf(content: string, needle: string): number {
  const beforeNeedle = splitTopLevel(content, needle)[0]
  return beforeNeedle.length === content.length ? -1 : beforeNeedle.length
}

function staticStringLiteralContent(value: string): string | undefined {
  const trimmed = value.trim()
  const match = trimmed.match(/^(['"`])([\s\S]*)\1$/)
  if (!match) {
    return undefined
  }
  if (match[1] === '`' && /\$\{/.test(match[2])) {
    return undefined
  }
  return match[2]
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
    providedInjections: [],
    usedInjections: [],
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
