package dockerfile

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
)

// Generation benchmarks (T3.4, test-suite-hardening-plan). Baseline
// recorded in docs/benchmarks/baseline.txt — when a PR regresses any
// of these meaningfully (≥ 30%), reviewers should ask why.

// benchExamplesDir mirrors examplesDir() but accepts *testing.B (the
// existing helper is *testing.T only).
func benchExamplesDir(b *testing.B) string {
	b.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("could not determine test file location")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "examples")
	abs, err := filepath.Abs(dir)
	if err != nil {
		b.Fatal(err)
	}
	return abs
}

func benchExample(b *testing.B, name string, env map[string]string) {
	b.Helper()
	dir := filepath.Join(benchExamplesDir(b), name)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userApp, _ := app.NewApp(dir)
		var envPtr *map[string]string
		if env != nil {
			envPtr = &env
		}
		appEnv := app.NewEnvironment(envPtr)
		result := core.GenerateBuildPlan(userApp, appEnv, &core.GenerateBuildPlanOptions{})
		if !result.Success {
			b.Fatalf("plan failed: %v", result.Logs)
		}
		if _, err := Generate(result.Plan); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerate_GoSimple(b *testing.B) {
	benchExample(b, "go-simple", nil)
}

func BenchmarkGenerate_NodeTurborepo(b *testing.B) {
	benchExample(b, "node-turborepo", map[string]string{
		"THEOPACKS_APP_NAME": "api",
		"THEOPACKS_APP_PATH": "apps/api",
	})
}

func BenchmarkGenerate_PythonFlask(b *testing.B) {
	benchExample(b, "python-flask", nil)
}
