package analyzer

import "testing"

func TestNodeID_Stable(t *testing.T) {
	id1 := NodeID("components/Button.vue")
	id2 := NodeID("components/Button.vue")
	if id1 != id2 {
		t.Fatalf("NodeID not stable: %s != %s", id1, id2)
	}
	if len(id1) != 12 {
		t.Fatalf("NodeID wrong length: %d", len(id1))
	}
}

func TestNodeID_Unique(t *testing.T) {
	if NodeID("components/Button.vue") == NodeID("components/Modal.vue") {
		t.Fatal("NodeID collision")
	}
}

func TestInferType(t *testing.T) {
	cases := []struct {
		path string
		want NodeType
	}{
		{"components/Button.vue", NodeTypeComponent},
		{"components/ui/Card.vue", NodeTypeComponent},
		{"pages/index.vue", NodeTypePage},
		{"pages/auth/login.vue", NodeTypePage},
		{"layouts/default.vue", NodeTypeLayout},
		{"composables/useAuth.ts", NodeTypeComposable},
		{"stores/user.ts", NodeTypeStore},
		{"plugins/api.ts", NodeTypePlugin},
		{"middleware/auth.ts", NodeTypeMiddleware},
		{"utils/format.ts", NodeTypeUtil},
		{"shared/helpers.ts", NodeTypeUtil},
		{"app.vue", NodeTypeComponent},
		{"other/foo.ts", NodeTypeUnknown},
		{"nuxt.config.ts", NodeTypeUnknown},
		{"components-lib/foo.vue", NodeTypeUnknown},
		{"pages-old/index.vue", NodeTypeUnknown},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := InferType(c.path)
			if got != c.want {
				t.Errorf("InferType(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

func TestInferType_WindowsSeparators(t *testing.T) {
	cases := []struct {
		path string
		want NodeType
	}{
		{"components\\Button.vue", NodeTypeComponent},
		{"pages\\auth\\login.vue", NodeTypePage},
		{"components-lib\\foo.vue", NodeTypeUnknown},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := InferType(c.path)
			if got != c.want {
				t.Errorf("InferType(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}

func TestGraph_AddEdge(t *testing.T) {
	g := NewGraph()
	n1 := &Node{ID: "aaa", Path: "a.vue", Name: "A", Type: NodeTypeComponent, Tags: []string{}}
	n2 := &Node{ID: "bbb", Path: "b.vue", Name: "B", Type: NodeTypeComponent, Tags: []string{}}
	g.AddNode(n1)
	g.AddNode(n2)
	g.AddEdge(Edge{From: "aaa", To: "bbb", Kind: EdgeTemplateUses, Confidence: ConfHigh})

	if len(g.ForwardDeps["aaa"]) != 1 || g.ForwardDeps["aaa"][0] != "bbb" {
		t.Fatal("ForwardDeps wrong")
	}
	if len(g.ReverseDeps["bbb"]) != 1 || g.ReverseDeps["bbb"][0] != "aaa" {
		t.Fatal("ReverseDeps wrong")
	}
	if len(g.Edges) != 1 {
		t.Fatal("Edges count wrong")
	}
}

func TestNodeName_WindowsSeparators(t *testing.T) {
	if got := NodeName(`components\Button.vue`); got != "Button" {
		t.Fatalf("NodeName(%q) = %q, want %q", `components\Button.vue`, got, "Button")
	}
}

func TestNodeName(t *testing.T) {
	cases := []struct{ path, want string }{
		{"components/Button.vue", "Button"},
		{"composables/useAuth.ts", "useAuth"},
		{"stores/user.ts", "user"},
	}
	for _, c := range cases {
		if got := NodeName(c.path); got != c.want {
			t.Errorf("NodeName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
