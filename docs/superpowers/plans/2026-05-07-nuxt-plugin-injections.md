# Nuxt Plugin Injection Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect Nuxt plugin-provided injections, detect `$injection` usage, and add graph edges from consumers to plugin providers.

**Architecture:** Extend the existing parser result contract with `providedInjections` and `usedInjections`, then add a builder pass that resolves those names to plugin nodes using `EdgeInjects`. Static providers create high-confidence edges; dynamic providers create low-confidence possible edges.

**Tech Stack:** Go analyzer and tests, TypeScript Node parser, Vue SFC compiler, esbuild parser bundle.

---

## File Structure

- Modify `internal/parser/node_bridge.go`: add JSON fields to `parser.ParsedFile`.
- Modify `internal/parser/node_bridge_test.go`: prove bridge JSON decoding exposes new fields.
- Modify `internal/analyzer/builder.go`: index plugin providers and add injection edges.
- Modify `internal/analyzer/builder_test.go`: prove static, dynamic, self-loop, and dedupe behavior.
- Modify `node/parser.ts`: extract plugin providers and injection usages.
- Modify `assets/parser.bundle.js`: rebuild from `node/parser.ts` using `make build-parser`.

### Task 1: Parser Contract

**Files:**
- Modify: `internal/parser/node_bridge.go`
- Modify: `internal/parser/node_bridge_test.go`

- [ ] **Step 1: Write the failing bridge decode test**

Add assertions to `TestBridgeParseReturnsResults` in `internal/parser/node_bridge_test.go`:

```go
if !reflect.DeepEqual(got.ProvidedInjections, []string{"api", "*"}) {
	t.Fatalf("ProvidedInjections = %#v, want %#v", got.ProvidedInjections, []string{"api", "*"})
}
if !reflect.DeepEqual(got.UsedInjections, []string{"api"}) {
	t.Fatalf("UsedInjections = %#v, want %#v", got.UsedInjections, []string{"api"})
}
```

Change the fake JSON response in the same test to:

```go
return []byte(`{"results":[{"path":"/a.vue","type":"component","imports":["./foo"],"templateRefs":["MyComp"],"dynamicComponents":["resolveComponent(name)"],"usedAutoImports":["useAuth"],"providedInjections":["api","*"],"usedInjections":["api"],"error":null}]}`), nil
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/parser -run TestBridgeParseReturnsResults -count=1
```

Expected: FAIL because `ProvidedInjections` and `UsedInjections` are not fields on `parser.ParsedFile`.

- [ ] **Step 3: Add parser contract fields**

In `internal/parser/node_bridge.go`, extend `ParsedFile`:

```go
ProvidedInjections []string `json:"providedInjections"`
UsedInjections     []string `json:"usedInjections"`
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/parser -run TestBridgeParseReturnsResults -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/parser/node_bridge.go internal/parser/node_bridge_test.go
git commit -m "feat: extend parser contract for plugin injections"
```

### Task 2: Builder Injection Edges

**Files:**
- Modify: `internal/analyzer/builder.go`
- Modify: `internal/analyzer/builder_test.go`

- [ ] **Step 1: Write failing static provider test**

Add this test to `internal/analyzer/builder_test.go`:

```go
func TestBuilderBuild_PluginInjectionEdgesFromStaticProvide(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	apiAbs := filepath.Join(root, "plugins", "api.ts")
	indexAbs := filepath.Join(root, "pages", "index.vue")

	files := []FileInfo{
		{AbsPath: apiAbs, RelPath: "plugins/api.ts", Type: NodeTypePlugin},
		{AbsPath: indexAbs, RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	parsed := []parser.ParsedFile{
		{Path: apiAbs, Type: "plugin", ProvidedInjections: []string{"api"}},
		{Path: indexAbs, Type: "page", UsedInjections: []string{"api"}},
	}

	graph, buildErrs, err := (Builder{ProjectRoot: root}).buildFromParsed(files, parsed, nil)
	if err != nil {
		t.Fatalf("buildFromParsed() error = %v", err)
	}
	if len(buildErrs) != 0 {
		t.Fatalf("len(buildErrs) = %d, want 0", len(buildErrs))
	}

	assertEdge(t, graph, Edge{
		From:       NodeID("pages/index.vue"),
		To:         NodeID("plugins/api.ts"),
		Kind:       EdgeInjects,
		Confidence: ConfHigh,
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/analyzer -run TestBuilderBuild_PluginInjectionEdgesFromStaticProvide -count=1
```

