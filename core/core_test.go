package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
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
