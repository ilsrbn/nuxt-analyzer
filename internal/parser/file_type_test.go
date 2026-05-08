package parser

import "testing"

func TestInferParsedType(t *testing.T) {
	tests := []struct {
		path string
		want ParsedType
	}{
		{"app.vue", ParsedTypeComponent},
		{"components/AppButton.vue", ParsedTypeComponent},
		{"pages/index.vue", ParsedTypePage},
		{"layouts/default.vue", ParsedTypeLayout},
		{"composables/useAuth.ts", ParsedTypeComposable},
		{"stores/auth.ts", ParsedTypeStore},
		{"plugins/api.ts", ParsedTypePlugin},
		{"middleware/auth.ts", ParsedTypeMiddleware},
		{"utils/date.ts", ParsedTypeUtil},
		{"shared/constants.ts", ParsedTypeUtil},
		{"server/api/hello.ts", ParsedTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := inferParsedType(tt.path); got != tt.want {
				t.Fatalf("inferParsedType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
