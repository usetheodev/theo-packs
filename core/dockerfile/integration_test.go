package dockerfile

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
)

func examplesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// core/dockerfile/integration_test.go -> core -> theo-packs/examples
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "examples")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}

func integrationPlan(t *testing.T, exampleName string, env map[string]string) string {
	t.Helper()
	dir := filepath.Join(examplesDir(t), exampleName)
	userApp, err := app.NewApp(dir)
	require.NoError(t, err)

	var envPtr *map[string]string
	if env != nil {
		envPtr = &env
	}
	appEnv := app.NewEnvironment(envPtr)

	opts := &core.GenerateBuildPlanOptions{}
	result := core.GenerateBuildPlan(userApp, appEnv, opts)
	require.True(t, result.Success, "GenerateBuildPlan failed for %s: %v", exampleName, result.Logs)
	require.NotNil(t, result.Plan)

	dockerfile, err := Generate(result.Plan)
	require.NoError(t, err, "Generate failed for %s", exampleName)
	return dockerfile
}

func goldenName(exampleName string) string {
	return "integration_" + strings.ReplaceAll(exampleName, "-", "_") + ".dockerfile"
}

func TestIntegration_AllExamples(t *testing.T) {
	cases := []struct {
		example string
		env     map[string]string
	}{
		// Go
		{"go-simple", nil},
		{"go-cmd-dirs", nil},
		{"go-workspaces", nil},
		// Node
		{"node-npm", nil},
		{"node-npm-workspaces", nil},
		{"node-npm-with-config", nil},
		{"node-npm-with-dockerignore", nil},
		{"node-pnpm-workspaces", nil},
		{"node-yarn-workspaces", nil},
		{"node-turborepo", nil},
		{"node-next", nil},
		{"node-vite-react", nil},
		{"node-vite-vue", nil},
		{"node-vite-svelte", nil},
		{"node-remix", nil},
		{"node-astro", nil},
		{"node-express", nil},
		{"node-nuxt", nil},
		// Python
		{"python-flask", map[string]string{"THEOPACKS_START_CMD": "gunicorn -w 4 app:app --bind 0.0.0.0:8000"}},
		{"python-fastapi", map[string]string{"THEOPACKS_START_CMD": "uvicorn main:app --host 0.0.0.0 --port 8000"}},
		{"python-django", map[string]string{"THEOPACKS_START_CMD": "gunicorn myproject.wsgi:application --bind 0.0.0.0:8000"}},
		{"python-pipfile", map[string]string{"THEOPACKS_START_CMD": "gunicorn -w 4 app:app --bind 0.0.0.0:8000"}},
		{"python-poetry", map[string]string{"THEOPACKS_START_CMD": "gunicorn -w 4 app:app --bind 0.0.0.0:8000"}},
		{"python-setuppy", map[string]string{"THEOPACKS_START_CMD": "myapp"}},
		{"python-uv-workspace", map[string]string{"THEOPACKS_START_CMD": "python main.py"}},
		{"python-streamlit", map[string]string{"THEOPACKS_START_CMD": "streamlit run app.py --server.port 8501 --server.address 0.0.0.0"}},
		{"python-gradio", map[string]string{"THEOPACKS_START_CMD": "python app.py"}},
		// Rust
		{"rust-axum", nil},
		{"rust-cli", nil},
		{"rust-workspace", map[string]string{"THEOPACKS_APP_NAME": "api"}},
		// Java
		{"java-spring-gradle", nil},
		{"java-spring-maven", nil},
		{"java-gradle-workspace", map[string]string{"THEOPACKS_APP_NAME": "api"}},
		// .NET
		{"dotnet-aspnet", nil},
		{"dotnet-console", nil},
		{"dotnet-solution", nil},
		// Ruby
		{"ruby-sinatra", nil},
		{"ruby-rails", nil},
		{"ruby-monorepo", map[string]string{"THEOPACKS_APP_NAME": "api"}},
		// PHP
		{"php-slim", nil},
		{"php-laravel", nil},
		{"php-monorepo", map[string]string{"THEOPACKS_APP_NAME": "api"}},
		// Deno
		{"deno-fresh", nil},
		{"deno-hono", nil},
		{"deno-workspace", map[string]string{"THEOPACKS_APP_NAME": "api"}},
		// Shell
		{"shell-script", map[string]string{"THEOPACKS_START_CMD": "bash start.sh"}},
		// Static
		{"staticfile", nil},
	}

	for _, tc := range cases {
		t.Run(tc.example, func(t *testing.T) {
			got := integrationPlan(t, tc.example, tc.env)
			assertGolden(t, goldenName(tc.example), got)
		})
	}
}

// Fullstack mixed: each service subdir is a separate service
func TestIntegration_FullstackMixed(t *testing.T) {
	base := filepath.Join(examplesDir(t), "fullstack-mixed", "services")

	subdirs := []struct {
		subdir string
		env    map[string]string
	}{
		{"api", nil},
		{"web", nil},
		{"worker", map[string]string{"THEOPACKS_START_CMD": "python worker.py"}},
	}

	for _, tc := range subdirs {
		t.Run(tc.subdir, func(t *testing.T) {
			dir := filepath.Join(base, tc.subdir)
			userApp, err := app.NewApp(dir)
			require.NoError(t, err)

			var envPtr *map[string]string
			if tc.env != nil {
				envPtr = &tc.env
			}
			appEnv := app.NewEnvironment(envPtr)

			result := core.GenerateBuildPlan(userApp, appEnv, &core.GenerateBuildPlanOptions{})
			require.True(t, result.Success, "GenerateBuildPlan failed for fullstack-mixed/services/%s: %v", tc.subdir, result.Logs)

			got, err := Generate(result.Plan)
			require.NoError(t, err)
			assertGolden(t, "integration_fullstack_"+tc.subdir+".dockerfile", got)
		})
	}
}
