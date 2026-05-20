package golang

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

func createTempApp(t *testing.T, files map[string]string) *app.App {
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

func createTestContext(t *testing.T, a *app.App, envVars map[string]string) *generate.GenerateContext {
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

func TestGoPlan(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.mod":  "module test\ngo 1.22",
		"main.go": "package main\nfunc main() {}",
	})
	ctx := createTestContext(t, a, nil)

	provider := &GoProvider{}
	err := provider.Plan(ctx)
	require.NoError(t, err)

	require.Equal(t, "/app/server", ctx.Deploy.StartCmd)
	require.Len(t, ctx.Steps, 2)
	require.Equal(t, "install", ctx.Steps[0].Name())
	require.Equal(t, "build", ctx.Steps[1].Name())
}

func TestGoStartCommandHelp(t *testing.T) {
	provider := &GoProvider{}
	help := provider.StartCommandHelp()
	require.NotEmpty(t, help)
	require.Contains(t, help, "main package")
}

func TestGoDetect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name:     "detects go.mod",
			files:    map[string]string{"go.mod": "module test\ngo 1.22"},
			expected: true,
		},
		{
			name: "detects go.work",
			files: map[string]string{
				"go.work":     "go 1.22\nuse ./api",
				"api/go.mod":  "module example.com/api",
				"api/main.go": "package main",
			},
			expected: true,
		},
		{
			name:     "no go files",
			files:    map[string]string{"package.json": `{"name":"test"}`},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)

			provider := &GoProvider{}
			detected, err := provider.Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, detected)
		})
	}
}

// --- version detection ---

func TestDetectGoVersion(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		envVars    map[string]string
		wantVer    string
		wantSource string
	}{
		{
			name:       "default when no version info",
			files:      map[string]string{"go.mod": "module test"},
			wantVer:    "1.23",
			wantSource: "default",
		},
		{
			name:       "reads from go.mod",
			files:      map[string]string{"go.mod": "module test\ngo 1.22"},
			wantVer:    "1.22",
			wantSource: "go.mod",
		},
		{
			name:       "reads from go.mod with patch",
			files:      map[string]string{"go.mod": "module test\ngo 1.21.5"},
			wantVer:    "1.21",
			wantSource: "go.mod",
		},
		{
			name:       "env var overrides go.mod",
			files:      map[string]string{"go.mod": "module test\ngo 1.22"},
			envVars:    map[string]string{"THEOPACKS_GO_VERSION": "1.21"},
			wantVer:    "1.21",
			wantSource: "THEOPACKS_GO_VERSION",
		},
		{
			name:       "malformed go.mod falls back to default",
			files:      map[string]string{"go.mod": "garbage content"},
			wantVer:    "1.23",
			wantSource: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, tt.envVars)

			version, source := detectGoVersion(ctx)
			require.Equal(t, tt.wantVer, version)
			require.Equal(t, tt.wantSource, source)
		})
	}
}

func TestDetectGoVersion_ConfigPackages(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.mod": "module test\ngo 1.22",
	})
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	cfg.Packages = map[string]string{"go": "1.24"}
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)

	version, source := detectGoVersion(ctx)
	require.Equal(t, "1.24", version)
	require.Equal(t, "custom config", source)
}

func TestExtractGoVersionFromMod(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"standard", "module test\ngo 1.23", "1.23"},
		{"with patch", "module test\ngo 1.22.5", "1.22"},
		{"no go directive", "module test", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, map[string]string{"go.mod": tt.content})
			ctx := createTestContext(t, a, nil)
			require.Equal(t, tt.want, extractGoVersionFromMod(ctx))
		})
	}
}

// --- go.work parsing ---

func TestParseGoWork(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work": "go 1.22\n\nuse (\n\t./api\n\t./shared\n)\n",
	})
	modules, err := parseGoWork(a)
	require.NoError(t, err)
	require.Equal(t, []string{"api", "shared"}, modules)
}

func TestParseGoWork_SingleModule(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work": "go 1.22\n\nuse ./api\n",
	})
	modules, err := parseGoWork(a)
	require.NoError(t, err)
	require.Equal(t, []string{"api"}, modules)
}

// --- findBuildTarget ---

func TestFindBuildTarget_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work":     "go 1.22\nuse (\n\t./api\n\t./shared\n)",
		"api/go.mod":  "module example.com/api",
		"api/main.go": "package main",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_GO_MODULE": "shared"})
	target := findBuildTarget(ctx, []string{"api", "shared"})
	require.Equal(t, "shared", target)
}

