package providers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLanguageProviders(t *testing.T) {
	providers := GetLanguageProviders()
	require.NotEmpty(t, providers)
	require.Len(t, providers, 5)

	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name()
	}

	require.Equal(t, []string{"go", "python", "node", "staticfile", "shell"}, names)
}

func TestGetProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		found    bool
	}{
		{name: "go provider", provider: "go", found: true},
		{name: "python provider", provider: "python", found: true},
		{name: "node provider", provider: "node", found: true},
		{name: "staticfile provider", provider: "staticfile", found: true},
		{name: "shell provider", provider: "shell", found: true},
		{name: "unknown provider", provider: "ruby", found: false},
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
