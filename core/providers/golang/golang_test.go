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
