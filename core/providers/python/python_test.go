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

func TestPythonPlan(t *testing.T) {
	t.Run("with requirements.txt", func(t *testing.T) {
		tempDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		provider := &PythonProvider{}
		err = provider.Plan(ctx)
		require.NoError(t, err)

		require.Len(t, ctx.Steps, 2)
		require.Equal(t, "install", ctx.Steps[0].Name())
		require.Equal(t, "build", ctx.Steps[1].Name())
	})

	t.Run("with pyproject.toml", func(t *testing.T) {
		tempDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte("[project]\nname = \"test\""), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		provider := &PythonProvider{}
		err = provider.Plan(ctx)
		require.NoError(t, err)

		require.Len(t, ctx.Steps, 1)
	})
}

func TestPythonPlan_Pipfile(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "Pipfile"), []byte("[packages]\nflask = \"*\"\n"), 0644)
	require.NoError(t, err)

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	provider := &PythonProvider{}
	err = provider.Plan(ctx)
	require.NoError(t, err)

	require.Len(t, ctx.Steps, 2)
	require.Equal(t, "install", ctx.Steps[0].Name())
	require.Equal(t, "build", ctx.Steps[1].Name())

	// Build the plan to verify it produces valid commands
	buildPlan, _, err := ctx.Generate()
	require.NoError(t, err)
	require.NotEmpty(t, buildPlan.Steps)
	require.NotEmpty(t, buildPlan.Steps[0].Commands,
		"Pipfile plan should generate install commands")
}

func TestPythonPlan_SetupPy(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "setup.py"), []byte("from setuptools import setup\nsetup(name='test')"), 0644)
	require.NoError(t, err)

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	provider := &PythonProvider{}
	err = provider.Plan(ctx)
	require.NoError(t, err)

	require.Len(t, ctx.Steps, 1)

	buildPlan, _, err := ctx.Generate()
	require.NoError(t, err)
	require.NotEmpty(t, buildPlan.Steps)
	require.NotEmpty(t, buildPlan.Steps[0].Commands,
		"setup.py plan should generate install commands")
}

func TestPythonStartCommandHelp(t *testing.T) {
	provider := &PythonProvider{}
	help := provider.StartCommandHelp()
	require.NotEmpty(t, help)
	require.Contains(t, help, "THEOPACKS_START_CMD")
}

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
