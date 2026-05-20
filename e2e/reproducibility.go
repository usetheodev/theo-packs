//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Reproducibility helpers (T3.2, robust-test-suite-plan).
//
// BuildKit ≥ 0.13 supports `--output type=image,rewrite-timestamp=true`
// combined with SOURCE_DATE_EPOCH to produce bit-identical OCI digests
// across re-runs. We exercise that on the examples whose runtime is
// most sensitive to drift: go-simple (static binary), node-npm (lots
// of metadata timestamps), python-flask (pip install).

const reproducibleEpoch = "1700000000"

// buildAndDigest builds an image with SOURCE_DATE_EPOCH set and returns
// the resulting digest. Requires `docker buildx` (ships with Docker
// Desktop / docker-buildx-plugin on Linux).
func buildAndDigest(t *testing.T, dockerfile, contextDir, tag string) string {
	t.Helper()
	dfPath := filepath.Join(contextDir, "Dockerfile.repro")
	require.NoError(t, os.WriteFile(dfPath, []byte(dockerfile), 0o644))
	t.Cleanup(func() { _ = os.Remove(dfPath) })

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "buildx", "build",
		"--builder", "default",
		"--platform", "linux/amd64",
		"--build-arg", "SOURCE_DATE_EPOCH="+reproducibleEpoch,
		"--output", "type=image,name="+tag+",rewrite-timestamp=true",
		"-f", dfPath,
		contextDir,
	)
	cmd.Env = append(os.Environ(),
		"DOCKER_BUILDKIT=1",
		"SOURCE_DATE_EPOCH="+reproducibleEpoch,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "buildx repro build failed:\n%s", string(out))

	// Capture image digest via inspect.
	digestOut, err := exec.Command("docker", "image", "inspect",
		tag, "--format", "{{.Id}}").CombinedOutput()
	require.NoError(t, err, "image inspect failed: %s", string(digestOut))
	return strings.TrimSpace(string(digestOut))
}

// reproducibleExamples — kept small. Each iteration is ~1-2 min.
var reproducibleExamples = []string{
	"go-simple",
	"python-flask",
	"node-npm",
}

// TestE2E_Reproducible_Build builds each reproducibleExamples twice
// back-to-back with the same SOURCE_DATE_EPOCH and asserts identical
// digests. When this fails, run diffoci (T3.3) to diagnose.
func TestE2E_Reproducible_Build(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	if !buildxAvailable() {
		t.Skip("docker buildx ≥ 0.13 required")
	}
	for _, example := range reproducibleExamples {
		example := example
		t.Run(example, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(examplesDir(t), example)
			df := generateDockerfile(t, dir, nil)

			tag1 := "te2e-repro-" + example + ":1"
			tag2 := "te2e-repro-" + example + ":2"
			t.Cleanup(func() {
				removeImage(tag1)
				removeImage(tag2)
			})

			d1 := buildAndDigest(t, df, dir, tag1)
			d2 := buildAndDigest(t, df, dir, tag2)
			if d1 != d2 {
				// T3.3 — invoke diffoci to surface what changed.
				diagnoseNonReproducible(t, tag1, tag2)
				t.Fatalf("reproducibility broken for %s:\n  build 1: %s\n  build 2: %s",
					example, d1, d2)
			}
		})
	}
}

// buildxAvailable reports whether `docker buildx build` works.
func buildxAvailable() bool {
	return exec.Command("docker", "buildx", "version").Run() == nil
}
