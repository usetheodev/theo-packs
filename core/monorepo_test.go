package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/plan"
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

func TestRealExample_GoWorkspaces_Root(t *testing.T) {
	// go.work at root — Go provider detects workspace and auto-targets module with main.go
	result := planFromExample(t, "go-workspaces", nil)

	assertValidPlan(t, result)
	require.Equal(t, "go", result.DetectedProviders[0])
	require.Equal(t, "/app/server", result.Plan.Deploy.StartCmd)
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

// =============================================================================
// Deep build plan validation — verify steps, commands, and layers
// =============================================================================

func findStepByName(result *BuildResult, name string) *plan.Step {
	for i := range result.Plan.Steps {
		if result.Plan.Steps[i].Name == name {
			return &result.Plan.Steps[i]
		}
	}
	return nil
}

func hasExecCommand(step *plan.Step, substr string) bool {
	for _, cmd := range step.Commands {
		if exec, ok := cmd.(plan.ExecCommand); ok {
			if strings.Contains(exec.Cmd, substr) {
				return true
			}
		}
	}
	return false
}

func TestRealExample_NodeNpm_PlanSteps(t *testing.T) {
	result := planFromExample(t, "node-npm", nil)
	assertValidPlan(t, result)

	// Node provider creates "install" and "build" steps
	installStep := findStepByName(result, "install")
	require.NotNil(t, installStep, "should have an install step")
	require.True(t, hasExecCommand(installStep, "npm ci") || hasExecCommand(installStep, "npm install"),
		"install step should run npm ci or npm install")

	buildStep := findStepByName(result, "build")
	require.NotNil(t, buildStep, "should have a build step")

	// Build step should reference install step as input
	hasInstallInput := false
	for _, input := range buildStep.Inputs {
		if input.Step == "install" {
			hasInstallInput = true
			break
		}
	}
	require.True(t, hasInstallInput, "build step should depend on install step")
}

func TestRealExample_GoSimple_PlanSteps(t *testing.T) {
	result := planFromExample(t, "go-simple", nil)
	assertValidPlan(t, result)

	// Go provider creates single "build" step
	buildStep := findStepByName(result, "build")
	require.NotNil(t, buildStep, "should have a build step")
	require.True(t, hasExecCommand(buildStep, "go build"),
		"build step should run go build")

	// Deploy should reference the binary from build step
	hasBuildInput := false
	for _, input := range result.Plan.Deploy.Inputs {
		if input.Step == "build" {
			hasBuildInput = true
			break
		}
	}
	require.True(t, hasBuildInput, "deploy should reference build step output")
}

func TestRealExample_PythonFlask_PlanSteps(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-flask", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn app:app",
	}, nil)
	assertValidPlan(t, result)

	installStep := findStepByName(result, "install")
	require.NotNil(t, installStep, "should have an install step")
	require.True(t, hasExecCommand(installStep, "pip install --no-cache-dir -r requirements.txt"),
		"install step should run pip install for requirements.txt")
}

func TestRealExample_PythonPipfile_PlanSteps(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-pipfile", map[string]string{
		"THEOPACKS_START_CMD": "python app.py",
	}, nil)
	assertValidPlan(t, result)

	installStep := findStepByName(result, "install")
	require.NotNil(t, installStep, "should have an install step")
	require.True(t, hasExecCommand(installStep, "pipenv requirements"),
		"install step should convert Pipfile to requirements via pipenv requirements")
}

func TestRealExample_PythonPoetry_PlanSteps(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-poetry", map[string]string{
		"THEOPACKS_START_CMD": "flask run",
	}, nil)
	assertValidPlan(t, result)

	installStep := findStepByName(result, "install")
	require.NotNil(t, installStep, "should have an install step")
	require.True(t, hasExecCommand(installStep, "poetry install"),
		"install step should run poetry install for Poetry projects")
}

func TestRealExample_PythonDjango_PlanSteps(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-django", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn myproject.wsgi:application",
	}, nil)
	assertValidPlan(t, result)

	installStep := findStepByName(result, "install")
	require.NotNil(t, installStep, "should have an install step")
	require.True(t, hasExecCommand(installStep, "pip install --no-cache-dir -r requirements.txt"),
		"Django with requirements.txt should use pip install -r")
}

func TestRealExample_ShellScript_PlanSteps(t *testing.T) {
	result := planFromExample(t, "shell-script", &GenerateBuildPlanOptions{
		StartCommand: "bash start.sh",
	})
	assertValidPlan(t, result)

	buildStep := findStepByName(result, "build")
	require.NotNil(t, buildStep, "should have a build step")

	// Shell provider copies files
	hasCopyCmd := false
	for _, cmd := range buildStep.Commands {
		if cmd.CommandType() == "copy" {
			hasCopyCmd = true
			break
		}
	}
	require.True(t, hasCopyCmd, "shell build step should copy files")
}

