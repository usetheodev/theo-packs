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
// directory name. The convention is: the prefix before the first hyphen maps
// to the provider, with a few special cases handled explicitly.
func expectedProvider(dirName string) string {
	switch {
	case strings.HasPrefix(dirName, "go-"):
		return "go"
	case strings.HasPrefix(dirName, "node-"):
		return "node"
	case strings.HasPrefix(dirName, "python-"):
		return "python"
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
		name      string // subtest name
		dir       string // absolute path to the project
		provider  string // expected detected provider
		startCmd  string // optional start command to supply
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
		if prov == "" {
			t.Logf("skipping unknown example %q (no expected provider mapping)", dirName)
			continue
		}

		cases = append(cases, exampleCase{
			name:     dirName,
			dir:      dirPath,
			provider: prov,
		})
	}

	require.NotEmpty(t, cases, "no example directories found")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userApp, err := app.NewApp(tc.dir)
			require.NoError(t, err)

			env := app.NewEnvironment(nil)

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