Expected: FAIL with missing `EdgeInjects`.

- [ ] **Step 3: Write failing dynamic provider test**

Add this test to `internal/analyzer/builder_test.go`:

```go
func TestBuilderBuild_PluginInjectionEdgesFromDynamicProvide(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	dynamicAbs := filepath.Join(root, "plugins", "dynamic.ts")
	indexAbs := filepath.Join(root, "pages", "index.vue")

	files := []FileInfo{
		{AbsPath: dynamicAbs, RelPath: "plugins/dynamic.ts", Type: NodeTypePlugin},
		{AbsPath: indexAbs, RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	parsed := []parser.ParsedFile{
		{Path: dynamicAbs, Type: "plugin", ProvidedInjections: []string{"*"}},
		{Path: indexAbs, Type: "page", UsedInjections: []string{"api"}},
	}

	graph, _, err := (Builder{ProjectRoot: root}).buildFromParsed(files, parsed, nil)
	if err != nil {
		t.Fatalf("buildFromParsed() error = %v", err)
	}

	assertEdge(t, graph, Edge{
		From:       NodeID("pages/index.vue"),
		To:         NodeID("plugins/dynamic.ts"),
		Kind:       EdgeInjects,
		Confidence: ConfLow,
	})
}
```

- [ ] **Step 4: Write failing self-loop and dedupe test**

Add this test to `internal/analyzer/builder_test.go`:

```go
func TestBuilderBuild_PluginInjectionEdgesSkipSelfLoopsAndDeduplicate(t *testing.T) {
	root := filepath.Join(`C:\repo`, `app`)
	apiAbs := filepath.Join(root, "plugins", "api.ts")
	indexAbs := filepath.Join(root, "pages", "index.vue")

	files := []FileInfo{
		{AbsPath: apiAbs, RelPath: "plugins/api.ts", Type: NodeTypePlugin},
		{AbsPath: indexAbs, RelPath: "pages/index.vue", Type: NodeTypePage},
	}

	parsed := []parser.ParsedFile{
		{Path: apiAbs, Type: "plugin", ProvidedInjections: []string{"api"}, UsedInjections: []string{"api"}},
		{Path: indexAbs, Type: "page", UsedInjections: []string{"api", "api"}},
	}

	graph, _, err := (Builder{ProjectRoot: root}).buildFromParsed(files, parsed, nil)
	if err != nil {
		t.Fatalf("buildFromParsed() error = %v", err)
	}

	apiID := NodeID("plugins/api.ts")
	indexID := NodeID("pages/index.vue")
	count := 0
	for _, e := range graph.Edges {
		if e.From == apiID && e.To == apiID {
			t.Fatalf("unexpected self-loop edge %#v", e)
		}
		if e.From == indexID && e.To == apiID && e.Kind == EdgeInjects {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 injection edge from index to api plugin, got %d", count)
	}
}
```

- [ ] **Step 5: Run builder tests to verify failures**

Run:

```bash
go test ./internal/analyzer -run 'TestBuilderBuild_PluginInjectionEdges' -count=1
```

Expected: FAIL because injection edge building is not implemented.

- [ ] **Step 6: Implement provider indexing and edge creation**

In `internal/analyzer/builder.go`, add constants near the top:

```go
const dynamicInjectionProvider = "*"
```

After the auto-import edge pass, add:

