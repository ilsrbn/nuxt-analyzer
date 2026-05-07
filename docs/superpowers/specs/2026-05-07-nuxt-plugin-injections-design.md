# Nuxt Plugin Injection Detection Design

## Goal

Detect Nuxt plugin-provided injections and connect their usage in pages, components, layouts, composables, stores, middleware, and utilities to the plugin file in the dependency graph.

## Current System

The analyzer scans Nuxt source files, parses Vue and TypeScript through `node/parser.ts`, and builds a graph in `internal/analyzer/builder.go`. Parser output already includes imports, template component references, dynamic component markers, and auto-import usages. The builder resolves those into graph edges, including `EdgeAutoImportUses`. `EdgeInjects` already exists as an edge kind but is not populated.

## Provider Detection

The parser will add `providedInjections` to each parsed file.

Static provider forms:

```ts
export default defineNuxtPlugin(() => {
  return {
    provide: {
      api: createApiClient(),
    },
  }
})
```

```ts
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.provide('api', createApiClient())
})
```

Static keys are emitted as their Nuxt injection name without `$`, for example `api`.

Dynamic provider forms:

```ts
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.provide(name, value)
})
```

When the first `provide()` argument cannot be resolved as a string literal, the parser emits a wildcard marker. The builder treats that plugin as a possible provider for any detected injection usage and creates low-confidence `EdgeInjects` edges.

## Consumer Detection

The parser will add `usedInjections` to each parsed file.

Supported usage forms:

```ts
const { $api } = useNuxtApp()
const nuxtApp = useNuxtApp()
nuxtApp.$api.get('/users')
useNuxtApp().$api.get('/users')
```

Vue template and script content will also be scanned for standalone `$api`-style identifiers. Emitted usage names omit `$`, for example `api`.

## Graph Behavior

The builder will index plugin nodes by `providedInjections`. For each parsed file usage:

- If one or more plugins statically provide the same injection key, add `EdgeInjects` from the consumer node to each provider plugin with high confidence.
- If a plugin has a dynamic provide wildcard, add `EdgeInjects` from the consumer node to that plugin with low confidence.
- Skip self-loops when a plugin references its own provided key.
- Deduplicate injection edges against existing edges between the same source and target.

This makes plugin changes flow through the existing reverse dependency traversal so affected pages and components appear in impact results.

## Parser Contract

`parser.ParsedFile` will gain:

```go
ProvidedInjections []string `json:"providedInjections"`
UsedInjections     []string `json:"usedInjections"`
```

Existing JSON inputs and outputs remain backward-compatible because missing arrays decode as nil.

## Testing

Tests will cover:

- Builder creates high-confidence `EdgeInjects` from static provider keys to consumer usages.
- Builder creates low-confidence `EdgeInjects` for dynamic provider wildcards.
- Builder skips plugin self-loops and avoids duplicate consumer-to-provider edges.
- Parser bridge decodes and exposes the new JSON fields.
- Node parser detects return-object providers, `nuxtApp.provide()` providers, dynamic providers, and common `$api` usages.

## Out Of Scope

The analyzer will not execute code or attempt full TypeScript data-flow analysis. Dynamic keys are intentionally represented as low-confidence possible edges.
