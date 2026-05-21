//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/usetheo/theopacks/e2e/theoyaml"
)

// runtime_probe.go — T0.2 of theo-stacks-build-and-run-plan.
//
// runAndHealthcheck builds nothing: it expects the image tag to exist.
// It runs the container, waits for the app to come up (HTTP probe for
// server/frontend, process-alive for worker), then tears the container
// down via t.Cleanup. Failure modes (container exits early, probe times
// out) emit the container's stdout/stderr through t.Fatalf so the
// reason is visible in CI logs.

// pickFreePort asks the kernel for a free port and immediately closes
// the socket. There's a tiny race window before the test's docker run
// claims it, but in practice the kernel doesn't reuse the same port
// within microseconds.
func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// uniqueContainerName generates a parallel-safe container name. The
// docker daemon refuses duplicate names; t.Parallel + the same image
// would collide without this.
func uniqueContainerName(prefix string) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
}

// runAndHealthcheck runs `image` and validates it serves traffic
// (server / frontend) or stays alive (worker), per `app`. Total timeout
// is `probeTimeout`. Returns nil on success; calls t.Fatalf with
// container logs on failure.
//
// The container is registered for cleanup via t.Cleanup, so even on
// t.Fatalf the rm-f happens. Pull policy is "never" — image must be
// pre-built. extraEnv contains additional -e flags (e.g. DATABASE_URL
// supplied by T0.3 testcontainers integration).
func runAndHealthcheck(t *testing.T, image string, app theoyaml.AppConfig, probeTimeout time.Duration, extraEnv map[string]string) {
	t.Helper()
	containerName := uniqueContainerName("te2e-rt")
	hostPort := pickFreePort(t)

	args := []string{"run", "--rm", "-d", "--name", containerName}
	if app.Port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:%d", hostPort, app.Port))
	}
	for k, v := range extraEnv {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, image)

	out, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run failed for %s:\n%s", image, string(out))
	}
	t.Cleanup(func() {
		// Capture logs BEFORE rm so post-mortem is possible.
		logs, _ := exec.Command("docker", "logs", containerName).CombinedOutput()
		if t.Failed() && len(logs) > 0 {
			t.Logf("=== container logs (%s) ===\n%s", containerName, string(logs))
		}
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	})

	switch app.Type {
	case theoyaml.TypeServer, theoyaml.TypeFrontend:
		if app.Port <= 0 {
			t.Fatalf("app type=%s but port=%d; theo.yaml must declare a port for HTTP probes",
				app.Type, app.Port)
		}
		probeHTTP(t, containerName, hostPort, probeTimeout)
	case theoyaml.TypeWorker:
		probeWorkerAlive(t, containerName, probeTimeout)
	default:
		t.Fatalf("unknown app type %q (expected server / worker / frontend)", app.Type)
	}
}

// probeHTTP polls /health and falls back to / until 2xx or timeout.
// On 3xx, follows up to 1 redirect. On any error, retries until the
// timeout elapses. On final failure, dumps logs and t.Fatalf.
func probeHTTP(t *testing.T, containerName string, hostPort int, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 1 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	paths := []string{"/health", "/"}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for ctx.Err() == nil && time.Now().Before(deadline) {
		// If the container died between iterations, fail fast with logs.
		if !containerRunning(containerName) {
			logs, _ := exec.Command("docker", "logs", containerName).CombinedOutput()
			t.Fatalf("container %s exited during probe.\n=== logs ===\n%s",
				containerName, string(logs))
		}
		for _, p := range paths {
			url := fmt.Sprintf("http://127.0.0.1:%d%s", hostPort, p)
			req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return // success
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		}
		time.Sleep(500 * time.Millisecond)
	}

	logs, _ := exec.Command("docker", "logs", containerName).CombinedOutput()
	t.Fatalf("HTTP probe timed out after %s for %s. last error: %v\n=== container logs ===\n%s",
		timeout, containerName, lastErr, string(logs))
}

// probeWorkerAlive waits 5s and asserts the container is still running
// OR exited with code 0 (some workers are batch-style). Failure modes:
// exited with non-zero (logs captured), or still running but with
// repeated restart symptoms (we don't auto-detect; --rm prevents
// restart so a non-zero exit ends the container).
func probeWorkerAlive(t *testing.T, containerName string, timeout time.Duration) {
	t.Helper()
	// Give the worker time to boot.
	settle := 5 * time.Second
	if timeout < settle {
		settle = timeout
	}
	time.Sleep(settle)

	if containerRunning(containerName) {
		return
	}
	// Exited — check the code.
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.ExitCode}}", containerName).CombinedOutput()
	if err != nil {
		// Container might have been removed already because --rm.
		// In that case there's no way to know the exit code — fail.
		t.Fatalf("worker container %s gone before probe could read exit code: %v", containerName, err)
	}
	exitCode := strings.TrimSpace(string(out))
	if exitCode != "0" {
		logs, _ := exec.Command("docker", "logs", containerName).CombinedOutput()
		t.Fatalf("worker container %s exited with code %s\n=== logs ===\n%s",
			containerName, exitCode, string(logs))
	}
}

// containerRunning returns true iff `docker inspect` reports the
// container in the running state. Returns false on any error
// (most often "no such container" after --rm cleanup).
func containerRunning(name string) bool {
	out, err := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", name).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
