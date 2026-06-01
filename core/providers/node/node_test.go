package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theokitpacks/core/app"
	"github.com/usetheo/theokitpacks/core/config"
	"github.com/usetheo/theokitpacks/core/generate"
	"github.com/usetheo/theokitpacks/core/logger"
	"github.com/usetheo/theokitpacks/core/plan"
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
				"package.json":  `{"name":"test"}`,
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
			envVars:    map[string]string{"THEOKIT_PACKS_NODE_VERSION": "22"},
			wantVer:    "22",
			wantSource: "THEOKIT_PACKS_NODE_VERSION",
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
		"package.json":   `{"name":"test"}`,
		"yarn.lock":      "# yarn",
		"pnpm-lock.yaml": "lockfileVersion: 6",
	})
	require.Equal(t, PackageManagerPnpm, DetectPackageManager(a))
}

// --- Build-step prune (Phase 1: drop devDependencies before deploy) ---

// buildStepCommandStrings returns the rendered command bodies for the build
// step, in order. Helper for asserting prune lands at the right place.
func buildStepCommandStrings(t *testing.T, ctx *generate.GenerateContext) []string {
	t.Helper()
	for _, s := range ctx.Steps {
		if s.Name() != "build" {
			continue
		}
		csb, ok := s.(*generate.CommandStepBuilder)
		require.True(t, ok, "build step must be a CommandStepBuilder")
		out := make([]string, 0, len(csb.Commands))
		for _, c := range csb.Commands {
			ec, ok := c.(plan.ExecCommand)
			if !ok {
				continue
			}
			out = append(out, ec.Cmd)
		}
		return out
	}
	t.Fatal("no build step found")
	return nil
}

func TestNodeProvider_BuildStepHasPrune_Npm(t *testing.T) {
	a := createNodeTempApp(t, map[string]string{
		"package.json":      `{"name": "test", "scripts": {"build": "echo build"}}`,
		"package-lock.json": "{}",
	})
	ctx := createNodeTestContext(t, a, nil)
	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	require.NotEmpty(t, cmds)
	require.Equal(t, "npm prune --omit=dev", cmds[len(cmds)-1])
}

func TestNodeProvider_BuildStepHasPrune_Pnpm(t *testing.T) {
	a := createNodeTempApp(t, map[string]string{
		"package.json":   `{"name": "test", "scripts": {"build": "echo build"}}`,
		"pnpm-lock.yaml": "lockfileVersion: 6",
	})
	ctx := createNodeTestContext(t, a, nil)
	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	require.NotEmpty(t, cmds)
	require.Equal(t, "pnpm prune --prod", cmds[len(cmds)-1])
}

func TestNodeProvider_BuildStepHasPrune_Yarn(t *testing.T) {
	a := createNodeTempApp(t, map[string]string{
		"package.json": `{"name": "test", "scripts": {"build": "echo build"}}`,
		"yarn.lock":    "# yarn",
	})
	ctx := createNodeTestContext(t, a, nil)
	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	require.NotEmpty(t, cmds)
	require.Equal(t, "yarn install --production --ignore-scripts --prefer-offline", cmds[len(cmds)-1])
}

func TestNodeProvider_BunBuildStepHasNoPrune(t *testing.T) {
	a := createNodeTempApp(t, map[string]string{
		"package.json": `{"name": "test", "scripts": {"build": "echo build"}}`,
		"bun.lockb":    "",
	})
	ctx := createNodeTestContext(t, a, nil)
	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	for _, c := range cmds {
		require.NotContains(t, c, "prune", "bun has no prune subcommand; we must skip it")
		require.NotContains(t, c, "--production", "no production reinstall for bun either")
	}
}

func TestNodeProvider_PruneRunsEvenWithoutBuildScript(t *testing.T) {
	// A package.json without a `build` script must still get the prune line —
	// devDependencies are still installed by the install step and should not
	// ship to deploy regardless of whether a build script ran.
	a := createNodeTempApp(t, map[string]string{
		"package.json":      `{"name": "test", "version": "1.0.0"}`,
		"package-lock.json": "{}",
	})
	ctx := createNodeTestContext(t, a, nil)
	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	require.Contains(t, cmds, "npm prune --omit=dev")
}

// --- B1 fix: pnpm/turbo --filter must resolve against package.json#name ---

