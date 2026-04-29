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

func createNodeTempApp(t *testing.T, files map[string]string) *app.App {
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

func createNodeTestContext(t *testing.T, a *app.App, envVars map[string]string) *generate.GenerateContext {
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

// --- version detection ---

func TestDetectNodeVersion(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		envVars    map[string]string
		wantVer    string
		wantSource string
	}{
		{
			name:       "default when no version info",
			files:      map[string]string{"package.json": `{"name":"test"}`},
			wantVer:    "20",
			wantSource: "default",
		},
		{
			name:       "reads engines.node",
			files:      map[string]string{"package.json": `{"name":"test","engines":{"node":">=22"}}`},
			wantVer:    "22",
			wantSource: "package.json engines.node",
		},
		{
			name:       "reads engines.node caret",
			files:      map[string]string{"package.json": `{"name":"test","engines":{"node":"^18.3.2"}}`},
			wantVer:    "18",
			wantSource: "package.json engines.node",
		},
		{
			name: "reads .nvmrc",
			files: map[string]string{
				"package.json": `{"name":"test"}`,
				".nvmrc":       "18",
			},
			wantVer:    "18",
			wantSource: ".nvmrc",
		},
		{
			name: "reads .nvmrc with v prefix",
			files: map[string]string{
				"package.json": `{"name":"test"}`,
				".nvmrc":       "v22.2.0",
			},
			wantVer:    "22",
			wantSource: ".nvmrc",
		},
		{
			name: "reads .node-version",
			files: map[string]string{
				"package.json": `{"name":"test"}`,
				".node-version": "18.20.5",
			},
			wantVer:    "18",
			wantSource: ".node-version",
		},
		{
			name: "engines.node beats .nvmrc",
			files: map[string]string{
				"package.json": `{"name":"test","engines":{"node":">=22"}}`,
				".nvmrc":       "18",
			},
			wantVer:    "22",
			wantSource: "package.json engines.node",
		},
		{
			name: ".nvmrc beats .node-version",
			files: map[string]string{
				"package.json":  `{"name":"test"}`,
				".nvmrc":        "22",
				".node-version": "18",
			},
			wantVer:    "22",
			wantSource: ".nvmrc",
		},
		{
			name: "env var overrides engines",
			files: map[string]string{
				"package.json": `{"name":"test","engines":{"node":">=18"}}`,
			},
			envVars:    map[string]string{"THEOPACKS_NODE_VERSION": "22"},
			wantVer:    "22",
			wantSource: "THEOPACKS_NODE_VERSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createNodeTempApp(t, tt.files)
			ctx := createNodeTestContext(t, a, tt.envVars)
			pkg := readPackageJSON(a, logger.NewLogger())

			version, source := detectNodeVersion(ctx, pkg)
			require.Equal(t, tt.wantVer, version)
			require.Equal(t, tt.wantSource, source)
		})
	}
}

func TestDetectNodeVersion_ConfigPackages(t *testing.T) {
	a := createNodeTempApp(t, map[string]string{
		"package.json": `{"name":"test","engines":{"node":">=18"}}`,
		".nvmrc":       "20",
	})
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	cfg.Packages = map[string]string{"node": "22"}
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)
	pkg := readPackageJSON(a, log)

	version, source := detectNodeVersion(ctx, pkg)
	require.Equal(t, "22", version)
	require.Equal(t, "custom config", source)
}

// --- hardening tests ---

func TestDetectPackageManager_BunLockb(t *testing.T) {
	// bun.lockb (binary lockfile) should trigger Bun detection
	a := createNodeTempApp(t, map[string]string{
		"package.json": `{"name":"test"}`,
		"bun.lockb":    "",
	})
	require.Equal(t, PackageManagerBun, DetectPackageManager(a))
}

func TestNodePlan_NoScripts(t *testing.T) {
	// package.json with no scripts at all should still produce a valid plan
	a := createNodeTempApp(t, map[string]string{
		"package.json": `{"name": "test", "version": "1.0.0"}`,
	})
	ctx := createNodeTestContext(t, a, nil)

	provider := &NodeProvider{}
	err := provider.Plan(ctx)
	require.NoError(t, err)

	// With no start script, falls back to "npm start"
	require.Equal(t, "npm start", ctx.Deploy.StartCmd)
	require.Len(t, ctx.Steps, 2)
}

func TestDetectPackageManager_ConflictingLockFiles(t *testing.T) {
	// When yarn.lock and pnpm-lock.yaml both exist, pnpm wins
	// because it's checked first in DetectPackageManager
	a := createNodeTempApp(t, map[string]string{
		"package.json":    `{"name":"test"}`,
		"yarn.lock":       "# yarn",
		"pnpm-lock.yaml":  "lockfileVersion: 6",
	})
	require.Equal(t, PackageManagerPnpm, DetectPackageManager(a))
}
