package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
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

func TestPythonPlan_Poetry(t *testing.T) {
	t.Run("with poetry.lock", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[project]
name = "myapp"

[tool.poetry]
name = "myapp"
version = "1.0.0"
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "poetry.lock"), []byte("# lock file\n"), 0644)
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

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)
		require.NotEmpty(t, buildPlan.Steps)

		// Verify poetry install command is used
		foundPoetry := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if execCmd, ok := cmd.(plan.ExecCommand); ok {
				if strings.Contains(execCmd.Cmd, "poetry install") {
					foundPoetry = true
				}
			}
		}
		require.True(t, foundPoetry, "Poetry project should use poetry install")

		// Verify poetry.lock is copied
		foundLockCopy := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if copyCmd, ok := cmd.(plan.CopyCommand); ok {
				if copyCmd.Src == "poetry.lock" {
					foundLockCopy = true
				}
			}
		}
		require.True(t, foundLockCopy, "Poetry project should copy poetry.lock")
	})

	t.Run("without poetry.lock", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[project]
name = "myapp"

[tool.poetry]
name = "myapp"
version = "1.0.0"
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
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

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)

		// Verify poetry.lock is NOT copied when absent
		for _, cmd := range buildPlan.Steps[0].Commands {
			if copyCmd, ok := cmd.(plan.CopyCommand); ok {
				require.NotEqual(t, "poetry.lock", copyCmd.Src,
					"Should not copy poetry.lock when it does not exist")
			}
		}
	})
}

func TestIsPoetryProject(t *testing.T) {
	t.Run("true when tool.poetry.name is set", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[tool.poetry]
name = "myapp"
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		require.True(t, isPoetryProject(ctx))
	})

	t.Run("false when no pyproject.toml", func(t *testing.T) {
		tempDir := t.TempDir()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		require.False(t, isPoetryProject(ctx))
	})

	t.Run("false when pyproject.toml has no poetry section", func(t *testing.T) {
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

		require.False(t, isPoetryProject(ctx))
	})
}

func TestPythonPlan_PipfileCommands(t *testing.T) {
	t.Run("with Pipfile.lock", func(t *testing.T) {
		tempDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tempDir, "Pipfile"), []byte("[packages]\nflask = \"*\"\n"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "Pipfile.lock"), []byte("{}\n"), 0644)
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

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)
		require.NotEmpty(t, buildPlan.Steps)

		// Verify pipenv command is used
		foundPipenv := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if execCmd, ok := cmd.(plan.ExecCommand); ok {
				if strings.Contains(execCmd.Cmd, "pipenv") {
					foundPipenv = true
				}
			}
		}
		require.True(t, foundPipenv, "Pipfile project should use pipenv")

		// Verify Pipfile.lock is copied
		foundLockCopy := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if copyCmd, ok := cmd.(plan.CopyCommand); ok {
				if copyCmd.Src == "Pipfile.lock" {
					foundLockCopy = true
				}
			}
		}
		require.True(t, foundLockCopy, "Pipfile project should copy Pipfile.lock")
	})

	t.Run("without Pipfile.lock", func(t *testing.T) {
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

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)

		// Verify Pipfile.lock is NOT copied when absent
		for _, cmd := range buildPlan.Steps[0].Commands {
			if copyCmd, ok := cmd.(plan.CopyCommand); ok {
				require.NotEqual(t, "Pipfile.lock", copyCmd.Src,
					"Should not copy Pipfile.lock when it does not exist")
			}
		}
	})
}

func TestPythonPlan_UvWorkspace(t *testing.T) {
	tempDir := t.TempDir()

	pyprojectContent := `[project]
name = "myworkspace"

[tool.uv.workspace]
members = ["packages/*"]
`
	err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "uv.lock"), []byte("# uv lock\n"), 0644)
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
	require.Equal(t, "install", ctx.Steps[0].Name())

	buildPlan, _, err := ctx.Generate()
	require.NoError(t, err)
	require.NotEmpty(t, buildPlan.Steps)

	// Verify uv sync command is used
	foundUv := false
	for _, cmd := range buildPlan.Steps[0].Commands {
		if execCmd, ok := cmd.(plan.ExecCommand); ok {
			if strings.Contains(execCmd.Cmd, "uv sync") {
				foundUv = true
			}
		}
	}
	require.True(t, foundUv, "UV workspace project should use uv sync")
}

func TestIsUvWorkspace(t *testing.T) {
	t.Run("true when workspace members are set", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[tool.uv.workspace]
members = ["packages/*"]
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		require.True(t, isUvWorkspace(ctx))
	})

	t.Run("false when no pyproject.toml", func(t *testing.T) {
		tempDir := t.TempDir()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		require.False(t, isUvWorkspace(ctx))
	})

	t.Run("false when workspace members are empty", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[tool.uv.workspace]
members = []
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := generate.NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		require.False(t, isUvWorkspace(ctx))
	})
}

func TestPythonPlan_SetupPyCommands(t *testing.T) {
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

	// Verify pip install . is used for setup.py
	foundPipInstall := false
	for _, cmd := range buildPlan.Steps[0].Commands {
		if execCmd, ok := cmd.(plan.ExecCommand); ok {
			if strings.Contains(execCmd.Cmd, "pip install") && strings.Contains(execCmd.Cmd, ".") {
				foundPipInstall = true
			}
		}
	}
	require.True(t, foundPipInstall, "setup.py project should use pip install .")
}

