package dockerfile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
)

// Determinism regression tests (T0.2, test-suite-hardening-plan).
//
// Go map iteration order is randomized; the renderer and providers must
// compensate via sort.Strings / sort.Slice at every interior loop that
// turns a map into output. A future PR that drops one of those sorts
// would produce a build plan whose Dockerfile string differs run-to-run
// — silent flakiness in golden tests and downstream cache invalidation.
//
// These tests gate that invariant: generate N times against a complex
// realistic example and assert byte-identical output.

const determinismIterations = 10

func generateNTimes(t *testing.T, exampleName string, env map[string]string, n int) []string {
	t.Helper()
	dir := filepath.Join(examplesDir(t), exampleName)

	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		userApp, err := app.NewApp(dir)
		require.NoError(t, err)

		var envPtr *map[string]string
		if env != nil {
			envPtr = &env
		}
		appEnv := app.NewEnvironment(envPtr)

		opts := &core.GenerateBuildPlanOptions{}
		result := core.GenerateBuildPlan(userApp, appEnv, opts)
		require.True(t, result.Success, "iter %d: plan failed: %v", i, result.Logs)

		df, err := Generate(result.Plan)
		require.NoError(t, err, "iter %d: render failed", i)
		out = append(out, df)
	}
	return out
}

func assertAllIdentical(t *testing.T, outputs []string, label string) {
	t.Helper()
	require.GreaterOrEqual(t, len(outputs), 2, "need ≥ 2 outputs to compare")
	first := outputs[0]
	for i := 1; i < len(outputs); i++ {
		if outputs[i] == first {
			continue
		}
		// Show a short context window — the diff is almost always small.
		t.Fatalf(
			"%s: iteration %d differs from iteration 0\n--- iter 0 ---\n%s\n--- iter %d ---\n%s",
			label, i, first, i, outputs[i],
		)
	}
}

// TestRegression_NodeTurborepo_DeterministicOutput — complex workspace
// with multiple workspaces, manifests, package managers, and cache mounts.
// If any of those code paths drops a sort, this test catches it.
func TestRegression_NodeTurborepo_DeterministicOutput(t *testing.T) {
	t.Parallel()
	outputs := generateNTimes(t, "node-turborepo", map[string]string{
		"THEOPACKS_APP_NAME": "api",
		"THEOPACKS_APP_PATH": "apps/api",
	}, determinismIterations)
	assertAllIdentical(t, outputs, "node-turborepo")
}

// TestRegression_PythonFlask_DeterministicOutput — simpler provider but
// exercises pip caching, env vars, and the start-command auto-detect path.
func TestRegression_PythonFlask_DeterministicOutput(t *testing.T) {
	t.Parallel()
	outputs := generateNTimes(t, "python-flask", nil, determinismIterations)
	assertAllIdentical(t, outputs, "python-flask")
}

// TestRegression_GoWorkspaces_DeterministicOutput — Go workspace mode
// has its own module-listing loop where map order would surface.
func TestRegression_GoWorkspaces_DeterministicOutput(t *testing.T) {
	t.Parallel()
	outputs := generateNTimes(t, "go-workspaces", nil, determinismIterations)
	assertAllIdentical(t, outputs, "go-workspaces")
}
