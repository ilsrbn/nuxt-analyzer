# Potential Bugs Audit

Sorted by business impact category, then priority.

---

## Category: "Will generate wrong result"

Incorrect impact analysis, false dependencies, or missing edges. User sees bad output.

| # | File | Issue | Priority |
|---|------|-------|----------|
| 11 | `internal/analyzer/scanner.go:82-89` | Scanner drops `.js`, `.tsx`, `.jsx`, `.mjs`, `.cjs` files entirely from graph | **Crit** |
| 15 | `internal/analyzer/scanner.go:24-27` | `filepath.WalkDir` aborts full scan on first unreadable dir/permission error | **Crit** |
| 4 | `internal/git/diff.go:94-105` | Generated-path filter uses `HasPrefix`, misses nested `.nuxt/` and `node_modules/` | **Medium** |
| 5 | `internal/analyzer/graph.go:153-170` | `AddEdge` allows duplicate `(From, To, Kind)` edges | **Medium** |
| 7 | `node/parser.ts:148-161` | `extractAutoImportUsages` regex runs on raw source: matches comments/strings | **Medium** |
| 8 | `node/parser.ts:561-567` | `extractImports` regex runs on raw source: captures imports inside comments/strings | **Medium** |
| 13 | `internal/analyzer/graph.go:93` | `InferType` checks `base == "app.vue"` before path segments — nested `pages/app.vue` misclassified as component | **Medium** |
| 17 | `internal/analyzer/builder.go:235-236` | `relPathToRoute` strips at first `pages/` occurrence — `pages/admin/pages/settings.vue` yields wrong route | **Medium** |
| 18 | `internal/analyzer/builder.go:34` | `LoadAutoImportMap` error silently discarded — malformed `.nuxt/imports.d.ts` causes missing edges without warning | **Medium** |
| 6 | `internal/analyzer/builder.go:113-116` | One self-loop added per dynamic component item — inflates counts | **Low** |
| 9 | `node/parser.ts:169-171` | Provide-object brace matcher finds first `{` after match start, can target nested call | **Low** |
| 12 | `node/parser.ts:661` | `is` attribute regex matches `is=` on **any** HTML tag, not just `<component>` | **Low** |

---

## Category: "Memory / Resource leak"

Stability issues. Will crash, hang, or exhaust resources in CI / long runs.

| # | File | Issue | Priority |
|---|------|-------|----------|
| 1 | `internal/parser/ts_parser.go:30-33` | `tree.Close()` skipped when `tree.RootNode()` is nil on malformed source | **Crit** |
| 2 | `internal/parser/node_bridge.go:28-49` + `hybrid_bridge.go:15-25` | Temp dir created before bundle write; on write/init failure, `Cleanup()` never finds it | **Crit** |
| 10 | `internal/impact/engine.go:83-86` | BFS `queue = queue[1:]` keeps backing array referenced — O(N²) memory churn on large graphs | **Medium** |

---

## Category: "Dead logic / Misleading UX"

Wasted cycles, non-working features, or confusing behavior. No crash, but trust erosion.

| # | File | Issue | Priority |
|---|------|-------|----------|
| 3 | `cmd/impact-map/main.go:128-136` | `reportCache` hardcoded to `cache.NoopCache{}`; `--no-cache` flag has zero effect | **Medium** |
| 16 | `cmd/impact-map/upgrade.go:197-212` | `os.Rename` cannot replace running executable on Windows — upgrade command always fails there | **Low** |
| 14 | `internal/nuxt/autoimports.go:28` | Regex recompiled inside `LoadAutoImportMap` on every run | **Low** |

---

## Priority Summary

| Priority | Count | Key issues |
|----------|-------|------------|
| **Crit** | 5 | Memory leaks, temp dir leaks, scan aborts, missing `.js`/`.tsx`/`.jsx` support |
| **Medium** | 10 | Wrong graph edges, wrong routes, wrong counts, dead cache, BFS memory churn |
| **Low** | 5 | Brace matcher edge case, `is` attr regex, Windows upgrade, duplicate self-loops, regex recompile |
