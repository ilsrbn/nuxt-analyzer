# PR #3 Review Fix Tasks

Generated from automated Codex review of PR #3.
Each task references the original review comment and file location.

## P1 — Critical

### 1. Route scanned JS files to parser
- **File**: `internal/parser/hybrid_bridge.go` (~line 52)
- **Issue**: `scanner.go` now scans `.js`, `.mjs`, `.cjs`, but `HybridBridge.Parse` switch only routes `.vue`, `.ts`, `.tsx`, `.jsx`. JS files are added to graph but never parsed, so imports/provides/usages are silently missing from impact results.
- **Fix**: Add `.js`, `.mjs`, `.cjs` cases to the `tsFiles` routing in the switch.

### 2. Resolve imports to new source extensions
- **File**: `internal/analyzer/builder.go` (~line 215–232)
- **Issue**: `resolveToNodeID` only tries `.vue`, `.ts`, `/index.vue`, `/index.ts`. If a component imports `~/utils/format` and the graph node is `utils/format.js`, the dependency edge is missing.
- **Fix**: Expand `candidates` to include `.js`, `.mjs`, `.cjs`, `.tsx`, `.jsx` and their `/index.*` variants.

### 3. Regenerate embedded parser bundle
- **File**: `assets/parser.bundle.js` (generated from `node/parser.ts`)
- **Issue**: `node/parser.ts` was updated but the embedded bundle wasn't rebuilt. Direct Go installs via `go install` use the checked-in bundle, so fixes in `node/parser.ts` are absent.
- **Fix**: Run `make build-parser` (or equivalent) and commit the regenerated `assets/parser.bundle.js`.

### 4. Record temp dir before writing bundle
- **File**: `internal/parser/node_bridge.go` (~line 33–51)
- **Issue**: `b.tempDir` is assigned only after `os.WriteFile` succeeds. If write fails, `cleanup()` skips removal because `b.tempDir` is still empty, leaking the temp directory.
- **Fix**: Assign `b.tempDir = dir` immediately after `os.MkdirTemp` succeeds, before `os.WriteFile`.

---

## P2 — Medium

### 5. Preserve nested pages route segments
- **File**: `internal/analyzer/builder.go` (~line 234–239)
- **Issue**: `relPathToRoute` uses `strings.LastIndex(route, "pages/")`. For a file like `pages/admin/pages/settings.vue`, it extracts `/settings` instead of `/admin/pages/settings` because the innermost `pages/` is treated as the route root.
- **Fix**: Locate the **first** `pages/` segment from the root, not the last. Or track whether a segment is actually a route directory vs. a nested folder named `pages`.

### 6. Avoid dereferencing nil DirEntry on root walk errors
- **File**: `internal/analyzer/scanner.go` (~line 26–34)
- **Issue**: When `filepath.WalkDir` invokes the callback with a root-level error, `d` can be `nil`. The code calls `d.IsDir()` after checking `os.IsNotExist(walkErr)`, which panics.
- **Fix**: Guard `d` with a nil check before calling `d.IsDir()`.

### 7. Classify page files before nested route folder names
- **File**: `internal/analyzer/graph.go` (~line 89–118)
- **Issue**: `InferType` walks segments from leaf to root. A file like `pages/components/Button.vue` hits `components` first and returns `NodeTypeComponent` instead of `NodeTypePage`. Same for `pages/stores/[id].vue`.
- **Fix**: Prioritize `pages` detection over other directory names, or walk from root to leaf so the closest containing directory wins.

### 8. Keep template-literal expressions searchable
- **File**: `node/parser.ts` (~line 148–161)
- **Issue**: `extractAutoImportUsages` feeds content through `stripJsCommentsAndStrings`, which blanks out `${...}` expressions inside template literals. A real usage like `` `${useAuth().user.name}` `` is no longer seen, missing auto-import edges.
- **Fix**: Update `stripJsCommentsAndStrings` to preserve interpolation bodies inside template literals, or use a different stripping strategy for auto-import detection.

### 9. Keep auto-import loading best-effort
- **File**: `internal/analyzer/builder.go` (~line 33–37)
- **Issue**: The comment says `.nuxt/imports.d.ts` is "best-effort, ignored on error", but the code now returns the error and aborts the entire graph build. In projects where `.nuxt` cache is transiently bad, analysis fails instead of gracefully omitting auto-import edges.
- **Fix**: Revert to ignoring the error (log it) and continuing with an empty `autoImportMap`, matching the documented behavior.

### 10. Preserve template-literal interpolation bodies (dead-code fix)
- **File**: `node/parser.ts` (~line 322–325)
- **Issue**: The branch `if (quote && char === '`' && next === '$')` outside the `if (quote)` block never runs because `quote` is `null` there. The intended fix for template-literal interpolation is not actually active.
- **Fix**: Move the interpolation-preservation logic inside the `if (quote)` block (where `quote` may be `` ` ``) so `${...}` bodies are kept searchable.

---

## Summary

| # | Priority | File | Task |
|---|----------|------|------|
| 1 | P1 | `internal/parser/hybrid_bridge.go` | Route `.js`/`.mjs`/`.cjs` files to parser |
| 2 | P1 | `internal/analyzer/builder.go` | Add JS/TSX/MJS/CJS candidates to `resolveToNodeID` |
| 3 | P1 | `assets/parser.bundle.js` | Regenerate embedded bundle from `node/parser.ts` |
| 4 | P1 | `internal/parser/node_bridge.go` | Assign `tempDir` before `WriteFile` to prevent leak |
| 5 | P2 | `internal/analyzer/builder.go` | Use root-first `pages/` index for route extraction |
| 6 | P2 | `internal/analyzer/scanner.go` | Guard nil `d` before `d.IsDir()` in walk callback |
| 7 | P2 | `internal/analyzer/graph.go` | Fix `InferType` to prioritize `pages` over inner dirs |
| 8 | P2 | `node/parser.ts` | Preserve `${...}` bodies when stripping for auto-imports |
| 9 | P2 | `internal/analyzer/builder.go` | Make auto-import loading best-effort again |
| 10 | P2 | `node/parser.ts` | Fix dead interpolation-preservation branch in stripper |
