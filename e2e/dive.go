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

// diveAvailable reports whether the `dive` binary is on PATH. Same
// "skip gracefully" pattern as hadolint — local dev without dive
// shouldn't break the suite; CI installs it explicitly.
func diveAvailable() bool {
	_, err := exec.LookPath("dive")
	return err == nil
}

// runDive runs `dive` in CI mode against the given image tag, using
// the versioned .dive-ci thresholds at the repo root. Returns without
// failing the test when:
//   - dive binary is absent (local dev),
//   - DIVE_SKIP=1 is set (caller wants to bypass for diagnostic runs).
//
// T0.4 — robust-test-suite-plan. Catches layer bloat (devDependencies
// leaking to runtime, duplicate COPYs) that a simple size budget
// doesn't surface.
func runDive(t *testing.T, tag string) {
	t.Helper()
	if os.Getenv("DIVE_SKIP") == "1" {
		t.Logf("[T0.4] DIVE_SKIP=1 — skipping dive efficiency check")
		return
	}
	if !diveAvailable() {
		t.Logf("[T0.4] dive not on PATH — skipping efficiency check")
		return
	}

	configPath := repoRootDiveConfig(t)
	cmd := exec.Command("dive", tag, "--ci-config", configPath)
	cmd.Env = append(os.Environ(), "CI=true")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dive efficiency check failed for %s:\n%s", tag, string(out))
	}
}

func repoRootDiveConfig(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", ".dive-ci")
}
