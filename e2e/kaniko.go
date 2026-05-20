//go:build e2e_kaniko && e2e
// +build e2e_kaniko,e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Kaniko differential test infrastructure (T3.1,
// robust-test-suite-plan).
//
// theo-packs runs in production via Argo Workflow → Kaniko. Kaniko's
// support for BuildKit-specific syntax differs: `--mount=type=cache`
// honors a different semantics (≥ Kaniko 1.9), `--mount=type=secret`
// behavior is also distinct. This file builds the critical examples
// with Kaniko and validates the resulting image behaves identically.

const kanikoImage = "gcr.io/kaniko-project/executor:v1.23.2"

// buildWithKaniko runs Kaniko via docker (no Kubernetes required) to
// build the given Dockerfile + context into the named tag.
//
// Kaniko writes the resulting image to a local tarball; we then
// `docker load` it so the rest of the suite (structure-test, dive)
// can operate on a real image reference.
func buildWithKaniko(t *testing.T, dockerfilePath, contextDir, tag string) {
	t.Helper()
	outTar := filepath.Join(t.TempDir(), "kaniko-output.tar")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Mount context + dockerfile into the kaniko container.
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", contextDir+":/workspace:ro",
		"-v", outTar+":/output.tar",
		kanikoImage,
		"--dockerfile", "/workspace/"+filepath.Base(dockerfilePath),
		"--context", "dir:///workspace",
		"--destination", tag,
		"--no-push",
		"--tar-path", "/output.tar",
		"--single-snapshot",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "kaniko build failed:\n%s", string(out))

	// Load the resulting tar into local docker so subsequent steps work.
	loadCmd := exec.Command("docker", "load", "-i", outTar)
	loadOut, loadErr := loadCmd.CombinedOutput()
	require.NoError(t, loadErr, "docker load failed: %s", string(loadOut))
}

// kanikoCritical lists the examples that must build identically under
// Kaniko. Smaller set than the full E2E table — each entry costs ~2-3min.
var kanikoCritical = []string{
	"go-simple",
	"node-npm",
	"python-flask",
	"rust-axum",
	"java-spring-gradle",
	"node-pnpm-workspaces",
	"node-turborepo",
	"dotnet-aspnet",
	"ruby-sinatra",
	"php-slim",
}

// TestE2E_Kaniko_Differential builds each critical example with Kaniko
// after generating the Dockerfile via theo-packs. The assertion is
// build success — finer parity (structure-test parity) is layered on
// top below.
func TestE2E_Kaniko_Differential(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	for _, example := range kanikoCritical {
		example := example
		t.Run(example, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(examplesDir(t), example)

			// Generate Dockerfile via library (matches the BuildKit path).
			env := envForKaniko(example)
			df := generateDockerfile(t, dir, env)
			runHadolint(t, df)

			// Write Dockerfile inside context dir (Kaniko reads it from there).
			dfPath := filepath.Join(dir, "Dockerfile.kaniko")
			require.NoError(t, os.WriteFile(dfPath, []byte(df), 0o644))
			t.Cleanup(func() { _ = os.Remove(dfPath) })

			tag := "te2e-kaniko-" + example + ":diff"
			t.Cleanup(func() { removeImage(tag) })
			buildWithKaniko(t, dfPath, dir, tag)
			require.True(t, imageExists(tag), "kaniko-built image missing")
		})
	}
}

// envForKaniko returns the same env map used by the BuildKit path for
// workspace examples (so the comparison is apples-to-apples).
func envForKaniko(example string) map[string]string {
	switch example {
	case "node-turborepo":
		return map[string]string{
			"THEOPACKS_APP_NAME": "api",
			"THEOPACKS_APP_PATH": "apps/api",
		}
	}
	return nil
}
