# PR #3 Unresolved Review Comments Plan

Source: https://github.com/ilsrbn/nuxt-analyzer/pull/3
Bot: chatgpt-codex-connector

---

## P1 — Route scanned JS extensions to the TS parser

**File:** `internal/parser/hybrid_bridge.go`
**Issue:** Scanner now admits `.js`, `.mjs`, `.cjs`, `.tsx`, `.jsx`, but `HybridBridge.Parse` only routes `.vue`, `.ts`, `.tsx`, `.jsx`. Files with new extensions reach the bridge but never get parsed, so their imports/provides/usages are silently missing from the impact graph.
**Action:** Add `.js`, `.mjs`, `.cjs` to the `tsFiles` branch in the extension switch. Ensure `.tsx`/`.jsx` still use a TSX-capable grammar path if needed.

---

## P1 — Regenerate the embedded parser bundle

**File:** `node/parser.ts` / `assets/parser.bundle.js`
**Issue:** `node/parser.ts` was updated (auto-import detection now uses `stripJsCommentsAndStrings`), but `assets/parser.bundle.js` was not regenerated. Users building via `go install` use the checked-in bundle and will not see the new filtering logic.
**Action:** Run `make build-parser` (or equivalent) and commit the regenerated `assets/parser.bundle.js`.

---

## P1 — Record temp dir before writing the bundle

**File:** `internal/parser/node_bridge.go`
**Issue:** `newNodeBridge` returns a cleanup function that removes `b.tempDir`, but `b.tempDir` is assigned only after `os.WriteFile` succeeds. If the write fails after `MkdirTemp`, cleanup on the error path leaks the temp directory.
**Action:** Assign `b.tempDir` immediately after `MkdirTemp`, before writing the bundle file.

---

## P2 — Match actual pages segment when building routes

**File:** `internal/analyzer/builder.go` — `relPathToRoute`
**Issue:** Using raw substring `Index(route, "pages/")` can match a parent directory whose name ends in `pages` (e.g., `app/my-pages/pages/foo.vue` strips at `my-pages/` and reports `/pages/foo` instead of `/foo`).
**Action:** Split on path segments and require a full segment match for `pages`, or use boundary-aware logic.

---

## P2 — Preserve explicit import extensions when resolving

**File:** `internal/analyzer/builder.go` — `resolveToNodeID`
**Issue:** When `Resolver.Resolve` strips a known extension, an explicit import like `~/utils/format.js` is resolved as `utils/format`. If both `utils/format.ts` and `utils/format.js` exist, the candidate list checks `.ts` before `.js` and may wire the edge to the wrong file despite the explicit `.js` specifier.
**Action:** Preserve the original extension when the import explicitly includes one; prioritize the exact extension in candidate resolution.

---

## P2 — Return root walk errors instead of hiding them

**File:** `internal/analyzer/scanner.go`
**Issue:** When `WalkDir` is invoked with a root-level error (e.g., `--project-root` is missing), `d` is nil. The current branch returns `nil` for `os.IsNotExist`, so `Scan` succeeds with an empty file list instead of reporting the invalid path.
**Action:** Keep the `d != nil` guard, but only skip permission/not-exist errors for entries below the root. Return root-level errors so callers do not silently produce empty analysis.

---

## P2 — Resolve imports to the new source extensions

**File:** `internal/analyzer/scanner.go` / `internal/analyzer/builder.go`
**Issue:** `resolveToNodeID` still only tries `.vue`, `.ts`, and index variants. A component importing `~/utils/format` (or `~/utils/format.js`) is normalized to `utils/format`, but the graph node is `utils/format.js`, so the dependency edge is missing for every newly included JS/TSX/MJS/CJS source file.
**Action:** Extend `resolveToNodeID` candidate list to include `.js`, `.mjs`, `.cjs`, `.tsx`, `.jsx` and their `/index.*` variants.

---

## P2 — Classify page files before nested route folder names

**File:** `internal/analyzer/graph.go` — `InferType`
**Issue:** Walking path segments from leaf to root means arbitrary route folders override the actual `pages` root. In a valid Nuxt route like `pages/components/Button.vue` or `pages/stores/[id].vue`, the function returns `component`/`store` instead of `page`.
**Action:** Walk segments root-to-leaf so `pages` is found before nested segments that happen to match special directory names.

---

## P2 — Keep template-literal expressions searchable

**File:** `node/parser.ts` — `stripJsCommentsAndStrings`
**Issue:** The function blanks out executable `${...}` expressions inside template literals. A real usage like ``const label = `${useAuth().user.name}` `` is no longer seen, so the auto-import edge is missing even though the identifier is executed code.
**Action:** Preserve interpolation bodies inside backtick strings when stripping for auto-import detection.

---

## P2 — Parse CommonJS dependencies before scanning cjs files

**File:** `internal/parser/hybrid_bridge.go`
**Issue:** Routing `.cjs` files through the TypeScript parser only extracts ES `import` statements and dynamic `import()` calls. A CommonJS plugin/store using `require('./api')` is added as a graph node but its dependency edge is silently missing.
**Action:** Either handle `require()` in the TS parser path, or avoid advertising `.cjs` as analyzable source until CommonJS support is added.

---

## P2 — Keep auto-import loading best-effort

**File:** `internal/analyzer/builder.go`
**Issue:** The comment says loading `.nuxt/imports.d.ts` is best-effort, but the code now aborts the entire graph build on any read/scanner error. In projects where the generated `.nuxt` cache is unreadable or truncated, analysis fails instead of producing a graph with auto-import edges omitted.
**Action:** Log the error and continue building the graph without the auto-import map, preserving the original best-effort behavior.
