package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
)

func repoExamplesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "examples")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}

// expectedProvider returns the provider name we expect for a given example
// directory name. The convention is: the prefix before the first hyphen
// maps to the provider, with a few special cases handled explicitly.
//
// EXHAUSTIVE — every provider registered in providers.GetLanguageProviders
// must have a case here. When a new provider lands and a corresponding
// example is added under examples/, the TestIntegrationExamples loop will
// fail loudly with "no expected provider mapping" instead of silently
// skipping (T0.1, test-suite-hardening-plan).
func expectedProvider(dirName string) string {
	switch {
	case strings.HasPrefix(dirName, "go-"):
		return "go"
	case strings.HasPrefix(dirName, "node-"):
		return "node"
	case strings.HasPrefix(dirName, "python-"):
		return "python"
	case strings.HasPrefix(dirName, "rust-"):
		return "rust"
	case strings.HasPrefix(dirName, "java-"):
		return "java"
	case strings.HasPrefix(dirName, "dotnet-"):
		return "dotnet"
	case strings.HasPrefix(dirName, "ruby-"):
		return "ruby"
	case strings.HasPrefix(dirName, "php-"):
		return "php"
	case strings.HasPrefix(dirName, "deno-"):
		return "deno"
	case dirName == "staticfile":
		return "staticfile"
	case dirName == "shell-script":
		return "shell"
	default:
		return ""
	}
}

func TestIntegrationExamples(t *testing.T) {
	exDir := repoExamplesDir(t)

	entries, err := os.ReadDir(exDir)
	require.NoError(t, err)

	// Collect top-level example directories. The fullstack-mixed example
	// contains multiple services in subdirectories; we expand those into
	// separate subtests.
	type exampleCase struct {
		name     string // subtest name
		dir      string // absolute path to the project
		provider string // expected detected provider
		startCmd string // optional start command to supply
		env      map[string]string // optional env vars (e.g. THEOPACKS_APP_NAME for workspace examples)
	}

	// workspaceTargets maps the multi-member workspace examples to a chosen
	// build target. Without an explicit selection the provider hard-fails
	// with "set THEOPACKS_APP_NAME to one of: ..." — which is intentional
	// behavior, but means the integration test must supply the env hint to
	// drive the realistic happy path. (T0.1)
	workspaceTargets := map[string]string{
		"rust-workspace":         "api",
		"java-gradle-workspace":  "api",
		"deno-workspace":         "api",
		"php-monorepo":           "api",
		"ruby-monorepo":          "api",
	}

	var cases []exampleCase

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		dirPath := filepath.Join(exDir, dirName)

		if dirName == "fullstack-mixed" {
			// Expand each service as its own subtest.
			services := []struct {
				subdir   string
				provider string
				startCmd string
			}{
				{"services/api", "go", ""},
				{"services/web", "node", ""},
				{"services/worker", "python", ""},
			}
			for _, svc := range services {
				cases = append(cases, exampleCase{
					name:     "fullstack-mixed/" + svc.subdir,
					dir:      filepath.Join(dirPath, svc.subdir),
					provider: svc.provider,
					startCmd: svc.startCmd,
				})
			}
			continue
		}

		prov := expectedProvider(dirName)
		require.NotEmpty(t, prov,
			"example %q has no entry in expectedProvider() — add a case for it (T0.1, test-suite-hardening-plan)",
			dirName)

		c := exampleCase{
			name:     dirName,
			dir:      dirPath,
			provider: prov,
		}
		if target, ok := workspaceTargets[dirName]; ok {
			c.env = map[string]string{"THEOPACKS_APP_NAME": target}
		}
		cases = append(cases, c)
	}

	require.NotEmpty(t, cases, "no example directories found")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userApp, err := app.NewApp(tc.dir)
			require.NoError(t, err)

			var envPtr *map[string]string
			if tc.env != nil {
				envPtr = &tc.env
			}
			env := app.NewEnvironment(envPtr)

			opts := &GenerateBuildPlanOptions{}
			if tc.startCmd != "" {
				opts.StartCommand = tc.startCmd
			}

			result := GenerateBuildPlan(userApp, env, opts)

			require.True(t, result.Success,
				"GenerateBuildPlan should succeed for %s, logs: %v", tc.name, result.Logs)

			require.NotNil(t, result.Plan,
				"Plan should be non-nil for %s", tc.name)

			require.GreaterOrEqual(t, len(result.Plan.Steps), 1,
				"Plan should have at least 1 step for %s", tc.name)

			require.NotEmpty(t, result.DetectedProviders,
				"DetectedProviders should be non-empty for %s", tc.name)

			require.Equal(t, tc.provider, result.DetectedProviders[0],
				"detected provider mismatch for %s", tc.name)
		})
	}
}
