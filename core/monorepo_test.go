package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
)

// Tests against real example projects in ../examples/.
// Each example is a real, complete project structure — not synthetic temp dirs.

// --- Helpers ---

// examplesDir returns the absolute path to the shared examples directory.
func examplesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "failed to determine test file location")

	// core/monorepo_test.go -> theo-packs/core -> theo-packs -> theo -> examples
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "examples")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)

	_, err = os.Stat(abs)
	require.NoError(t, err, "examples directory not found at %s", abs)
	return abs
}

func examplePath(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(examplesDir(t), name)
	_, err := os.Stat(p)
	require.NoError(t, err, "example %q not found at %s", name, p)
	return p
}

func planFromExample(t *testing.T, name string, opts *GenerateBuildPlanOptions) *BuildResult {
	t.Helper()
	return planFromDir(t, examplePath(t, name), opts)
}

func planFromDir(t *testing.T, dir string, opts *GenerateBuildPlanOptions) *BuildResult {
	t.Helper()
	userApp, err := app.NewApp(dir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	if opts == nil {
		opts = &GenerateBuildPlanOptions{}
	}
	return GenerateBuildPlan(userApp, env, opts)
}

func planFromDirWithEnv(t *testing.T, dir string, envVars map[string]string, opts *GenerateBuildPlanOptions) *BuildResult {
	t.Helper()
	userApp, err := app.NewApp(dir)
	require.NoError(t, err)
	env := app.NewEnvironment(&envVars)
	if opts == nil {
		opts = &GenerateBuildPlanOptions{}
	}
	return GenerateBuildPlan(userApp, env, opts)
}

func planFromExampleWithEnv(t *testing.T, name string, envVars map[string]string, opts *GenerateBuildPlanOptions) *BuildResult {
	t.Helper()
	return planFromDirWithEnv(t, examplePath(t, name), envVars, opts)
}

// assertValidPlan checks structural invariants that every successful plan must satisfy.
func assertValidPlan(t *testing.T, result *BuildResult) {
	t.Helper()
	require.True(t, result.Success, "plan should succeed, logs: %v", result.Logs)
	require.NotNil(t, result.Plan)
	require.NotEmpty(t, result.Plan.Steps, "plan should have build steps")
	require.NotEmpty(t, result.Plan.Deploy.Base.Image, "plan should have a base deploy image")

	for _, step := range result.Plan.Steps {
		require.NotEmpty(t, step.Name, "every step should have a name")
		require.NotEmpty(t, step.Inputs, "every step should have inputs")
	}

	require.NotEmpty(t, result.Plan.Deploy.Inputs, "deploy should have inputs")
}

// =============================================================================
// Node.js — Single projects
// =============================================================================

func TestRealExample_NodeNpm(t *testing.T) {
	result := planFromExample(t, "node-npm", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeNext(t *testing.T) {
	result := planFromExample(t, "node-next", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeViteReact(t *testing.T) {
	result := planFromExample(t, "node-vite-react", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

// =============================================================================
// Node.js — Workspace monorepos
// =============================================================================

func TestRealExample_NodeNpmWorkspaces(t *testing.T) {
	result := planFromExample(t, "node-npm-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeNpmWorkspaces_SubdirAPI(t *testing.T) {
	dir := filepath.Join(examplePath(t, "node-npm-workspaces"), "packages", "api")
	result := planFromDir(t, dir, nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

func TestRealExample_NodePnpmWorkspaces(t *testing.T) {
	result := planFromExample(t, "node-pnpm-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeYarnWorkspaces(t *testing.T) {
	result := planFromExample(t, "node-yarn-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "npm start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeTurborepo(t *testing.T) {
	result := planFromExample(t, "node-turborepo", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

func TestRealExample_NodeTurborepo_WithBuildCommand(t *testing.T) {
	result := planFromExample(t, "node-turborepo", &GenerateBuildPlanOptions{
		BuildCommand: "turbo build",
		StartCommand: "cd apps/web && npm start",
	})

	assertValidPlan(t, result)
	require.Equal(t, "cd apps/web && npm start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeTurborepo_SubdirAPI(t *testing.T) {
	dir := filepath.Join(examplePath(t, "node-turborepo"), "apps", "api")
	result := planFromDir(t, dir, nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

func TestRealExample_NodeTurborepo_SubdirWeb(t *testing.T) {
	dir := filepath.Join(examplePath(t, "node-turborepo"), "apps", "web")
	result := planFromDir(t, dir, nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
}

// =============================================================================
// Go — Single and workspace projects
// =============================================================================

func TestRealExample_GoSimple(t *testing.T) {
	result := planFromExample(t, "go-simple", nil)

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}

func TestRealExample_GoCmdDirs(t *testing.T) {
	result := planFromExample(t, "go-cmd-dirs", nil)

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}

func TestRealExample_GoWorkspaces_RootFails(t *testing.T) {
	// go.work at root without go.mod — no provider should match
	result := planFromExample(t, "go-workspaces", nil)

	require.False(t, result.Success,
		"Go workspace root has go.work but no go.mod — should not match any provider")
}

func TestRealExample_GoWorkspaces_SubdirAPI(t *testing.T) {
	dir := filepath.Join(examplePath(t, "go-workspaces"), "api")
	result := planFromDir(t, dir, nil)

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}

func TestRealExample_GoWorkspaces_SubdirSharedFails(t *testing.T) {
	// shared/ has go.mod but no main package — Go provider still detects it
	// (it only checks for go.mod, not main package)
	dir := filepath.Join(examplePath(t, "go-workspaces"), "shared")
	result := planFromDir(t, dir, nil)

	require.True(t, result.Success, "shared module has go.mod, provider detects it")
	require.Equal(t, "go", result.DetectedProviders[0])
}

// =============================================================================
// Python — Various package managers
// =============================================================================

func TestRealExample_PythonFlask(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-flask", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn app:app --bind 0.0.0.0:8000",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "gunicorn app:app --bind 0.0.0.0:8000", result.Plan.Deploy.StartCmd)
}

func TestRealExample_PythonFastAPI(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-fastapi", map[string]string{
		"THEOPACKS_START_CMD": "uvicorn main:app --host 0.0.0.0 --port 8000",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "uvicorn main:app --host 0.0.0.0 --port 8000", result.Plan.Deploy.StartCmd)
}

func TestRealExample_PythonDjango(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-django", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn myproject.wsgi:application --bind 0.0.0.0:$PORT",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "gunicorn myproject.wsgi:application --bind 0.0.0.0:$PORT", result.Plan.Deploy.StartCmd)
}

func TestRealExample_PythonPoetry(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-poetry", map[string]string{
		"THEOPACKS_START_CMD": "flask run --host=0.0.0.0",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "flask run --host=0.0.0.0", result.Plan.Deploy.StartCmd)
}

func TestRealExample_PythonPipfile(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-pipfile", map[string]string{
		"THEOPACKS_START_CMD": "python app.py",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "python app.py", result.Plan.Deploy.StartCmd)
}

func TestRealExample_PythonUvWorkspace(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-uv-workspace", map[string]string{
		"THEOPACKS_START_CMD": "python main.py",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "python main.py", result.Plan.Deploy.StartCmd)
}

// =============================================================================
// Shell and Staticfile
// =============================================================================

func TestRealExample_ShellScript(t *testing.T) {
	result := planFromExample(t, "shell-script", &GenerateBuildPlanOptions{
		StartCommand: "bash start.sh",
	})

	assertValidPlan(t, result)
	require.Equal(t, "shell", result.DetectedProviders[0])
	require.Equal(t, "bash start.sh", result.Plan.Deploy.StartCmd)
}

func TestRealExample_Staticfile(t *testing.T) {
	result := planFromExample(t, "staticfile", &GenerateBuildPlanOptions{
		StartCommand: "python -m http.server 8080",
	})

	assertValidPlan(t, result)
	require.Equal(t, "staticfile", result.DetectedProviders[0])
	require.Equal(t, "python -m http.server 8080", result.Plan.Deploy.StartCmd)
}

// =============================================================================
// Full-stack mixed monorepo — each service independently
// =============================================================================

func TestRealExample_FullstackMixed_GoAPI(t *testing.T) {
	dir := filepath.Join(examplePath(t, "fullstack-mixed"), "services", "api")
	result := planFromDir(t, dir, nil)

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
}

func TestRealExample_FullstackMixed_NodeWeb(t *testing.T) {
	dir := filepath.Join(examplePath(t, "fullstack-mixed"), "services", "web")
	result := planFromDir(t, dir, &GenerateBuildPlanOptions{
		BuildCommand: "next build",
		StartCommand: "next start",
	})

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "next start", result.Plan.Deploy.StartCmd)
}

func TestRealExample_FullstackMixed_PythonWorker(t *testing.T) {
	dir := filepath.Join(examplePath(t, "fullstack-mixed"), "services", "worker")
	result := planFromDirWithEnv(t, dir, map[string]string{
		"THEOPACKS_START_CMD": "celery -A worker worker --loglevel=info",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "celery -A worker worker --loglevel=info", result.Plan.Deploy.StartCmd)
}

// =============================================================================
// Cross-cutting concerns
// =============================================================================

func TestRealExample_JSONRoundTrip(t *testing.T) {
	examples := []struct {
		name     string
		provider string
		env      map[string]string
		opts     *GenerateBuildPlanOptions
	}{
		{"node-npm", "node", nil, nil},
		{"go-simple", "go", nil, nil},
		{"python-flask", "python", map[string]string{"THEOPACKS_START_CMD": "gunicorn app:app"}, nil},
		{"shell-script", "shell", nil, &GenerateBuildPlanOptions{StartCommand: "bash start.sh"}},
		{"staticfile", "staticfile", nil, &GenerateBuildPlanOptions{StartCommand: "python -m http.server"}},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			var result *BuildResult
			if ex.env != nil {
				result = planFromExampleWithEnv(t, ex.name, ex.env, ex.opts)
			} else {
				result = planFromExample(t, ex.name, ex.opts)
			}

			require.True(t, result.Success, "logs: %v", result.Logs)

			jsonBytes, err := json.MarshalIndent(result, "", "  ")
			require.NoError(t, err)
			require.NotEmpty(t, jsonBytes)

			var roundTrip BuildResult
			require.NoError(t, json.Unmarshal(jsonBytes, &roundTrip))
			require.True(t, roundTrip.Success)
			require.Equal(t, ex.provider, roundTrip.DetectedProviders[0])
			require.NotNil(t, roundTrip.Plan)
			require.NotEmpty(t, roundTrip.Plan.Deploy.StartCmd)
		})
	}
}

func TestRealExample_AllProviders_PlanStructure(t *testing.T) {
	examples := []struct {
		name     string
		provider string
		env      map[string]string
		opts     *GenerateBuildPlanOptions
	}{
		{"node-npm", "node", nil, nil},
		{"node-npm-workspaces", "node", nil, nil},
		{"node-turborepo", "node", nil, nil},
		{"go-simple", "go", nil, nil},
		{"go-cmd-dirs", "go", nil, nil},
		{"python-flask", "python", map[string]string{"THEOPACKS_START_CMD": "gunicorn app:app"}, nil},
		{"python-django", "python", map[string]string{"THEOPACKS_START_CMD": "gunicorn myproject.wsgi:application"}, nil},
		{"python-uv-workspace", "python", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"shell-script", "shell", nil, &GenerateBuildPlanOptions{StartCommand: "bash start.sh"}},
		{"staticfile", "staticfile", nil, &GenerateBuildPlanOptions{StartCommand: "python -m http.server"}},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			var result *BuildResult
			if ex.env != nil {
				result = planFromExampleWithEnv(t, ex.name, ex.env, ex.opts)
			} else {
				result = planFromExample(t, ex.name, ex.opts)
			}

			assertValidPlan(t, result)
			require.Equal(t, ex.provider, result.DetectedProviders[0])
		})
	}
}

// =============================================================================
// Environment variable configuration with real projects
// =============================================================================

func TestRealExample_NodeNpm_CustomStartViaEnv(t *testing.T) {
	result := planFromExampleWithEnv(t, "node-npm", map[string]string{
		"THEOPACKS_START_CMD": "node dist/server.js",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "node dist/server.js", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeNpmWorkspaces_CustomBuildAndStartViaEnv(t *testing.T) {
	result := planFromExampleWithEnv(t, "node-npm-workspaces", map[string]string{
		"THEOPACKS_INSTALL_CMD": "npm ci",
		"THEOPACKS_BUILD_CMD":   "npm run build",
		"THEOPACKS_START_CMD":   "node packages/api/dist/index.js",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "node packages/api/dist/index.js", result.Plan.Deploy.StartCmd)
}

func TestRealExample_GoSimple_CustomStartViaOptions(t *testing.T) {
	result := planFromExample(t, "go-simple", &GenerateBuildPlanOptions{
		StartCommand: "/app/custom-binary",
	})

	assertValidPlan(t, result)
	require.Equal(t, "/app/custom-binary", result.Plan.Deploy.StartCmd)
}

// =============================================================================
// Version and metadata
// =============================================================================

func TestRealExample_TheopacksVersion(t *testing.T) {
	result := planFromExample(t, "node-npm", &GenerateBuildPlanOptions{
		TheopacksVersion: "0.3.0",
	})

	require.True(t, result.Success)
	require.Equal(t, "0.3.0", result.TheopacksVersion)
}

func TestRealExample_ProviderMetadata(t *testing.T) {
	result := planFromExample(t, "node-npm", nil)

	require.True(t, result.Success)
	require.NotNil(t, result.Metadata)
	require.Equal(t, "node", result.Metadata["providers"])
}
