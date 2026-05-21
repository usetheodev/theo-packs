//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/e2e/theoyaml"
)

// theo_stacks_runtime_test.go — T1.1 of
// docs/plans/theo-stacks-build-and-run-plan.md.
//
// For each upstream template:
//   1. render templates → temp dir
//   2. run theopacks-generate → Dockerfile
//   3. docker build → image tag
//   4. docker run + health probe (HTTP for server/frontend,
//      process-alive for worker) via T0.2 runAndHealthcheck
//   5. t.Cleanup removes the image + container
//
// The build step is the same one TestE2E_TheoStacksTemplates uses to
// gate the Dockerfile-generation contract; this test extends it with
// real `docker run` + healthcheck. Together they form the full
// build-and-run gate.

const runtimeProbeTimeout = 90 * time.Second

func TestE2E_TheoStacksTemplates_Runtime(t *testing.T) {
	stacksDir := theoStacksDir(t)
	if _, err := os.Stat(stacksDir); err != nil {
		t.Skipf("theo-stacks templates not at %s — clone https://github.com/usetheodev/theo-stacks or set %s",
			stacksDir, theoStacksEnv)
	}
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	for _, exp := range templateExpectations {
		exp := exp
		t.Run(exp.template, func(t *testing.T) {
			t.Parallel()
			if exp.skipReason != "" {
				t.Skip(exp.skipReason)
			}
			srcDir := filepath.Join(stacksDir, exp.template)
			if _, err := os.Stat(srcDir); err != nil {
				t.Skipf("template %s not present at %s", exp.template, srcDir)
			}

			// 1. render
			rendered := t.TempDir()
			renderTemplate(t, srcDir, rendered)

			// 2. theo-packs → Dockerfile
			df := runTheopacksOnTemplate(t, rendered, exp.appName, exp.appPath)

			// 3. docker build (write Dockerfile inside context dir)
			dfPath := filepath.Join(rendered, "Dockerfile.runtime")
			require.NoError(t, os.WriteFile(dfPath, []byte(df), 0o644))
			t.Cleanup(func() { _ = os.Remove(dfPath) })

			tag := "te2e-runtime-" + exp.template + ":t"
			buildCtx, buildCancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer buildCancel()
			buildCmd := exec.CommandContext(buildCtx,
				"docker", "build", "-f", dfPath, "-t", tag, rendered)
			buildCmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
			if out, err := buildCmd.CombinedOutput(); err != nil {
				t.Fatalf("docker build failed for %s:\n%s", exp.template, string(out))
			}
			t.Cleanup(func() { _ = exec.Command("docker", "rmi", "-f", tag).Run() })

			// 4. read theo.yaml + run
			cfg, err := theoyaml.ParseFile(filepath.Join(rendered, "theo.yaml"))
			require.NoError(t, err, "every template must ship a parseable theo.yaml")

			app, ok := cfg.App(exp.appName)
			if !ok {
				// Single-app template: pick the lone app.
				app, ok = cfg.App("")
				require.True(t, ok,
					"theo.yaml for %s declares no app under appName=%q and isn't single-app",
					exp.template, exp.appName)
			}

			runAndHealthcheck(t, tag, app, runtimeProbeTimeout, nil)
		})
	}
}
