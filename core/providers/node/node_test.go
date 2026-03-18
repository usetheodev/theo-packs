package node

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
)

func TestNodeDetect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name:     "detects package.json",
			files:    map[string]string{"package.json": `{"name": "test"}`},
			expected: true,
		},
		{
			name:     "no package.json",
			files:    map[string]string{"main.go": "package main"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "node-test")
			require.NoError(t, err)
			defer func() { _ = os.RemoveAll(tempDir) }()

			for name, content := range tt.files {
				err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
				require.NoError(t, err)
			}

			testApp, err := app.NewApp(tempDir)
			require.NoError(t, err)
			env := app.NewEnvironment(nil)
			cfg := config.EmptyConfig()
			log := logger.NewLogger()

			ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
			require.NoError(t, err)

			provider := &NodeProvider{}
			detected, err := provider.Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, detected)
		})
	}
}

func TestNodePlan(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "node-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test"}`), 0644)
	require.NoError(t, err)

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	provider := &NodeProvider{}
	err = provider.Plan(ctx)
	require.NoError(t, err)

	require.Equal(t, "npm start", ctx.Deploy.StartCmd)
	require.Len(t, ctx.Steps, 2)
}
