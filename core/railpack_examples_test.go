package core

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
)

// Tests against the railpack example projects — battle-tested real projects
// used in railpack's own integration test suite.

func railpackExamplesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "railpack", "examples")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}

func railpackExample(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(railpackExamplesDir(t), name)
}

func railpackPlan(t *testing.T, name string, opts *GenerateBuildPlanOptions) *BuildResult {
	t.Helper()
	userApp, err := app.NewApp(railpackExample(t, name))
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	if opts == nil {
		opts = &GenerateBuildPlanOptions{}
	}
	return GenerateBuildPlan(userApp, env, opts)
}

func railpackPlanWithEnv(t *testing.T, name string, envVars map[string]string, opts *GenerateBuildPlanOptions) *BuildResult {
	t.Helper()
	userApp, err := app.NewApp(railpackExample(t, name))
	require.NoError(t, err)
	env := app.NewEnvironment(&envVars)
	if opts == nil {
		opts = &GenerateBuildPlanOptions{}
	}
	return GenerateBuildPlan(userApp, env, opts)
}

// =============================================================================
// Node.js — railpack examples
// =============================================================================

func TestRailpack_NodeNpm(t *testing.T) {
	result := railpackPlan(t, "node-npm", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRailpack_NodeNpmWorkspaces(t *testing.T) {
	result := railpackPlan(t, "node-npm-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRailpack_NodeNpmWorkspaces_SubdirAPI(t *testing.T) {
	dir := filepath.Join(railpackExample(t, "node-npm-workspaces"), "packages", "api")
	userApp, err := app.NewApp(dir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	result := GenerateBuildPlan(userApp, env, &GenerateBuildPlanOptions{})

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

func TestRailpack_NodePnpmWorkspaces(t *testing.T) {
	result := railpackPlan(t, "node-pnpm-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRailpack_NodeYarnWorkspaces(t *testing.T) {
	result := railpackPlan(t, "node-yarn-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRailpack_NodeTurborepo(t *testing.T) {
	result := railpackPlan(t, "node-turborepo", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

func TestRailpack_NodeTurborepo_SubdirApps(t *testing.T) {
	base := railpackExample(t, "node-turborepo")

	t.Run("apps/web", func(t *testing.T) {
		userApp, err := app.NewApp(filepath.Join(base, "apps", "web"))
		require.NoError(t, err)
		result := GenerateBuildPlan(userApp, app.NewEnvironment(nil), &GenerateBuildPlanOptions{})
		assertValidPlan(t, result)
		require.Equal(t, "node", result.DetectedProviders[0])
	})
}

func TestRailpack_NodeBunWorkspaces(t *testing.T) {
	result := railpackPlan(t, "node-bun-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

// =============================================================================
// Go — railpack examples
// =============================================================================

func TestRailpack_GoMod(t *testing.T) {
	result := railpackPlan(t, "go-mod", nil)

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}

func TestRailpack_GoWorkspaces_RootFails(t *testing.T) {
	result := railpackPlan(t, "go-workspaces", nil)

	require.False(t, result.Success,
		"go.work root without go.mod should not match any provider")
}

func TestRailpack_GoWorkspaces_SubdirAPI(t *testing.T) {
	dir := filepath.Join(railpackExample(t, "go-workspaces"), "api")
	userApp, err := app.NewApp(dir)
	require.NoError(t, err)
	result := GenerateBuildPlan(userApp, app.NewEnvironment(nil), &GenerateBuildPlanOptions{})

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}

// =============================================================================
// Python — railpack examples
// =============================================================================

func TestRailpack_PythonFlask(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-flask", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn -w 4 -b 0.0.0.0:8000 main:app",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonDjango(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-django", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn mysite.wsgi:application",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonFastAPI(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-fastapi", map[string]string{
		"THEOPACKS_START_CMD": "uvicorn main:app --host 0.0.0.0",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonPip(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-pip", map[string]string{
		"THEOPACKS_START_CMD": "python app.py",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonPipfile(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-pipfile", map[string]string{
		"THEOPACKS_START_CMD": "python main.py",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonPoetry(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-poetry", map[string]string{
		"THEOPACKS_START_CMD": "flask run",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonUvWorkspace(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-uv-workspace", map[string]string{
		"THEOPACKS_START_CMD": "python main.py",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

func TestRailpack_PythonUvWorkspacePostgres(t *testing.T) {
	result := railpackPlanWithEnv(t, "python-uv-workspace-postgres", map[string]string{
		"THEOPACKS_START_CMD": "python main.py",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
}

// =============================================================================
// Shell and Static — railpack examples
// =============================================================================

func TestRailpack_ShellScript(t *testing.T) {
	result := railpackPlan(t, "shell-script", &GenerateBuildPlanOptions{
		StartCommand: "bash start.sh",
	})

	assertValidPlan(t, result)
	require.Equal(t, "shell", result.DetectedProviders[0])
}

func TestRailpack_StaticfileIndex(t *testing.T) {
	result := railpackPlan(t, "staticfile-index", &GenerateBuildPlanOptions{
		StartCommand: "python -m http.server 80",
	})

	assertValidPlan(t, result)
	require.Equal(t, "staticfile", result.DetectedProviders[0])
}

// =============================================================================
// Dockerignore — railpack example
// =============================================================================

func TestRailpack_Dockerignore(t *testing.T) {
	result := railpackPlan(t, "dockerignore", &GenerateBuildPlanOptions{
		StartCommand: "bash start.sh",
	})

	assertValidPlan(t, result)
	require.Equal(t, "true", result.Metadata["dockerIgnore"],
		"should detect .dockerignore from railpack example")
}

// =============================================================================
// Bulk: every railpack example that theo-packs can handle
// =============================================================================

func TestRailpack_AllSupportedExamples(t *testing.T) {
	cases := []struct {
		dir      string
		provider string
		env      map[string]string
		opts     *GenerateBuildPlanOptions
	}{
		// Node — core
		{"node-npm", "node", nil, nil},
		{"node-npm-workspaces", "node", nil, nil},
		{"node-pnpm-workspaces", "node", nil, nil},
		{"node-yarn-workspaces", "node", nil, nil},
		{"node-turborepo", "node", nil, nil},
		{"node-bun", "node", nil, nil},
		{"node-bun-workspaces", "node", nil, nil},
		{"node-next", "node", nil, nil},
		{"node-vite-react", "node", nil, nil},
		{"node-cra", "node", nil, nil},
		{"node-nuxt", "node", nil, nil},
		{"node-svelte-kit", "node", nil, nil},
		{"node-remix", "node", nil, nil},
		{"node-astro", "node", nil, nil},
		{"node-angular", "node", nil, nil},
		{"node-oldest", "node", nil, nil},
		{"node-prisma", "node", nil, nil},
		// Node — additional frameworks and configs
		{"node-astro-server", "node", nil, nil},
		{"node-bun-bunfig", "node", nil, nil},
		{"node-bun-no-deps", "node", nil, nil},
		{"node-corepack", "node", nil, nil},
		{"node-npm-install-in-build", "node", nil, nil},
		{"node-pnpm-engines", "node", nil, nil},
		{"node-puppeteer", "node", nil, nil},
		{"node-tanstack-start", "node", nil, nil},
		{"node-vite-react-router-spa", "node", nil, nil},
		{"node-vite-react-router-ssr", "node", nil, nil},
		{"node-vite-svelte", "node", nil, nil},
		{"node-vite-vanilla", "node", nil, nil},
		{"node-yarn-1", "node", nil, nil},
		{"node-yarn-2", "node", nil, nil},
		{"node-yarn-2-node-linker", "node", nil, nil},
		{"node-yarn-3", "node", nil, nil},
		{"node-yarn-4", "node", nil, nil},
		{"node-latest-npm-native-deps", "node", nil, nil},
		{"node-latest-pnpm-mise-native-deps", "node", nil, nil},
		{"bun-pnpm", "node", nil, nil},
		// Go
		{"go-mod", "go", nil, nil},
		{"go-cmd-dirs", "go", nil, nil},
		// Python — all variants
		{"python-flask", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-fastapi", "python", map[string]string{"THEOPACKS_START_CMD": "uvicorn main:app"}, nil},
		{"python-pip", "python", map[string]string{"THEOPACKS_START_CMD": "python app.py"}, nil},
		{"python-pipfile", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-poetry", "python", map[string]string{"THEOPACKS_START_CMD": "flask run"}, nil},
		{"python-uv-workspace", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-uv-workspace-postgres", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		// python-bot-only skipped: no requirements.txt/pyproject.toml/Pipfile/setup.py (railpack uses custom detection)
		{"python-compiled", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-fasthtml", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-latest", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-latest-psycopg", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-oldest", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-pdm", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-psycopg-binary", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-system-deps", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-uv", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-uv-packaged", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-uv-tool-versions", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		// Shell — all variants
		{"shell-script", "shell", nil, &GenerateBuildPlanOptions{StartCommand: "bash start.sh"}},
		{"shell-bash-arrays", "shell", nil, &GenerateBuildPlanOptions{StartCommand: "bash start.sh"}},
		{"shell-platform-arch", "shell", nil, &GenerateBuildPlanOptions{StartCommand: "bash start.sh"}},
		// Static
		{"staticfile-index", "staticfile", nil, &GenerateBuildPlanOptions{StartCommand: "python -m http.server"}},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			dir := railpackExample(t, tc.dir)
			userApp, err := app.NewApp(dir)
			if err != nil {
				t.Skipf("example %q not accessible: %v", tc.dir, err)
				return
			}

			var envPtr *map[string]string
			if tc.env != nil {
				envPtr = &tc.env
			}
			env := app.NewEnvironment(envPtr)

			opts := tc.opts
			if opts == nil {
				opts = &GenerateBuildPlanOptions{}
			}

			result := GenerateBuildPlan(userApp, env, opts)

			require.True(t, result.Success,
				"railpack example %q should produce a valid plan, logs: %v", tc.dir, result.Logs)
			require.Equal(t, tc.provider, result.DetectedProviders[0],
				"railpack example %q should detect provider %q", tc.dir, tc.provider)
			require.NotNil(t, result.Plan)
			require.NotEmpty(t, result.Plan.Steps)
		})
	}
}