func TestRealExample_Staticfile_PlanSteps(t *testing.T) {
	result := planFromExample(t, "staticfile", &GenerateBuildPlanOptions{
		StartCommand: "python -m http.server 8080",
	})
	assertValidPlan(t, result)

	buildStep := findStepByName(result, "build")
	require.NotNil(t, buildStep, "should have a build step")

	hasCopyCmd := false
	for _, cmd := range buildStep.Commands {
		if cmd.CommandType() == "copy" {
			hasCopyCmd = true
			break
		}
	}
	require.True(t, hasCopyCmd, "staticfile build step should copy files")
}

// =============================================================================
// Deploy layer chain integrity — verify no broken step references
// =============================================================================

func TestRealExample_DeployLayerChainIntegrity(t *testing.T) {
	examples := []struct {
		name string
		env  map[string]string
		opts *GenerateBuildPlanOptions
	}{
		{"node-npm", nil, nil},
		{"node-npm-workspaces", nil, nil},
		{"node-turborepo", nil, nil},
		{"go-simple", nil, nil},
		{"go-cmd-dirs", nil, nil},
		{"python-flask", map[string]string{"THEOPACKS_START_CMD": "gunicorn app:app"}, nil},
		{"python-django", map[string]string{"THEOPACKS_START_CMD": "gunicorn wsgi:app"}, nil},
		{"python-uv-workspace", map[string]string{"THEOPACKS_START_CMD": "python main.py"}, nil},
		{"python-pipfile", map[string]string{"THEOPACKS_START_CMD": "python app.py"}, nil},
		{"python-poetry", map[string]string{"THEOPACKS_START_CMD": "flask run"}, nil},
		{"shell-script", nil, &GenerateBuildPlanOptions{StartCommand: "bash start.sh"}},
		{"staticfile", nil, &GenerateBuildPlanOptions{StartCommand: "python -m http.server"}},
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

			// Build step name index
			stepNames := make(map[string]bool)
			for _, step := range result.Plan.Steps {
				stepNames[step.Name] = true
			}

			// Every deploy input that references a step must reference an existing step
			for _, input := range result.Plan.Deploy.Inputs {
				if input.Step != "" {
					require.True(t, stepNames[input.Step],
						"deploy input references step %q which doesn't exist in plan steps: %v",
						input.Step, stepNames)
				}
			}

			// Every step input that references another step must reference an existing step
			for _, step := range result.Plan.Steps {
				for _, input := range step.Inputs {
					if input.Step != "" {
						require.True(t, stepNames[input.Step],
							"step %q input references step %q which doesn't exist",
							step.Name, input.Step)
					}
				}
			}
		})
	}
}

// =============================================================================
// Monorepo subdirectory isolation — services don't leak into each other
// =============================================================================

func TestRealExample_FullstackMixed_ServiceIsolation(t *testing.T) {
	// Each service in fullstack-mixed should produce a plan
	// independent of sibling services.

	t.Run("api_does_not_detect_node", func(t *testing.T) {
		dir := filepath.Join(examplePath(t, "fullstack-mixed"), "services", "api")
		result := planFromDir(t, dir, nil)

		assertValidPlan(t, result)
		require.Equal(t, "go", result.DetectedProviders[0],
			"Go API should not be confused by Node.js sibling")
	})

	t.Run("web_does_not_detect_go", func(t *testing.T) {
		dir := filepath.Join(examplePath(t, "fullstack-mixed"), "services", "web")
		result := planFromDir(t, dir, nil)

		assertValidPlan(t, result)
		require.Equal(t, "node", result.DetectedProviders[0],
			"Node web should not be confused by Go sibling")
	})

	t.Run("worker_does_not_detect_node_or_go", func(t *testing.T) {
		dir := filepath.Join(examplePath(t, "fullstack-mixed"), "services", "worker")
		result := planFromDirWithEnv(t, dir, map[string]string{
			"THEOPACKS_START_CMD": "celery worker",
		}, nil)

		assertValidPlan(t, result)
		require.Equal(t, "python", result.DetectedProviders[0],
			"Python worker should not be confused by Go/Node siblings")
	})
}

// =============================================================================
// Workspace subdirectory deploys — verify individual packages work
// =============================================================================

