package parser

import "testing"

func TestTypeScriptParserReturnsEmptyMetadataForEmptyFile(t *testing.T) {
	p := NewTypeScriptParser()
	got := p.ParseSource("/project/utils/empty.ts", []byte(""), nil)

	if got.Path != "/project/utils/empty.ts" {
		t.Fatalf("Path = %q, want %q", got.Path, "/project/utils/empty.ts")
	}
	if got.Type != string(ParsedTypeUtil) {
		t.Fatalf("Type = %q, want %q", got.Type, ParsedTypeUtil)
	}
	if got.Error != nil {
		t.Fatalf("Error = %v, want nil", *got.Error)
	}
}

func TestTypeScriptParserExtractsImports(t *testing.T) {
	source := []byte(`
import foo from './foo'
import type { Bar } from '@/types'
import './setup'
const lazy = import('../lazy')
const dynamic = import(name)
`)

	p := NewTypeScriptParser()
	got := p.ParseSource("/project/composables/useThing.ts", source, nil)

	want := []string{"./foo", "@/types", "./setup", "../lazy"}
	if !equalStringSlices(got.Imports, want) {
		t.Fatalf("Imports = %#v, want %#v", got.Imports, want)
	}
	if got.Error != nil {
		t.Fatalf("Error = %v, want nil", *got.Error)
	}
}

func TestTypeScriptParserExtractsAutoImportUsages(t *testing.T) {
	source := []byte(`
const auth = useAuth()
const routeName = router.currentRoute.value.name
object.useThing()
const notUseAuth = true
`)

	p := NewTypeScriptParser()
	got := p.ParseSource("/project/pages/index.ts", source, []string{"useAuth", "useThing", "router"})

	want := []string{"useAuth", "router"}
	if !equalStringSlices(got.UsedAutoImports, want) {
		t.Fatalf("UsedAutoImports = %#v, want %#v", got.UsedAutoImports, want)
	}
}

func TestTypeScriptParserExtractsInjectionUsage(t *testing.T) {
	source := []byte(`
const data = $api('/users')
const viaApp = useNuxtApp().$client('/users')
const { $tracker } = useNuxtApp()
const { $analytics: analytics } = useNuxtApp()
const ignored = $route.params.id
const alsoIgnored = { "$api": true }
const alsoIgnoredPair = { $client: true }
`)

	p := NewTypeScriptParser()
	got := p.ParseSource("/project/composables/useUsers.ts", source, nil)

	want := []string{"api", "client", "tracker", "analytics"}
	if !equalStringSlices(got.UsedInjections, want) {
		t.Fatalf("UsedInjections = %#v, want %#v", got.UsedInjections, want)
	}
}

func TestTypeScriptParserExtractsNuxtAppProvideCalls(t *testing.T) {
	source := []byte(`
export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.provide('api', {})
  useNuxtApp().provide(name, {})
})
`)

	p := NewTypeScriptParser()
	got := p.ParseSource("/project/plugins/api.ts", source, nil)

	want := []string{"api", DynamicInjectionProvider}
	if !equalStringSlices(got.ProvidedInjections, want) {
		t.Fatalf("ProvidedInjections = %#v, want %#v", got.ProvidedInjections, want)
	}
}

func TestTypeScriptParserExtractsReturnedProvideObject(t *testing.T) {
	source := []byte(`
export default defineNuxtPlugin(() => {
  return {
    provide: {
      api: {},
      $client: {},
      "tracker": {},
      [dynamicName]: {},
    }
  }
})
`)

	p := NewTypeScriptParser()
	got := p.ParseSource("/project/plugins/api.ts", source, nil)

	want := []string{"api", "client", "tracker"}
	if !equalStringSlices(got.ProvidedInjections, want) {
		t.Fatalf("ProvidedInjections = %#v, want %#v", got.ProvidedInjections, want)
	}
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
