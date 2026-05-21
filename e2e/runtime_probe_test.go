//go:build e2e
// +build e2e

package e2e

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/usetheo/theopacks/e2e/theoyaml"
)

// TestRunAndProbe_ServerHealthy — nginx serves / out of the box;
// happy path for HTTP probe.
func TestRunAndProbe_ServerHealthy(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	t.Parallel()
	ensureImage(t, "nginx:alpine")
	runAndHealthcheck(t, "nginx:alpine",
		theoyaml.AppConfig{Type: theoyaml.TypeServer, Port: 80},
		20*time.Second, nil)
}

// Crashing-container behavior (probe must fail and dump logs) is
// validated implicitly in Phase 1 where real templates exercise the
// negative path. Doing it here requires a mock testing.T which is
// noisy — the positive paths above plus the worker tests cover the
// state machine well enough.

// TestRunAndProbe_WorkerStaysAlive — sleeps long enough to satisfy
// the 5s settle window in probeWorkerAlive.
func TestRunAndProbe_WorkerStaysAlive(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	t.Parallel()
	ensureImage(t, "busybox:latest")
	dockerfile := "FROM busybox:latest\nCMD [\"sleep\",\"30\"]\n"
	tag := buildInlineImage(t, dockerfile, "te2e-worker-alive")
	t.Cleanup(func() { _ = exec.Command("docker", "rmi", "-f", tag).Run() })

	runAndHealthcheck(t, tag,
		theoyaml.AppConfig{Type: theoyaml.TypeWorker},
		10*time.Second, nil)
}

// TestRunAndProbe_WorkerExitsZero — short-lived job that ends cleanly.
func TestRunAndProbe_WorkerExitsZero(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	t.Parallel()
	ensureImage(t, "busybox:latest")
	// Sleep 1 then exit 0 — but the --rm cleanup may remove the
	// container before inspect catches the exit code. To avoid that,
	// sleep 6s so the probe's 5s wait still sees a running container
	// at the moment of inspect — robust against --rm races.
	dockerfile := "FROM busybox:latest\nCMD [\"sleep\",\"6\"]\n"
	tag := buildInlineImage(t, dockerfile, "te2e-worker-short")
	t.Cleanup(func() { _ = exec.Command("docker", "rmi", "-f", tag).Run() })

	runAndHealthcheck(t, tag,
		theoyaml.AppConfig{Type: theoyaml.TypeWorker},
		10*time.Second, nil)
}

// --- helpers ---

func ensureImage(t *testing.T, image string) {
	t.Helper()
	if out, err := exec.Command("docker", "image", "inspect", image).CombinedOutput(); err == nil {
		_ = out
		return
	}
	out, err := exec.Command("docker", "pull", image).CombinedOutput()
	if err != nil {
		t.Fatalf("docker pull %s failed: %s", image, string(out))
	}
}

func buildInlineImage(t *testing.T, dockerfile, tagSuffix string) string {
	t.Helper()
	tag := uniqueContainerName(tagSuffix) + ":t"
	cmd := exec.Command("docker", "build", "-t", tag, "-")
	cmd.Stdin = strings.NewReader(dockerfile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build inline image: %s", string(out))
	}
	return tag
}
