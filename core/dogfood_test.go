package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
)

// Dogfood tests simulate real user workflows end-to-end.
// Each test creates a realistic project structure and validates
// the complete output including JSON serialization.

func TestDogfood_NodeProject_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{
		"name": "my-api",
		"version": "1.0.0",
		"scripts": { "start": "node server.js", "build": "tsc" }
	}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "server.js"), []byte(`
		const http = require('http');
		http.createServer((req, res) => res.end('ok')).listen(3000);
	`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.NotNil(t, result.Plan)
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
	require.NotEmpty(t, result.DetectedProviders)
	require.Equal(t, "node", result.DetectedProviders[0])

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	require.NotEmpty(t, jsonBytes)

	var roundTrip BuildResult
	require.NoError(t, json.Unmarshal(jsonBytes, &roundTrip))
	require.True(t, roundTrip.Success)
	require.Equal(t, "npm start", roundTrip.Plan.Deploy.StartCmd)
}

func TestDogfood_GoProject_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module myapp\ngo 1.22\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(`package main
import "fmt"
func main() { fmt.Println("hello") }
`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
	require.Equal(t, "go", result.DetectedProviders[0])

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)

	var roundTrip BuildResult
	require.NoError(t, json.Unmarshal(jsonBytes, &roundTrip))
	require.Equal(t, "/app/server", roundTrip.Plan.Deploy.StartCmd)
}

func TestDogfood_PythonProject_WithEnvConfig(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte("flask==2.0\ngunicorn==20.0\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "app.py"), []byte("from flask import Flask\napp = Flask(__name__)"), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	envVars := map[string]string{
		"THEOPACKS_START_CMD": "gunicorn app:app",
	}
	env := app.NewEnvironment(&envVars)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "gunicorn app:app", result.Plan.Deploy.StartCmd)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestDogfood_StaticSite_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte(`<!DOCTYPE html>
<html><head><title>My Site</title></head>
<body><h1>Hello World</h1></body></html>`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "style.css"), []byte("body { color: red; }"), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		StartCommand: "python -m http.server 8080",
	})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "python -m http.server 8080", result.Plan.Deploy.StartCmd)
	require.Equal(t, "staticfile", result.DetectedProviders[0])
}

func TestDogfood_ShellProject_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "start.sh"), []byte("#!/bin/bash\necho 'starting server'\nnginx -g 'daemon off;'"), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		StartCommand: "bash start.sh",
	})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "bash start.sh", result.Plan.Deploy.StartCmd)
	require.Equal(t, "shell", result.DetectedProviders[0])
}

func TestDogfood_ConfigFile(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "theopacks.json"), []byte(`{
		"deploy": {
			"startCommand": "node custom-server.js"
		},
		"packages": {
			"node": "20"
		}
	}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "node custom-server.js", result.Plan.Deploy.StartCmd)
}

func TestDogfood_ConfigFilePrecedence(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "theopacks.json"), []byte(`{
		"deploy": {
			"startCommand": "from-config-file"
		}
	}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	envVars := map[string]string{
		"THEOPACKS_START_CMD": "from-env",
	}
	env := app.NewEnvironment(&envVars)

	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		StartCommand: "from-options",
	})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "from-options", result.Plan.Deploy.StartCmd,
		"options should take highest precedence")
}

func TestDogfood_DockerignoreIntegration(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte("node_modules\n.git\n*.log\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "node_modules", "some-pkg"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "node_modules", "some-pkg", "index.js"), []byte(""), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success)
	require.Equal(t, "true", result.Metadata["dockerIgnore"])
}

func TestDogfood_EmptyProject(t *testing.T) {
	tempDir := t.TempDir()

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.False(t, result.Success, "empty project should fail — no provider matches")
}

func TestDogfood_FullEnvironmentConfig(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name": "test"}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	envVars := map[string]string{
		"THEOPACKS_INSTALL_CMD":         "npm ci",
		"THEOPACKS_BUILD_CMD":           "npm run build",
		"THEOPACKS_START_CMD":           "node dist/server.js",
		"THEOPACKS_BUILD_APT_PACKAGES":  "build-essential",
		"THEOPACKS_DEPLOY_APT_PACKAGES": "libssl-dev",
	}
	env := app.NewEnvironment(&envVars)

	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "node dist/server.js", result.Plan.Deploy.StartCmd)
}

func TestDogfood_BuildResultJSON_Stability(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test\ngo 1.22"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc main() {}"), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		TheopacksVersion: "0.1.0",
	})

	require.True(t, result.Success)

	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(jsonBytes, &parsed))

	require.Contains(t, parsed, "plan")
	require.Contains(t, parsed, "detectedProviders")
	require.Contains(t, parsed, "success")
	require.Contains(t, parsed, "theopacksVersion")
	require.Equal(t, "0.1.0", parsed["theopacksVersion"])
}

func TestDogfood_InvalidConfigFile(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "theopacks.json"), []byte(`{invalid json`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.False(t, result.Success, "invalid config file should fail")
	require.NotEmpty(t, result.Logs)
}

func TestDogfood_ConfigFileWithComments(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))
	// theopacks.json supports JSONC (comments + trailing commas)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "theopacks.json"), []byte(`{
		// This is a comment
		"deploy": {
			"startCommand": "node server.js", // trailing comma
		},
	}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "JSONC config should be parsed, logs: %v", result.Logs)
	require.Equal(t, "node server.js", result.Plan.Deploy.StartCmd)
}

func TestDogfood_CustomConfigFilePath(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "custom.json"), []byte(`{
		"deploy": { "startCommand": "node custom.js" }
	}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		ConfigFilePath: "custom.json",
	})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "node custom.js", result.Plan.Deploy.StartCmd)
}

func TestDogfood_CustomConfigFileViaEnv(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "env-config.json"), []byte(`{
		"deploy": { "startCommand": "node env.js" }
	}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	envVars := map[string]string{
		"THEOPACKS_CONFIG_FILE": "env-config.json",
	}
	env := app.NewEnvironment(&envVars)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "node env.js", result.Plan.Deploy.StartCmd)
}

func TestDogfood_MissingStartCommand_Error(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		ErrorMissingStartCommand: true,
	})

	// Node provider sets "npm start" as StartCmd, so it shouldn't error
	require.True(t, result.Success, "Node provider sets start command automatically")
}

func TestDogfood_PreviousVersions(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{
		PreviousVersions: map[string]string{
			"node": "18.0.0",
		},
	})

	require.True(t, result.Success, "logs: %v", result.Logs)
}

func TestDogfood_PlanNormalization(t *testing.T) {
	tempDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test\ngo 1.22"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc main() {}"), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success)

	// All steps in the plan should be referenced by deploy or other steps
	for _, step := range result.Plan.Steps {
		require.NotEmpty(t, step.Name, "every step should have a name")
		require.NotEmpty(t, step.Inputs, "every step should have inputs")
	}

	// Deploy should have a base image
	require.NotEmpty(t, result.Plan.Deploy.Base.Image)
}

func TestDogfood_ProviderPriority(t *testing.T) {
	tempDir := t.TempDir()

	// Project has both go.mod AND package.json — Go should win (first in priority)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test\ngo 1.22"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\nfunc main() {}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(`{"name":"test"}`), 0644))

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, result.Success)
	require.Equal(t, "go", result.DetectedProviders[0],
		"Go provider should have higher priority than Node")
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}