func TestNodeProvider_B1_ResolvesRealPackageNameFromAppPath(t *testing.T) {
	// Workspace fixture where the app's directory name ("api") does NOT
	// match its package.json name ("@scope/api"). The CLI contract is that
	// --app-name = dir-name; the provider must read the real name from
	// apps/api/package.json and pass THAT to pnpm --filter.
	a := createNodeTempApp(t, map[string]string{
		"package.json":             `{"name":"root","private":true}`,
		"pnpm-workspace.yaml":      "packages:\n  - apps/*\n",
		"pnpm-lock.yaml":           "lockfileVersion: '9.0'\n",
		"apps/api/package.json":    `{"name":"@scope/api","scripts":{"build":"echo build","start":"node ."}}`,
		"apps/web/package.json":    `{"name":"@scope/web","scripts":{"build":"echo build"}}`,
	})
	ctx := createNodeTestContext(t, a, map[string]string{
		"THEOKIT_PACKS_APP_NAME": "api",
		"THEOKIT_PACKS_APP_PATH": "apps/api",
	})

	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	var foundFilter string
	for _, c := range cmds {
		if strings.Contains(c, "--filter") {
			foundFilter = c
			break
		}
	}
	require.NotEmpty(t, foundFilter, "expected a --filter command in build step")
	require.Contains(t, foundFilter, "@scope/api", "filter must use real package.json#name, not dir-name")
	require.NotContains(t, foundFilter, "--filter api...", "filter must NOT use the bare dir-name")
}

func TestNodeProvider_B1_FallsBackToAppNameWhenPackageJSONMissing(t *testing.T) {
	// If <appPath>/package.json is unreadable, the provider must NOT fail —
	// it falls back to the appName literal from the env. (Defensive: handles
	// races and unusual workspace layouts without aborting the build plan.)
	a := createNodeTempApp(t, map[string]string{
		"package.json":          `{"name":"root","private":true}`,
		"pnpm-workspace.yaml":   "packages:\n  - apps/*\n",
		"pnpm-lock.yaml":        "lockfileVersion: '9.0'\n",
		"apps/api/package.json": `{"name":"@scope/api","scripts":{"build":"echo build"}}`,
	})
	ctx := createNodeTestContext(t, a, map[string]string{
		"THEOKIT_PACKS_APP_NAME": "api",
		"THEOKIT_PACKS_APP_PATH": "apps/nonexistent",
	})

	require.NoError(t, (&NodeProvider{}).Plan(ctx))

	cmds := buildStepCommandStrings(t, ctx)
	var foundFilter string
	for _, c := range cmds {
		if strings.Contains(c, "--filter") {
			foundFilter = c
			break
		}
	}
	require.NotEmpty(t, foundFilter, "expected a --filter command")
	require.Contains(t, foundFilter, "api", "fallback to appName literal when real name unreadable")
}

// --- B3 fix: fail-fast on cross-repo workspace entries (../) ---

func TestNodeProvider_B3_RejectsSiblingWorkspaceEntry(t *testing.T) {
	// pnpm-workspace.yaml lists "../theokit-sdk/packages/sdk" — that path
	// can never exist inside a Docker build context rooted at --source.
	// The provider must fail fast with a structured error nameing the
	// offending entry and the two remediation paths.
	a := createNodeTempApp(t, map[string]string{
		"package.json": `{"name":"root","private":true}`,
		"pnpm-workspace.yaml": "packages:\n" +
			"  - apps/*\n" +
			"  - '../theokit-sdk/packages/sdk'\n",
		"pnpm-lock.yaml":        "lockfileVersion: '9.0'\n",
		"apps/api/package.json": `{"name":"@scope/api","scripts":{"build":"echo build"}}`,
	})
	ctx := createNodeTestContext(t, a, nil)
	err := (&NodeProvider{}).Plan(ctx)
	require.Error(t, err, "Plan() must fail when workspace has a ../ entry")
	require.Contains(t, err.Error(), "../theokit-sdk/packages/sdk", "error must name the offending entry")
	require.Contains(t, err.Error(), "Publish", "error must offer the publish-as-npm remediation")
	require.Contains(t, err.Error(), "common parent", "error must offer the multi-repo-as-context remediation")
}

func TestNodeProvider_B3_AcceptsPureSiblingFreeWorkspace(t *testing.T) {
	// Workspace with only legitimate entries (apps/*, packages/*) must NOT
	// trigger the validator.
	a := createNodeTempApp(t, map[string]string{
		"package.json":          `{"name":"root","private":true}`,
		"pnpm-workspace.yaml":   "packages:\n  - apps/*\n  - packages/*\n",
		"pnpm-lock.yaml":        "lockfileVersion: '9.0'\n",
		"apps/api/package.json": `{"name":"@scope/api","scripts":{"build":"echo build"}}`,
	})
	ctx := createNodeTestContext(t, a, nil)
	require.NoError(t, (&NodeProvider{}).Plan(ctx))
}

func TestValidateWorkspaceEntries_AbsolutePath(t *testing.T) {
	err := ValidateWorkspaceEntries([]string{"apps/*", "/usr/local/lib/x"})
	require.Error(t, err, "absolute paths must also be rejected")
	require.Contains(t, err.Error(), "/usr/local/lib/x")
}