func TestRealExample_NodeNpmWorkspaces_EachPackage(t *testing.T) {
	base := examplePath(t, "node-npm-workspaces")

	t.Run("root", func(t *testing.T) {
		result := planFromDir(t, base, nil)
		assertValidPlan(t, result)
		require.Equal(t, "node", result.DetectedProviders[0])
	})

	t.Run("packages/api", func(t *testing.T) {
		dir := filepath.Join(base, "packages", "api")
		result := planFromDir(t, dir, nil)
		assertValidPlan(t, result)
		require.Equal(t, "node", result.DetectedProviders[0])
	})

	t.Run("packages/shared", func(t *testing.T) {
		dir := filepath.Join(base, "packages", "shared")
		result := planFromDir(t, dir, nil)
		assertValidPlan(t, result)
		require.Equal(t, "node", result.DetectedProviders[0])
	})
}

func TestRealExample_GoWorkspaces_EachModule(t *testing.T) {
	base := examplePath(t, "go-workspaces")

	t.Run("root_workspace", func(t *testing.T) {
		result := planFromDir(t, base, nil)
		assertValidPlan(t, result)
		require.Equal(t, "go", result.DetectedProviders[0])
	})

	t.Run("api", func(t *testing.T) {
		dir := filepath.Join(base, "api")
		result := planFromDir(t, dir, nil)
		assertValidPlan(t, result)
		require.Equal(t, "go", result.DetectedProviders[0])
	})

	t.Run("shared", func(t *testing.T) {
		dir := filepath.Join(base, "shared")
		result := planFromDir(t, dir, nil)
		require.True(t, result.Success)
		require.Equal(t, "go", result.DetectedProviders[0])
	})
}

func TestRealExample_PythonUvWorkspace_EachMember(t *testing.T) {
	base := examplePath(t, "python-uv-workspace")

	t.Run("root", func(t *testing.T) {
		result := planFromDirWithEnv(t, base, map[string]string{
			"THEOPACKS_START_CMD": "python main.py",
		}, nil)
		assertValidPlan(t, result)
		require.Equal(t, "python", result.DetectedProviders[0])
	})

	t.Run("workspace-package", func(t *testing.T) {
		dir := filepath.Join(base, "workspace-package")
		result := planFromDirWithEnv(t, dir, map[string]string{
			"THEOPACKS_START_CMD": "python -m workspace_package",
		}, nil)
		assertValidPlan(t, result)
		require.Equal(t, "python", result.DetectedProviders[0])
	})
}

func TestRealExample_NodeTurborepo_EachApp(t *testing.T) {
	base := examplePath(t, "node-turborepo")

	t.Run("root", func(t *testing.T) {
		result := planFromDir(t, base, nil)
		assertValidPlan(t, result)
	})

	t.Run("apps/api", func(t *testing.T) {
		dir := filepath.Join(base, "apps", "api")
		result := planFromDir(t, dir, nil)
		assertValidPlan(t, result)
	})

	t.Run("apps/web", func(t *testing.T) {
		dir := filepath.Join(base, "apps", "web")
		result := planFromDir(t, dir, nil)
		assertValidPlan(t, result)
	})

	// packages/ui and packages/utils have package.json, so node detects them
	t.Run("packages/ui", func(t *testing.T) {
		dir := filepath.Join(base, "packages", "ui")
		result := planFromDir(t, dir, nil)
		assertValidPlan(t, result)
		require.Equal(t, "node", result.DetectedProviders[0])
	})
}

// =============================================================================
// Config precedence with real examples
// =============================================================================

