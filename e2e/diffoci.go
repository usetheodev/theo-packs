//go:build e2e
// +build e2e

package e2e

import (
	"os/exec"
	"testing"
)

// diagnoseNonReproducible runs diffoci (when available) to compare
// the two image references and emits the diff to the test log.
// Called from TestE2E_Reproducible_Build on digest mismatch so the
// failure is actionable instead of "digests differ, good luck".
//
// T3.3 — robust-test-suite-plan. Skipped silently when diffoci isn't
// installed; the test still fails with the digest diff, just less
// helpfully.
func diagnoseNonReproducible(t *testing.T, tag1, tag2 string) {
	t.Helper()
	if _, err := exec.LookPath("diffoci"); err != nil {
		t.Logf("[T3.3] diffoci not on PATH — install reproducible-containers/diffoci for diff")
		return
	}
	out, err := exec.Command("diffoci", "diff",
		"docker://"+tag1, "docker://"+tag2,
	).CombinedOutput()
	if err != nil {
		t.Logf("[T3.3] diffoci diff failed: %v\n%s", err, string(out))
		return
	}
	t.Logf("[T3.3] diffoci output (build 1 vs build 2):\n%s", string(out))
}