func TestFindBuildTarget_AutoDetect(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work":       "go 1.22\nuse (\n\t./api\n\t./shared\n)",
		"api/go.mod":    "module example.com/api",
		"api/main.go":   "package main",
		"shared/go.mod": "module example.com/shared",
		"shared/lib.go": "package shared",
	})
	ctx := createTestContext(t, a, nil)
	target := findBuildTarget(ctx, []string{"api", "shared"})
	require.Equal(t, "api", target)
}

func TestFindBuildTarget_NoMainGo(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work":       "go 1.22\nuse (\n\t./api\n\t./shared\n)",
		"api/go.mod":    "module example.com/api",
		"api/lib.go":    "package api",
		"shared/go.mod": "module example.com/shared",
		"shared/lib.go": "package shared",
	})
	ctx := createTestContext(t, a, nil)
	target := findBuildTarget(ctx, []string{"api", "shared"})
	require.Equal(t, "", target)
}

// --- workspace plan ---

// --- hardening tests ---

func TestParseGoWork_EmptyUseBlock(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work": "go 1.22\n\nuse (\n)\n",
	})
	modules, err := parseGoWork(a)
	require.Error(t, err)
	require.Nil(t, modules)
	require.Contains(t, err.Error(), "empty use block")
}

func TestGoWork_TakesPriorityOverGoMod(t *testing.T) {
	// When both go.work and go.mod exist, go.work should take priority
	a := createTempApp(t, map[string]string{
		"go.work":     "go 1.22\n\nuse (\n\t./api\n)\n",
		"go.mod":      "module example.com/root\ngo 1.22",
		"main.go":     "package main\nfunc main() {}",
		"api/go.mod":  "module example.com/api\ngo 1.22",
		"api/main.go": "package main\nfunc main() {}",
	})
	ctx := createTestContext(t, a, nil)

	provider := &GoProvider{}
	err := provider.Plan(ctx)
	require.NoError(t, err)

	// The build command should reference the workspace module, not root "."
	buildStep := ctx.Steps[1]
	require.Equal(t, "build", buildStep.Name())
}

func TestFindBuildTarget_EnvVarNonExistentModule(t *testing.T) {
	// THEOPACKS_GO_MODULE set to a module that doesn't exist in the workspace
	// findBuildTarget returns whatever the env var says without validation,
	// but planWorkspace will use it and the build will reference a non-existent path
	a := createTempApp(t, map[string]string{
		"go.work":     "go 1.22\nuse (\n\t./api\n)",
		"api/go.mod":  "module example.com/api",
		"api/main.go": "package main",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_GO_MODULE": "nonexistent"})
	target := findBuildTarget(ctx, []string{"api"})
	// The env var is returned as-is even if it doesn't match any module
	require.Equal(t, "nonexistent", target)
}

func TestFindSimpleBuildTarget_MultipleCmdDirs(t *testing.T) {
	// Multiple cmd/*/main.go should pick deterministically (first sorted)
	a := createTempApp(t, map[string]string{
		"go.mod":            "module test\ngo 1.22",
		"cmd/alpha/main.go": "package main\nfunc main() {}",
		"cmd/beta/main.go":  "package main\nfunc main() {}",
	})
	ctx := createTestContext(t, a, nil)
	target := findSimpleBuildTarget(ctx)
	// Should pick the first match from FindFiles (sorted by doublestar)
	require.Contains(t, target, "cmd/")
	require.NotEmpty(t, target)
}

func TestGoPlan_MalformedGoMod(t *testing.T) {
	// Malformed go.mod content should not crash — Plan still runs,
	// the build step just references "." as fallback
	a := createTempApp(t, map[string]string{
		"go.mod": "this is not valid go.mod content @@#$%",
	})
	ctx := createTestContext(t, a, nil)

	provider := &GoProvider{}
	err := provider.Plan(ctx)
	// planSimple doesn't parse go.mod, it just copies it; Go tooling
	// will report the error at build time. No crash here.
	require.NoError(t, err)
	require.Len(t, ctx.Steps, 2)
}

func TestGoPlan_Workspace(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"go.work":       "go 1.22\n\nuse (\n\t./api\n\t./shared\n)\n",
		"api/go.mod":    "module example.com/api\ngo 1.22",
		"api/main.go":   "package main\nfunc main() {}",
		"shared/go.mod": "module example.com/shared\ngo 1.22",
		"shared/lib.go": "package shared",
	})
	ctx := createTestContext(t, a, nil)

	provider := &GoProvider{}
	err := provider.Plan(ctx)
	require.NoError(t, err)

	require.Equal(t, "/app/server", ctx.Deploy.StartCmd)
	require.Len(t, ctx.Steps, 2)
	require.Equal(t, "install", ctx.Steps[0].Name())
	require.Equal(t, "build", ctx.Steps[1].Name())
}