func TestRealExample_NodeNpm_EnvOverridesProvider(t *testing.T) {
	// Provider sets "npm start", env should override
	result := planFromExampleWithEnv(t, "node-npm", map[string]string{
		"THEOPACKS_START_CMD": "node custom-entry.js",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "node custom-entry.js", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeNpm_OptionsOverrideEnv(t *testing.T) {
	result := planFromExampleWithEnv(t, "node-npm", map[string]string{
		"THEOPACKS_START_CMD": "from-env",
	}, &GenerateBuildPlanOptions{
		StartCommand: "from-options",
	})

	assertValidPlan(t, result)
	require.Equal(t, "from-options", result.Plan.Deploy.StartCmd,
		"options should take highest precedence over env")
}

func TestRealExample_GoSimple_OptionsOverrideProvider(t *testing.T) {
	result := planFromExample(t, "go-simple", &GenerateBuildPlanOptions{
		StartCommand: "/app/custom",
	})

	assertValidPlan(t, result)
	require.Equal(t, "/app/custom", result.Plan.Deploy.StartCmd,
		"options should override Go provider's default /app/server")
}

// =============================================================================
// Build and install commands via env on real workspaces
// =============================================================================

func TestRealExample_NodeTurborepo_FullEnvConfig(t *testing.T) {
	result := planFromExampleWithEnv(t, "node-turborepo", map[string]string{
		"THEOPACKS_INSTALL_CMD":         "npm ci",
		"THEOPACKS_BUILD_CMD":           "turbo build --filter=web",
		"THEOPACKS_START_CMD":           "cd apps/web && next start",
		"THEOPACKS_BUILD_APT_PACKAGES":  "build-essential",
		"THEOPACKS_DEPLOY_APT_PACKAGES": "libssl-dev",
	}, nil)

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.NotNil(t, result.Plan)
	require.Equal(t, "cd apps/web && next start", result.Plan.Deploy.StartCmd)
	require.NotEmpty(t, result.Plan.Steps, "should have build steps with apt packages")
}

func TestRealExample_PythonFlask_AptPackagesViaEnv(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-flask", map[string]string{
		"THEOPACKS_START_CMD":           "gunicorn app:app",
		"THEOPACKS_DEPLOY_APT_PACKAGES": "libpq-dev libssl-dev",
	}, nil)

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.NotNil(t, result.Plan)
	require.Equal(t, "gunicorn app:app", result.Plan.Deploy.StartCmd)
	require.NotEmpty(t, result.Plan.Steps, "should have build steps with apt packages")
}

// =============================================================================
// theopacks.json config file with real examples
// =============================================================================

func TestRealExample_NodeWithConfig(t *testing.T) {
	result := planFromExample(t, "node-npm-with-config", nil)

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.NotNil(t, result.Plan)
	require.Equal(t, "node server.js", result.Plan.Deploy.StartCmd,
		"theopacks.json startCommand should override provider default")
}

func TestRealExample_NodeWithConfig_EnvOverridesConfigFile(t *testing.T) {
	result := planFromExampleWithEnv(t, "node-npm-with-config", map[string]string{
		"THEOPACKS_START_CMD": "from-env",
	}, nil)

	require.True(t, result.Success, "logs: %v", result.Logs)
	// Env should override config file but not options
	require.Equal(t, "from-env", result.Plan.Deploy.StartCmd)
}

func TestRealExample_NodeWithConfig_OptionsOverrideAll(t *testing.T) {
	result := planFromExampleWithEnv(t, "node-npm-with-config", map[string]string{
		"THEOPACKS_START_CMD": "from-env",
	}, &GenerateBuildPlanOptions{
		StartCommand: "from-options",
	})

	require.True(t, result.Success, "logs: %v", result.Logs)
	require.Equal(t, "from-options", result.Plan.Deploy.StartCmd,
		"options > env > config file > provider default")
}

// =============================================================================
// .dockerignore with real examples
// =============================================================================

func TestRealExample_NodeWithDockerignore(t *testing.T) {
	result := planFromExample(t, "node-npm-with-dockerignore", nil)

	assertValidPlan(t, result)
	require.Equal(t, "node", result.DetectedProviders[0])
	require.Equal(t, "true", result.Metadata["dockerIgnore"],
		"should detect .dockerignore file")
}

func TestRealExample_NodeWithDockerignore_PlanSteps(t *testing.T) {
	result := planFromExample(t, "node-npm-with-dockerignore", nil)

	assertValidPlan(t, result)

	// Build step should have local layer with dockerignore excludes
	buildStep := findStepByName(result, "build")
	require.NotNil(t, buildStep)

	hasLocalInput := false
	for _, input := range buildStep.Inputs {
		if input.Local {
			hasLocalInput = true
			// Dockerignore should add excludes to the local layer
			require.NotEmpty(t, input.Exclude,
				"local layer should have dockerignore excludes applied")
			break
		}
	}
	require.True(t, hasLocalInput, "build step should have a local input with dockerignore")
}

// =============================================================================
// Python setup.py (legacy) with real example
// =============================================================================

func TestRealExample_PythonSetupPy(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-setuppy", map[string]string{
		"THEOPACKS_START_CMD": "gunicorn myapp.app:app --bind 0.0.0.0:8000",
	}, nil)

	assertValidPlan(t, result)
	require.Equal(t, "python", result.DetectedProviders[0])
	require.Equal(t, "gunicorn myapp.app:app --bind 0.0.0.0:8000", result.Plan.Deploy.StartCmd)
}

func TestRealExample_PythonSetupPy_PlanSteps(t *testing.T) {
	result := planFromExampleWithEnv(t, "python-setuppy", map[string]string{
		"THEOPACKS_START_CMD": "myapp",
	}, nil)

	assertValidPlan(t, result)

	installStep := findStepByName(result, "install")
	require.NotNil(t, installStep, "should have an install step")
	require.True(t, hasExecCommand(installStep, "pip install --no-cache-dir ."),
		"setup.py project should use pip install .")
}
