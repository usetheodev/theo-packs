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

// TestGenerateBuildPlan_DenoBeforeNode locks the ADR-D3 invariant end-to-end:
// when a project ships both deno.json and an npm-compat package.json, the
// detection order must route to the Deno provider, not Node. Provider order
// is asserted at the registry level by TestRegistrationOrder; this test
// closes the loop through the public API (GenerateBuildPlan) so a future
// change to detection semantics — not just registration order — can't
// regress this without breaking a test.
func TestGenerateBuildPlan_DenoBeforeNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Project with BOTH manifests. Many real Deno projects ship a npm-compat
	// package.json so editors / package metadata tooling recognize them.
	err = os.WriteFile(filepath.Join(tempDir, "deno.json"),
		[]byte(`{"imports":{"hono":"jsr:@hono/hono@4"},"tasks":{"start":"deno run -A main.ts"}}`),
		0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "package.json"),
		[]byte(`{"name":"deno-with-npm-compat","scripts":{"start":"echo this should not run"}}`),
		0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "main.ts"), []byte("console.log('hi')"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	buildResult := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, buildResult.Success, "build plan generation should succeed, logs: %v", buildResult.Logs)
	require.NotNil(t, buildResult.Plan)
	require.Contains(t, buildResult.DetectedProviders, "deno",
		"project with both deno.json and package.json must route to Deno (ADR D3)")
	require.NotContains(t, buildResult.DetectedProviders, "node",
		"Node must not win over Deno when deno.json is present")
	require.Equal(t, "deno task start", buildResult.Plan.Deploy.StartCmd,
		"start command must come from Deno provider, not Node's `npm start`")
}

func TestGenerateBuildPlanForRustApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "Cargo.toml"), []byte(`[package]
name = "myapp"
version = "0.1.0"
edition = "2021"
`), 0644)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "src"), 0755))
	err = os.WriteFile(filepath.Join(tempDir, "src", "main.rs"), []byte("fn main(){}"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	r := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, r.Success, "rust plan should succeed, logs: %v", r.Logs)
	require.Contains(t, r.DetectedProviders, "rust")
	require.Equal(t, "/app/server", r.Plan.Deploy.StartCmd)
}

func TestGenerateBuildPlanForJavaApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "build.gradle.kts"), []byte(`plugins {
    java
    id("org.springframework.boot") version "3.3.0"
}
`), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	r := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, r.Success, "java plan should succeed, logs: %v", r.Logs)
	require.Contains(t, r.DetectedProviders, "java")
	require.Equal(t, "java -jar /app/app.jar", r.Plan.Deploy.StartCmd)
}

func TestGenerateBuildPlanForDotnetApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "app.csproj"), []byte(`<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	r := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, r.Success, "dotnet plan should succeed, logs: %v", r.Logs)
	require.Contains(t, r.DetectedProviders, "dotnet")
	require.Equal(t, "dotnet /app/app.dll", r.Plan.Deploy.StartCmd)
}

func TestGenerateBuildPlanForRubyApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "Gemfile"), []byte(`source "https://rubygems.org"
gem "sinatra", "~> 4.0"
`), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "config.ru"), []byte(`run lambda { |env| [200, {}, ["ok"]] }`), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	r := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, r.Success, "ruby plan should succeed, logs: %v", r.Logs)
	require.Contains(t, r.DetectedProviders, "ruby")
	require.Contains(t, r.Plan.Deploy.StartCmd, "rackup")
}

func TestGenerateBuildPlanForPhpApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "composer.json"), []byte(`{
  "require": {"php": ">=8.1", "slim/slim": "^4.0"}
}
`), 0644)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "public"), 0755))
	err = os.WriteFile(filepath.Join(tempDir, "public", "index.php"), []byte("<?php"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	r := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, r.Success, "php plan should succeed, logs: %v", r.Logs)
	require.Contains(t, r.DetectedProviders, "php")
	require.Contains(t, r.Plan.Deploy.StartCmd, "-t public")
}

func TestGenerateBuildPlanForDenoApp(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "core-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	err = os.WriteFile(filepath.Join(tempDir, "deno.json"), []byte(`{
  "imports": {"hono": "jsr:@hono/hono@4"},
  "tasks": {"start": "deno run -A main.ts"}
}
`), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "main.ts"), []byte("Deno.serve(()=>new Response('ok'))"), 0644)
	require.NoError(t, err)

	userApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	r := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	require.True(t, r.Success, "deno plan should succeed, logs: %v", r.Logs)
	require.Contains(t, r.DetectedProviders, "deno")
	require.Equal(t, "deno task start", r.Plan.Deploy.StartCmd)
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
