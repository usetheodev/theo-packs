//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// hadolintAvailable reports whether the hadolint binary is on PATH.
// E2E suites skip the lint gate gracefully in environments where the
// tool isn't installed (local dev without hadolint) while CI always
// has it.
func hadolintAvailable() bool {
	_, err := exec.LookPath("hadolint")
	return err == nil
}

// runHadolint lints `dockerfileContent` using the repo-level
// .hadolint.yaml. Failure-threshold is warning (configured in the
// YAML), so info/style findings don't fail the run. Skips silently
// when the binary is absent so local runs don't break.
//
// T0.2 — robust-test-suite-plan: extends the static gate on goldens
// to the dynamic Dockerfiles produced by E2E so providers that emit
// different output at runtime (cache mounts driven by env, scoped
// workspace targets, etc.) still get lint feedback.
func runHadolint(t *testing.T, dockerfileContent string) {
	t.Helper()
	if !hadolintAvailable() {
		t.Logf("[T0.2] hadolint not on PATH — skipping lint gate")
		return
	}

	// Write to a temp Dockerfile so hadolint has a real path to report.
	dir := t.TempDir()
	df := filepath.Join(dir, "Dockerfile.e2e")
	if err := os.WriteFile(df, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("write temp dockerfile: %v", err)
	}

	configPath := repoRootHadolintConfig(t)
	cmd := exec.Command("hadolint", "--no-color", "--config", configPath, df)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hadolint reported findings in generated Dockerfile:\n%s\n--- Dockerfile ---\n%s",
			string(out), dockerfileContent)
	}
}

// repoRootHadolintConfig returns the absolute path to the versioned
// .hadolint.yaml. Resolved relative to this source file so the test
// works regardless of cwd.
func repoRootHadolintConfig(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	// e2e/hadolint.go → repo root → .hadolint.yaml
	return filepath.Join(filepath.Dir(thisFile), "..", ".hadolint.yaml")
}
