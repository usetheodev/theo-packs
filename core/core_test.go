package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	c "github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/logger"
)

func TestGenerateBuildPlanForNodeApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test-app", "scripts": {"start": "node index.js"}}`), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(`console.log("hello")`), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, buildResult.Success, "build plan generation should succeed, logs: %v", buildResult.Logs)
	require.NotNil(t, buildResult.Plan)
	require.Equal(t, "npm start", buildResult.Plan.Deploy.StartCmd)
}

func TestGenerateBuildPlanForGoApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test\ngo 1.22"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, buildResult.Success, "build plan generation should succeed, logs: %v", buildResult.Logs)
	require.NotNil(t, buildResult.Plan)
	require.Equal(t, "/app/server", buildResult.Plan.Deploy.StartCmd)
}

func TestGenerateBuildPlanForPythonApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("flask==2.0\n"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "app.py"), []byte("print('hello')"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		StartCommand: "python app.py",
	})

	require.True(t, buildResult.Success, "build plan generation should succeed, logs: %v", buildResult.Logs)
	require.NotNil(t, buildResult.Plan)
	require.Equal(t, "python app.py", buildResult.Plan.Deploy.StartCmd)
}

func TestGenerateConfigFromFile_NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	l := logger.NewLogger()

	options := &GenerateBuildPlanOptions{ConfigFilePath: "does-not-exist.theopacks.json"}
	cfg, genErr := GenerateConfigFromFile(userApp, env, options, l)

	require.Error(t, genErr, "expected an error when explicit config file does not exist")
	require.Nil(t, cfg, "config should be nil on error")
}

func TestGenerateBuildPlan_DockerignoreMetadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a node app with dockerignore
	err = os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test"}`), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte("node_modules\n"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, buildResult.Success)
	require.NotNil(t, buildResult.Metadata)
	require.Equal(t, "true", buildResult.Metadata["dockerIgnore"])
}

func TestGenerateConfigFromEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected string
	}{
		{
			name:    "empty environment",
			envVars: map[string]string{},
			expected: `{
				"steps": {},
				"packages": {},
				"caches": {},
				"deploy": {}
			}`,
		},
		{
			name: "kitchen sink",
			envVars: map[string]string{
				"THEOPACKS_INSTALL_CMD":         "npm install",
				"THEOPACKS_BUILD_CMD":           "npm run build",
				"THEOPACKS_START_CMD":           "npm start",
				"THEOPACKS_PACKAGES":            "node@18 python@3.9",
				"THEOPACKS_BUILD_APT_PACKAGES":  "build-essential libssl-dev",
				"THEOPACKS_DEPLOY_APT_PACKAGES": "libssl-dev",
			},
			expected: `{
				"steps": {
					"install": {
						"name": "install",
						"commands": [
							{ "src": ".", "dest": "." },
							"npm install"
						],
						"secrets": ["*"],
						"assets": {},
						"variables": {}
					},
					"build": {
						"name": "build",
						"commands": [
							{ "src": ".", "dest": "." },
							"npm run build"
						],
						"secrets": ["*"],
						"assets": {},
						"variables": {}
					}
				},
				"buildAptPackages": ["build-essential", "libssl-dev"],
				"packages": {
					"node": "18",
					"python": "3.9"
				},
				"caches": {},
				"deploy": {
					"startCommand": "npm start",
					"aptPackages": ["libssl-dev"]
				},
				"secrets": ["THEOPACKS_BUILD_APT_PACKAGES", "THEOPACKS_BUILD_CMD", "THEOPACKS_DEPLOY_APT_PACKAGES",
					"THEOPACKS_INSTALL_CMD", "THEOPACKS_PACKAGES", "THEOPACKS_START_CMD"]
			}`,
		},
		{
			name: "unversioned packages",
			envVars: map[string]string{
				"THEOPACKS_PACKAGES": "jq pipx:httpie@3.2.4",
			},
			expected: `{
				"steps": {},
				"packages": {
					"jq": "latest",
					"pipx:httpie": "3.2.4"
				},
				"caches": {},
				"deploy": {},
				"secrets": ["THEOPACKS_PACKAGES"]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := app.NewEnvironment(&tt.envVars)
			gotConfig := GenerateConfigFromEnvironment(env)

			serializedConfig := c.Config{}
			err := json.Unmarshal([]byte(tt.expected), &serializedConfig)
			require.NoError(t, err)

			if diff := cmp.Diff(serializedConfig, *gotConfig); diff != "" {
				t.Errorf("GenerateConfigFromEnvironment() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGenerateConfigFromOptions(t *testing.T) {
	t.Run("nil options", func(t *testing.T) {
		cfg := GenerateConfigFromOptions(nil)
		require.NotNil(t, cfg)
		require.Empty(t, cfg.Steps)
		require.Empty(t, cfg.Deploy.StartCmd)
	})

	t.Run("with build and start commands", func(t *testing.T) {
		cfg := GenerateConfigFromOptions(&GenerateBuildPlanOptions{
			BuildCommand: "npm run build",
			StartCommand: "npm start",
		})
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.Steps["build"])
		require.Equal(t, "npm start", cfg.Deploy.StartCmd)
	})
}

func TestGenerateConfigFromFile_DefaultNotRequired(t *testing.T) {
	tempDir := t.TempDir()

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	l := logger.NewLogger()

	cfg, genErr := GenerateConfigFromFile(userApp, env, &GenerateBuildPlanOptions{}, l)

	require.NoError(t, genErr, "default config file not existing should not error")
	require.NotNil(t, cfg)
}

func TestGenerateBuildPlanWithStartCommand(t *testing.T) {
	tempDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("<html></html>"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		StartCommand: "python -m http.server",
	})

	require.True(t, buildResult.Success, "build plan generation should succeed, logs: %v", buildResult.Logs)
	require.Equal(t, "python -m http.server", buildResult.Plan.Deploy.StartCmd)
}

func TestGenerateBuildPlanNoProvider(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Empty directory - no provider should match
	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.False(t, buildResult.Success)
}