```go
	pluginProviders := make(map[string][]string)
	dynamicPluginProviders := make([]string, 0)

	for _, parsedFile := range parsed {
		info, ok := infoByAbs[parsedFile.Path]
		if !ok || info.Type != NodeTypePlugin {
			continue
		}
		pluginID := NodeID(info.RelPath)
		for _, provided := range parsedFile.ProvidedInjections {
			if provided == "" {
				continue
			}
			if provided == dynamicInjectionProvider {
				dynamicPluginProviders = append(dynamicPluginProviders, pluginID)
				continue
			}
			pluginProviders[provided] = append(pluginProviders[provided], pluginID)
		}
	}

	for _, parsedFile := range parsed {
		info, ok := infoByAbs[parsedFile.Path]
		if !ok {
			continue
		}
		fromID := NodeID(info.RelPath)
		for _, used := range parsedFile.UsedInjections {
			for _, toID := range pluginProviders[used] {
				if fromID == toID {
					continue
				}
				key := fromID + ":" + toID
				if _, exists := existingEdges[key]; exists {
					continue
				}
				existingEdges[key] = struct{}{}
				graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeInjects, Confidence: ConfHigh})
			}
			for _, toID := range dynamicPluginProviders {
				if fromID == toID {
					continue
				}
				key := fromID + ":" + toID
				if _, exists := existingEdges[key]; exists {
					continue
				}
				existingEdges[key] = struct{}{}
				graph.AddEdge(Edge{From: fromID, To: toID, Kind: EdgeInjects, Confidence: ConfLow})
			}
		}
	}
```

- [ ] **Step 7: Run builder tests to verify pass**

Run:

```bash
go test ./internal/analyzer -run 'TestBuilderBuild_PluginInjectionEdges|TestBuilderBuild_AutoImport' -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/analyzer/builder.go internal/analyzer/builder_test.go
git commit -m "feat: connect nuxt plugin injections in graph"
```

### Task 3: Node Parser Extraction

**Files:**
- Modify: `node/parser.ts`

- [ ] **Step 1: Add parser helper tests as temporary command cases**

There is no dedicated TypeScript test runner in the repo. Add behavior by exercising the parser through fixture files in Task 4. For this task, the failing signal is the full bridge/parser integration test added in Task 4 before implementation.

- [ ] **Step 2: Extend TypeScript interfaces and empty result**

In `node/parser.ts`, extend `ParsedFile`:

```ts
  providedInjections: string[]
  usedInjections: string[]
```

Update `empty()` to return empty arrays:

```ts
    providedInjections: [],
    usedInjections: [],
```

- [ ] **Step 3: Add provider and usage extraction calls**

In `parseVue`, after `extractImports(scriptContent, imports)`, add:

```ts
    const fullContent = [scriptContent, descriptor.template?.content ?? '']
      .filter((part) => part.length > 0)
      .join('\n')
    const providedInjections = extractProvidedInjections(scriptContent)
    const usedInjections = extractInjectionUsages(fullContent)
```

Return those values:

```ts
      providedInjections,
      usedInjections,
```

In `parseTS`, add:

```ts
  const providedInjections = extractProvidedInjections(content)
  const usedInjections = extractInjectionUsages(content)
```

Return those values:

```ts
    providedInjections,
    usedInjections,
```

- [ ] **Step 4: Add extraction helpers**

Add these helpers near `extractAutoImportUsages`:

```ts
const dynamicInjectionProvider = '*'

function extractProvidedInjections(content: string): string[] {
  const found: string[] = []

  const provideObjectRe = /\bprovide\s*:\s*\{([\s\S]*?)\}/g
  let objectMatch: RegExpExecArray | null
  while ((objectMatch = provideObjectRe.exec(content)) !== null) {
    const body = objectMatch[1] ?? ''
    const keyRe = /(?:^|[,;\s])(?:['"]([^'"]+)['"]|([A-Za-z_$][\w$]*))\s*:/g
    let keyMatch: RegExpExecArray | null
    while ((keyMatch = keyRe.exec(body)) !== null) {
      const key = keyMatch[1] ?? keyMatch[2]
      if (key) {
        found.push(normalizeInjectionName(key))
      }
    }
  }

  const provideCallRe = /\b[A-Za-z_$][\w$]*\.provide\s*\(\s*([^,\n)]+)/g
  let callMatch: RegExpExecArray | null
  while ((callMatch = provideCallRe.exec(content)) !== null) {
    const expression = callMatch[1]?.trim() ?? ''
    const staticKey = expression.match(/^['"`]([^'"`]+)['"`]$/)
    if (staticKey?.[1]) {
      found.push(normalizeInjectionName(staticKey[1]))
    } else if (expression.length > 0) {
      found.push(dynamicInjectionProvider)
    }
  }

  return dedupe(found)
}

