package analyzer

import "testing"

func TestResolver_Resolve(t *testing.T) {
	r := NewResolver("/repo", nil)

	cases := []struct {
		name     string
		raw      string
		fromFile string
		want     string
		ok       bool
	}{
		{
			name:     "tilde alias",
			raw:      "~/components/Button.vue",
			fromFile: "pages/index.vue",
			want:     "components/Button",
			ok:       true,
		},
		{
			name:     "at alias",
			raw:      "@/components/Button",
			fromFile: "pages/index.vue",
			want:     "components/Button",
			ok:       true,
		},
		{
			name:     "relative sibling",
			raw:      "./utils/helper.ts",
			fromFile: "composables/useAuth.ts",
			want:     "composables/utils/helper",
			ok:       true,
		},
		{
			name:     "relative parent",
			raw:      "../composables/useAuth",
			fromFile: "components/Foo.vue",
			want:     "composables/useAuth",
			ok:       true,
		},
		{
			name:     "imports virtual module",
			raw:      "#imports",
			fromFile: "pages/index.vue",
			ok:       false,
		},
		{
			name:     "bare package",
			raw:      "vue",
			fromFile: "pages/index.vue",
			ok:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := r.Resolve(tc.raw, tc.fromFile)
			if ok != tc.ok {
				t.Fatalf("Resolve(%q, %q) ok = %v, want %v", tc.raw, tc.fromFile, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("Resolve(%q, %q) = %q, want %q", tc.raw, tc.fromFile, got, tc.want)
			}
		})
	}
}

func TestResolver_MatchTemplateRef(t *testing.T) {
	knownNodes := map[string]*Node{
		NodeID("components/Button.vue"): {
			ID:   NodeID("components/Button.vue"),
			Path: "components/Button.vue",
			Name: "Button",
			Type: NodeTypeComponent,
		},
		NodeID("components/base-button.vue"): {
			ID:   NodeID("components/base-button.vue"),
			Path: "components/base-button.vue",
			Name: "BaseButton",
			Type: NodeTypeComponent,
		},
		NodeID("composables/useAuth.ts"): {
			ID:   NodeID("composables/useAuth.ts"),
			Path: "composables/useAuth.ts",
			Name: "useAuth",
			Type: NodeTypeComposable,
		},
	}

	r := NewResolver("/repo", knownNodes)

	cases := []struct {
		tag  string
		want string
		ok   bool
	}{
		{tag: "Button", want: NodeID("components/Button.vue"), ok: true},
		{tag: "base-button", want: NodeID("components/base-button.vue"), ok: true},
		{tag: "LazyButton", want: NodeID("components/Button.vue"), ok: true},
		{tag: "LazyBaseButton", want: NodeID("components/base-button.vue"), ok: true},
		{tag: "MissingWidget", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got, ok := r.MatchTemplateRef(tc.tag)
			if ok != tc.ok {
				t.Fatalf("MatchTemplateRef(%q) ok = %v, want %v", tc.tag, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("MatchTemplateRef(%q) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestResolver_MatchAutoImport(t *testing.T) {
	knownNodes := map[string]*Node{
		NodeID("composables/useAuth.ts"): {
			ID:   NodeID("composables/useAuth.ts"),
			Path: "composables/useAuth.ts",
			Name: "useAuth",
			Type: NodeTypeComposable,
		},
		NodeID("stores/user.ts"): {
			ID:   NodeID("stores/user.ts"),
			Path: "stores/user.ts",
			Name: "user",
			Type: NodeTypeStore,
		},
		NodeID("components/Button.vue"): {
			ID:   NodeID("components/Button.vue"),
			Path: "components/Button.vue",
			Name: "Button",
			Type: NodeTypeComponent,
		},
	}

	r := NewResolver("/repo", knownNodes)

	cases := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "composable", raw: "useAuth", want: NodeID("composables/useAuth.ts"), ok: true},
		{name: "store", raw: "user", want: NodeID("stores/user.ts"), ok: true},
		{name: "component not auto import", raw: "Button", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := r.MatchAutoImport(tc.raw)
			if ok != tc.ok {
				t.Fatalf("MatchAutoImport(%q) ok = %v, want %v", tc.raw, ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("MatchAutoImport(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResolver_DuplicateNamesAreDeterministic(t *testing.T) {
	knownNodes := map[string]*Node{
		NodeID("components/zeta/Button.vue"): {
			ID:   NodeID("components/zeta/Button.vue"),
			Path: "components/zeta/Button.vue",
			Name: "Button",
			Type: NodeTypeComponent,
		},
		NodeID("components/alpha/Button.vue"): {
			ID:   NodeID("components/alpha/Button.vue"),
			Path: "components/alpha/Button.vue",
			Name: "Button",
			Type: NodeTypeComponent,
		},
		NodeID("composables/zeta/useAuth.ts"): {
			ID:   NodeID("composables/zeta/useAuth.ts"),
			Path: "composables/zeta/useAuth.ts",
			Name: "useAuth",
			Type: NodeTypeComposable,
		},
		NodeID("composables/alpha/useAuth.ts"): {
			ID:   NodeID("composables/alpha/useAuth.ts"),
			Path: "composables/alpha/useAuth.ts",
			Name: "useAuth",
			Type: NodeTypeComposable,
		},
	}

	r := NewResolver("/repo", knownNodes)

	t.Run("template ref chooses smallest path", func(t *testing.T) {
		got, ok := r.MatchTemplateRef("Button")
		if !ok {
			t.Fatal("expected template ref match")
		}
		want := NodeID("components/alpha/Button.vue")
		if got != want {
			t.Fatalf("MatchTemplateRef(%q) = %q, want %q", "Button", got, want)
		}
	})

	t.Run("auto import chooses smallest path", func(t *testing.T) {
		got, ok := r.MatchAutoImport("useAuth")
		if !ok {
			t.Fatal("expected auto import match")
		}
		want := NodeID("composables/alpha/useAuth.ts")
		if got != want {
			t.Fatalf("MatchAutoImport(%q) = %q, want %q", "useAuth", got, want)
		}
	})
}
