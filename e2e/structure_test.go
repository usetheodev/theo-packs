//go:build e2e
// +build e2e

package e2e

import (
	"os/exec"
	"testing"
)

// structureTestAvailable reports whether `container-structure-test` is
// on PATH. Same skip-gracefully pattern as hadolint / dive — local dev
// without it is OK; CI must install it explicitly.
func structureTestAvailable() bool {
	_, err := exec.LookPath("container-structure-test")
	return err == nil
}

// runStructureTest validates `tag` against the YAML config at
// configPath using Google's container-structure-test
// (T1.1 — robust-test-suite-plan). The YAML declares file existence,
// command, content, and metadata assertions; structure-test runs them
// against the image using the docker driver.
//
// Skips silently when the binary is absent (local dev). CI installs
// container-structure-test explicitly so the gate is enforced there.
//
// Note: container-structure-test is in maintenance mode (per its
// README) but remains the most concise way to declare image
// invariants. Documented in docs/adrs/0003-structure-test.md.
func runStructureTest(t *testing.T, tag, configPath string) {
	t.Helper()
	if !structureTestAvailable() {
		t.Logf("[T1.1] container-structure-test not on PATH — skipping")
		return
	}
	cmd := exec.Command("container-structure-test", "test",
		"--image", tag,
		"--config", configPath,
		"--driver", "docker",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("structure-test failed for %s with config %s:\n%s",
			tag, configPath, string(out))
	}
}
