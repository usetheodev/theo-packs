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

// syftAvailable reports whether the syft binary is on PATH.
func syftAvailable() bool {
	_, err := exec.LookPath("syft")
	return err == nil
}

// generateSBOM produces SPDX and CycloneDX SBOMs for the given image
// tag and writes them under e2e/sboms/<tag>.{spdx,cyclonedx}.json. The
// directory is gitignored except in release pipelines, where the L5
// workflow archives the artifacts and attests them via cosign (T4.3).
//
// Skips silently when syft isn't installed (local dev). Failures here
// never fail the test — SBOM generation is advisory; a malformed
// image is caught by the build step itself.
//
// T1.3 — robust-test-suite-plan.
func generateSBOM(t *testing.T, tag string) {
	t.Helper()
	if !syftAvailable() {
		t.Logf("[T1.3] syft not on PATH — skipping SBOM generation")
		return
	}

	outDir := repoRootSBOMDir(t)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Logf("[T1.3] mkdir %s: %v (skipping)", outDir, err)
		return
	}

	safe := safeName(tag)
	for _, fmt := range []struct {
		flag, ext string
	}{
		{"spdx-json", "spdx.json"},
		{"cyclonedx-json", "cyclonedx.json"},
	} {
		out := filepath.Join(outDir, safe+"."+fmt.ext)
		cmd := exec.Command("syft", tag, "-o", fmt.flag+"="+out)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Logf("[T1.3] syft %s for %s failed: %v\n%s", fmt.flag, tag, err, string(b))
			return
		}
	}
}

// safeName turns a docker tag (which may contain `/` and `:`) into a
// filename-safe slug.
func safeName(tag string) string {
	out := make([]byte, 0, len(tag))
	for i := 0; i < len(tag); i++ {
		c := tag[i]
		switch {
		case c == '/' || c == ':' || c == ' ':
			out = append(out, '-')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

func repoRootSBOMDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "sboms")
}