func TestPythonPlan_PriorityOrder(t *testing.T) {
	t.Run("requirements.txt takes priority over pyproject.toml", func(t *testing.T) {
		tempDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte("[project]\nname = \"test\""), 0644)
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

		// requirements.txt path produces 2 steps (install + build)
		require.Len(t, ctx.Steps, 2)
		require.Equal(t, "install", ctx.Steps[0].Name())
		require.Equal(t, "build", ctx.Steps[1].Name())

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)

		// Verify requirements.txt is copied (not pip install .)
		foundReqCopy := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if copyCmd, ok := cmd.(plan.CopyCommand); ok {
				if copyCmd.Src == "requirements.txt" {
					foundReqCopy = true
				}
			}
		}
		require.True(t, foundReqCopy,
			"When both requirements.txt and pyproject.toml exist, requirements.txt should take priority")
	})

	t.Run("uv workspace takes priority over requirements.txt", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[project]
name = "myworkspace"

[tool.uv.workspace]
members = ["packages/*"]
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
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

		// UV workspace produces 1 step (install only)
		require.Len(t, ctx.Steps, 1)
		require.Equal(t, "install", ctx.Steps[0].Name())

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)

		// Verify uv sync is used, not pip -r requirements.txt
		foundUv := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if execCmd, ok := cmd.(plan.ExecCommand); ok {
				if strings.Contains(execCmd.Cmd, "uv sync") {
					foundUv = true
				}
			}
		}
		require.True(t, foundUv,
			"UV workspace should take priority over requirements.txt")
	})

	t.Run("requirements.txt takes priority over poetry", func(t *testing.T) {
		tempDir := t.TempDir()

		pyprojectContent := `[project]
name = "myapp"

[tool.poetry]
name = "myapp"
version = "1.0.0"
`
		err := os.WriteFile(filepath.Join(tempDir, "pyproject.toml"), []byte(pyprojectContent), 0644)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
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

		buildPlan, _, err := ctx.Generate()
		require.NoError(t, err)

		// Verify requirements.txt approach is used
		foundReqCopy := false
		for _, cmd := range buildPlan.Steps[0].Commands {
			if copyCmd, ok := cmd.(plan.CopyCommand); ok {
				if copyCmd.Src == "requirements.txt" {
					foundReqCopy = true
				}
			}
		}
		require.True(t, foundReqCopy,
			"requirements.txt should take priority over poetry")
	})
}

// --- version detection ---

func createPythonTempApp(t *testing.T, files map[string]string) *app.App {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}
	a, err := app.NewApp(dir)
	require.NoError(t, err)
	return a
}

func createPythonTestContext(t *testing.T, a *app.App, envVars map[string]string) *generate.GenerateContext {
	t.Helper()
	var envPtr *map[string]string
	if envVars != nil {
		envPtr = &envVars
	}
	env := app.NewEnvironment(envPtr)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)
	return ctx
}

func TestDetectPythonVersion(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		envVars    map[string]string
		wantVer    string
		wantSource string
	}{
		{
			name:       "default when no version info",
			files:      map[string]string{"requirements.txt": "flask"},
			wantVer:    "3.12",
			wantSource: "default",
		},
		{
			name: "reads .python-version",
			files: map[string]string{
				"requirements.txt": "flask",
				".python-version":  "3.11",
			},
			wantVer:    "3.11",
			wantSource: ".python-version",
		},
		{
			name: "reads .python-version with patch",
			files: map[string]string{
				"requirements.txt": "flask",
				".python-version":  "3.9.18",
			},
			wantVer:    "3.9",
			wantSource: ".python-version",
		},
		{
			name: "reads runtime.txt",
			files: map[string]string{
				"requirements.txt": "flask",
				"runtime.txt":      "python-3.10.12",
			},
			wantVer:    "3.10",
			wantSource: "runtime.txt",
		},
		{
			name: ".python-version beats runtime.txt",
			files: map[string]string{
				"requirements.txt": "flask",
				".python-version":  "3.11",
				"runtime.txt":      "python-3.10.12",
			},
			wantVer:    "3.11",
			wantSource: ".python-version",
		},
		{
			name: "env var overrides .python-version",
			files: map[string]string{
				"requirements.txt": "flask",
				".python-version":  "3.11",
			},
			envVars:    map[string]string{"THEOPACKS_PYTHON_VERSION": "3.9"},
			wantVer:    "3.9",
			wantSource: "THEOPACKS_PYTHON_VERSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createPythonTempApp(t, tt.files)
			ctx := createPythonTestContext(t, a, tt.envVars)

			version, source := detectPythonVersion(ctx)
			require.Equal(t, tt.wantVer, version)
			require.Equal(t, tt.wantSource, source)
		})
	}
}

func TestDetectPythonVersion_ConfigPackages(t *testing.T) {
	a := createPythonTempApp(t, map[string]string{
		"requirements.txt": "flask",
		".python-version":  "3.11",
	})
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	cfg.Packages = map[string]string{"python": "3.9"}
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)

	version, source := detectPythonVersion(ctx)
	require.Equal(t, "3.9", version)
	require.Equal(t, "custom config", source)
}

func TestPythonDeployIncludes(t *testing.T) {
	includes := pythonDeployIncludes("3.11")
	require.Equal(t, []string{
		".",
		"/usr/local/lib/python3.11/site-packages",
		"/usr/local/bin",
	}, includes)

	includes312 := pythonDeployIncludes("3.12")
	require.Equal(t, "/usr/local/lib/python3.12/site-packages", includes312[1])
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
