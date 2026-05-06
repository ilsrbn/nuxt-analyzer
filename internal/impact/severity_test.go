package impact

import (
	"reflect"
	"testing"

	"github.com/ilsrbn/nuxt-analyzer/internal/analyzer"
)

func TestScore_HighBandRules(t *testing.T) {
	t.Run("plugin", func(t *testing.T) {
		g := analyzer.NewGraph()
		n := &analyzer.Node{ID: "p1", Path: "plugins/api.ts", Name: "api", Type: analyzer.NodeTypePlugin}
		g.AddNode(n)

		result := &Result{ChangedNodes: []string{"p1"}}

		sr := Score(g, result)

		if len(sr.High) != 1 {
			t.Fatalf("High = %v, want 1 entry", sr.High)
		}
		if sr.High[0].NodeID != "p1" {
			t.Fatalf("High[0].NodeID = %q, want p1", sr.High[0].NodeID)
		}
		if !containsReason(sr.High[0].Reasons, "is-plugin") {
			t.Fatalf("High[0].Reasons = %v, want is-plugin", sr.High[0].Reasons)
		}
	})

	t.Run("ten-plus-dependents", func(t *testing.T) {
		g := analyzer.NewGraph()
		n := &analyzer.Node{ID: "c1", Path: "composables/useFoo.ts", Name: "useFoo", Type: analyzer.NodeTypeComposable, DependentCount: 10}
		g.AddNode(n)

		result := &Result{AffectedNodes: []string{"c1"}}

		sr := Score(g, result)

		if len(sr.High) != 1 {
			t.Fatalf("High = %v, want 1 entry", sr.High)
		}
		if !containsReason(sr.High[0].Reasons, "10+-dependents") {
			t.Fatalf("High[0].Reasons = %v, want 10+-dependents", sr.High[0].Reasons)
		}
	})

	t.Run("five-plus-routes", func(t *testing.T) {
		g := analyzer.NewGraph()
		n := &analyzer.Node{ID: "u1", Path: "utils/useFoo.ts", Name: "useFoo", Type: analyzer.NodeTypeUtil}
		g.AddNode(n)
		for i := 0; i < 5; i++ {
			pageID := string(rune('a' + i))
			g.AddNode(&analyzer.Node{ID: pageID, Path: "pages/" + pageID + ".vue", Name: pageID, Type: analyzer.NodeTypePage})
			g.AddEdge(analyzer.Edge{From: pageID, To: "u1", Kind: analyzer.EdgeScriptImports, Confidence: analyzer.ConfHigh})
		}

		result := &Result{ChangedNodes: []string{"u1"}}

		sr := Score(g, result)

		if len(sr.High) != 1 {
			t.Fatalf("High = %v, want 1 entry", sr.High)
		}
		if !containsReason(sr.High[0].Reasons, "reaches-5-routes") {
			t.Fatalf("High[0].Reasons = %v, want reaches-5-routes", sr.High[0].Reasons)
		}
	})
}

func TestScore_MediumBandRules(t *testing.T) {
	t.Run("two-plus-dependents", func(t *testing.T) {
		g := analyzer.NewGraph()
		n := &analyzer.Node{ID: "c1", Path: "components/card.vue", Name: "card", Type: analyzer.NodeTypeComponent, DependentCount: 2}
		g.AddNode(n)

		result := &Result{AffectedNodes: []string{"c1"}}

		sr := Score(g, result)

		if len(sr.Medium) != 1 {
			t.Fatalf("Medium = %v, want 1 entry", sr.Medium)
		}
		if sr.Medium[0].NodeID != "c1" {
			t.Fatalf("Medium[0].NodeID = %q, want c1", sr.Medium[0].NodeID)
		}
	})

	t.Run("composable", func(t *testing.T) {
		g := analyzer.NewGraph()
		n := &analyzer.Node{ID: "x1", Path: "composables/useAuth.ts", Name: "useAuth", Type: analyzer.NodeTypeComposable}
		g.AddNode(n)

		result := &Result{ChangedNodes: []string{"x1"}}

		sr := Score(g, result)

		if len(sr.Medium) != 1 {
			t.Fatalf("Medium = %v, want 1 entry", sr.Medium)
		}
		if !containsReason(sr.Medium[0].Reasons, "shared-composable") {
			t.Fatalf("Medium[0].Reasons = %v, want shared-composable", sr.Medium[0].Reasons)
		}
	})
}