function extractInjectionUsages(content: string): string[] {
  const found: string[] = []
  const usageRe = /(?<![\w$])\$([A-Za-z_][\w$]*)\b/g
  let match: RegExpExecArray | null

  while ((match = usageRe.exec(content)) !== null) {
    const name = match[1]
    if (name) {
      found.push(normalizeInjectionName(name))
    }
  }

  return dedupe(found)
}

function normalizeInjectionName(name: string): string {
  return name.trim().replace(/^\$/, '')
}
```

- [ ] **Step 5: Run Go tests to catch TypeScript contract fallout**

Run:

```bash
go test ./internal/parser ./internal/analyzer -count=1
```

Expected: PASS after Task 1 and Task 2 are complete.

- [ ] **Step 6: Commit**

```bash
git add node/parser.ts
git commit -m "feat: detect nuxt plugin injection names"
```

### Task 4: Parser Bundle Integration

**Files:**
- Modify: `assets/parser.bundle.js`

- [ ] **Step 1: Create temporary fixture files outside the repo**

Run:

```bash
tmpdir="$(mktemp -d)"
mkdir -p "$tmpdir/plugins" "$tmpdir/pages"
cat > "$tmpdir/plugins/api.ts" <<'EOF'
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.provide('api', { get: () => null })
  const dynamicName = 'tracking'
  nuxtApp.provide(dynamicName, {})
  return {
    provide: {
      auth: { user: null },
    },
  }
})
EOF
cat > "$tmpdir/pages/index.vue" <<'EOF'
<template>
  <div>{{ $api }}</div>
</template>
<script setup lang="ts">
const { $auth } = useNuxtApp()
useNuxtApp().$api.get('/users')
</script>
EOF
printf '%s\n' "$tmpdir"
```

Expected: prints a temporary directory path.

- [ ] **Step 2: Run parser before rebuilding to verify missing fields or empty extraction**

Use the printed temp path as `$tmpdir`, then run:

```bash
node node/parser.ts <<EOF
{"files":["$tmpdir/plugins/api.ts","$tmpdir/pages/index.vue"]}
EOF
```

Expected before implementation or before bundle rebuild: output does not include the desired extracted `providedInjections:["api","*","auth"]` and `usedInjections:["api","auth"]`.

- [ ] **Step 3: Rebuild parser bundle**

Run:

```bash
make build-parser
```

Expected: `assets/parser.bundle.js` updates.

- [ ] **Step 4: Run parser fixture against bundled parser**

Run:

```bash
node assets/parser.bundle.js <<EOF
{"files":["$tmpdir/plugins/api.ts","$tmpdir/pages/index.vue"]}
EOF
```

Expected: plugin result includes `providedInjections` containing `api`, `auth`, and `*`; page result includes `usedInjections` containing `api` and `auth`.

- [ ] **Step 5: Commit**

```bash
git add assets/parser.bundle.js
git commit -m "build: update embedded parser bundle"
```

### Task 5: Full Verification

**Files:**
- No source changes expected.

- [ ] **Step 1: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run build**

Run:

```bash
make build
```

Expected: PASS and writes `bin/impact-map`.

- [ ] **Step 3: Inspect final git status**

Run:

```bash
git status --short
```

Expected: no unintended uncommitted source changes. `bin/` may be ignored by git.
