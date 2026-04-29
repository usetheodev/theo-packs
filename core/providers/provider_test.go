package providers

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// expectedOrder locks the provider detection order. Any change here is a
// behavior change (first-match-wins detection) and MUST be intentional.
// See ADR D3 in docs/plans/add-six-language-providers-plan.md.
var expectedOrder = []string{
	"go",
	"rust",
	"java",
	"dotnet",
	"ruby",
	"php",
	"python",
	"deno",
	"node",
	"staticfile",
	"shell",
}

func TestGetLanguageProviders(t *testing.T) {
	providers := GetLanguageProviders()
	require.NotEmpty(t, providers)
	require.Len(t, providers, len(expectedOrder))

	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}

	require.Equal(t, expectedOrder, names)
}

// TestRegistrationOrder pins the full provider order — the public contract
// for first-match-wins detection in core/core.go.
func TestRegistrationOrder(t *testing.T) {
	providers := GetLanguageProviders()
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}
	require.Equal(t, expectedOrder, names, "provider order is part of the public contract")
}

// TestDenoBeforeNode asserts the most ordering-sensitive invariant from
// ADR D3 in isolation. Kept as its own test (rather than a sub-assertion
// of TestRegistrationOrder) so a regression produces a focused failure
// with the rationale right next to the diff.
func TestDenoBeforeNode(t *testing.T) {
	providers := GetLanguageProviders()
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}

	denoIdx := slices.Index(names, "deno")
	nodeIdx := slices.Index(names, "node")
	require.NotEqual(t, -1, denoIdx, "deno must be registered")
	require.NotEqual(t, -1, nodeIdx, "node must be registered")
	require.Less(t, denoIdx, nodeIdx,
		"Deno must be detected BEFORE Node so projects with both deno.json and package.json route to Deno (ADR D3)")
}

func TestRegistrationCount(t *testing.T) {
	require.Len(t, GetLanguageProviders(), 11,
		"theo-packs registers 11 providers: 5 original (go, python, node, staticfile, shell) + 6 new (rust, java, dotnet, ruby, php, deno)")
}

func TestNamesAreUnique(t *testing.T) {
	providers := GetLanguageProviders()
	seen := make(map[string]bool, len(providers))
	for _, p := range providers {
		name := p.Name()
		require.False(t, seen[name], "duplicate provider name: %s", name)
		seen[name] = true
	}
}

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		found    bool
	}{
		{name: "go provider", provider: "go", found: true},
		{name: "rust provider", provider: "rust", found: true},
		{name: "java provider", provider: "java", found: true},
		{name: "dotnet provider", provider: "dotnet", found: true},
		{name: "ruby provider", provider: "ruby", found: true},
		{name: "php provider", provider: "php", found: true},
		{name: "python provider", provider: "python", found: true},
		{name: "deno provider", provider: "deno", found: true},
		{name: "node provider", provider: "node", found: true},
		{name: "staticfile provider", provider: "staticfile", found: true},
		{name: "shell provider", provider: "shell", found: true},
		{name: "unknown provider", provider: "elixir", found: false},
		{name: "empty name", provider: "", found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := GetProvider(tt.provider)
			if tt.found {
				require.NotNil(t, provider)
				require.Equal(t, tt.provider, provider.Name())
			} else {
				require.Nil(t, provider)
			}
		})
	}
}