func TestScore_LowBandRule(t *testing.T) {
	g := analyzer.NewGraph()
	n := &analyzer.Node{ID: "pg1", Path: "pages/about.vue", Name: "about", Type: analyzer.NodeTypePage}
	g.AddNode(n)

	result := &Result{AffectedNodes: []string{"pg1"}}

	sr := Score(g, result)

	if len(sr.Low) != 1 {
		t.Fatalf("Low = %v, want 1 entry", sr.Low)
	}
	if !containsReason(sr.Low[0].Reasons, "local-page") {
		t.Fatalf("Low[0].Reasons = %v, want local-page", sr.Low[0].Reasons)
	}
}

func TestScore_Tags_GlobalAndAuth(t *testing.T) {
	g := analyzer.NewGraph()
	route := "/"
	n := &analyzer.Node{ID: "m1", Path: "middleware/auth.ts", Name: "auth", Type: analyzer.NodeTypeMiddleware, Route: &route}
	g.AddNode(n)

	result := &Result{ChangedNodes: []string{"m1"}}

	Score(g, result)

	if !containsString(n.Tags, "global-risk") {
		t.Fatalf("Tags = %v, want global-risk", n.Tags)
	}
	if !containsString(n.Tags, "navigation-risk") {
		t.Fatalf("Tags = %v, want navigation-risk", n.Tags)
	}
	if !containsString(n.Tags, "auth-risk") {
		t.Fatalf("Tags = %v, want auth-risk", n.Tags)
	}
	want := []string{"auth-risk", "global-risk", "navigation-risk"}
	if !reflect.DeepEqual(n.Tags, want) {
		t.Fatalf("Tags order = %v, want %v", n.Tags, want)
	}
}

func TestScore_Tags_PaymentsAndUI(t *testing.T) {
	g := analyzer.NewGraph()
	payment := &analyzer.Node{ID: "s1", Path: "stores/payment.ts", Name: "payment", Type: analyzer.NodeTypeStore}
	ui := &analyzer.Node{ID: "c1", Path: "components/Button.vue", Name: "Button", Type: analyzer.NodeTypeComponent, DependentCount: 5}
	g.AddNode(payment)
	g.AddNode(ui)

	result := &Result{ChangedNodes: []string{"s1", "c1"}}

	Score(g, result)

	if !containsString(payment.Tags, "payments-risk") {
		t.Fatalf("payment tags = %v, want payments-risk", payment.Tags)
	}
	if !containsString(ui.Tags, "ui-risk") {
		t.Fatalf("ui tags = %v, want ui-risk", ui.Tags)
	}
}

func TestScore_Tags_RecomputeRemovesNoLongerApplicableTags(t *testing.T) {
	g := analyzer.NewGraph()
	n := &analyzer.Node{ID: "c1", Path: "components/Card.vue", Name: "Card", Type: analyzer.NodeTypeComponent, DependentCount: 5}
	g.AddNode(n)

	first := Score(g, &Result{ChangedNodes: []string{"c1"}})
	if len(first.Low)+len(first.Medium)+len(first.High) != 1 {
		t.Fatalf("first score result = %+v, want one entry", first)
	}
	if !containsString(n.Tags, "ui-risk") {
		t.Fatalf("first tags = %v, want ui-risk", n.Tags)
	}

	n.DependentCount = 1
	second := Score(g, &Result{ChangedNodes: []string{"c1"}})

	if containsString(n.Tags, "ui-risk") {
		t.Fatalf("second tags = %v, want ui-risk removed", n.Tags)
	}
	if !reflect.DeepEqual(n.Tags, []string{}) {
		t.Fatalf("second tags = %v, want no tags", n.Tags)
	}
	if len(second.Low) != 1 {
		t.Fatalf("second result = %+v, want low band", second)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
