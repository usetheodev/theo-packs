package python

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

func TestPythonDetect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name:     "detects requirements.txt",
			files:    map[string]string{"requirements.txt": "flask==2.0"},
			expected: true,
		},
		{
			name:     "detects pyproject.toml",
			files:    map[string]string{"pyproject.toml": "[project]\nname = \"test\""},
			expected: true,
		},
		{
			name:     "detects Pipfile",
			files:    map[string]string{"Pipfile": "[packages]\nflask = \"*\""},
			expected: true,
		},
		{
			name:     "detects setup.py",
			files:    map[string]string{"setup.py": "from setuptools import setup"},
			expected: true,
		},
		{
			name:     "no python files",
			files:    map[string]string{"main.go": "package main"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "python-test")
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

			provider := &PythonProvider{}
			detected, err := provider.Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, detected)
		})
	}
}
