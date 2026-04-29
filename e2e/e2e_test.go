//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/dockerfile"
)

// E2E tests build real Docker images from example projects. They require
// Docker to be running and are gated behind the `e2e` build tag so a plain
// `go test ./...` does not pull them in.
//
// Run with: go test -tags e2e ./e2e/ -timeout 1500s

func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

func examplesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "examples")
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)
	return abs
}

// generateDockerfile runs the full theopacks pipeline and returns a Dockerfile string.
func generateDockerfile(t *testing.T, projectDir string, envVars map[string]string) string {
	t.Helper()

	userApp, err := app.NewApp(projectDir)
	require.NoError(t, err)

	var envPtr *map[string]string
	if envVars != nil {
		envPtr = &envVars
	}
	env := app.NewEnvironment(envPtr)

	opts := &core.GenerateBuildPlanOptions{
		TheopacksVersion: "e2e-test",
	}

	result := core.GenerateBuildPlan(userApp, env, opts)
	require.True(t, result.Success, "plan generation failed: %v", result.Logs)

	df, err := dockerfile.Generate(result.Plan)
	require.NoError(t, err)

	return df
}

// buildImage writes a Dockerfile into the project dir and builds an image.
func buildImage(t *testing.T, projectDir, dockerfileContent, tag string) {
	t.Helper()

	dfPath := filepath.Join(projectDir, "Dockerfile")
	err := os.WriteFile(dfPath, []byte(dockerfileContent), 0644)
	require.NoError(t, err)
	defer func() { _ = os.Remove(dfPath) }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build", "-t", tag, "-f", dfPath, projectDir)
	cmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "docker build failed:\n%s", string(output))
}

// removeImage cleans up a Docker image.
func removeImage(tag string) {
	_ = exec.Command("docker", "rmi", "-f", tag).Run()
}

// imageExists checks if a Docker image exists.
// requireBinaryAt verifies a path exists inside an image even when the image
// has no shell or `ls` (e.g., distroless). It uses `docker create` + `docker
// cp` to materialize the file on the host — succeeds iff the file exists in
// the image.
func requireBinaryAt(t *testing.T, tag, path string) {
	t.Helper()
	cid, err := exec.Command("docker", "create", tag).Output()
	require.NoError(t, err, "docker create failed for %s", tag)
	containerID := strings.TrimSpace(string(cid))
	defer func() { _ = exec.Command("docker", "rm", "-f", containerID).Run() }()

	tmp := filepath.Join(t.TempDir(), "binary")
	out, err := exec.Command("docker", "cp", containerID+":"+path, tmp).CombinedOutput()
	require.NoError(t, err, "binary missing at %s in image %s: %s", path, tag, string(out))

	info, err := os.Stat(tmp)
	require.NoError(t, err)
	require.False(t, info.IsDir(), "expected %s to be a file, got directory", path)
	require.Greater(t, info.Size(), int64(0), "%s is empty", path)
}

func imageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	return cmd.Run() == nil
}

// requireSizeLessThan asserts the named image is smaller than maxMB megabytes.
// Reports the actual size in MB on failure. Caps are intentionally loose
// (D6: absolute bounds beat ratios because base images drift across Debian
// point releases — generous bounds won't flake within a 12-month horizon).
func requireSizeLessThan(t *testing.T, tag string, maxMB int) {
	t.Helper()
	out, err := exec.Command("docker", "image", "inspect", tag, "--format", "{{.Size}}").Output()
	require.NoError(t, err, "docker image inspect failed for %s", tag)
	sizeBytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	require.NoError(t, err, "could not parse image size %q", string(out))
	sizeMB := int(sizeBytes / (1024 * 1024))
	require.LessOrEqual(t, sizeMB, maxMB,
		"image %s is %d MB, expected <= %d MB — devDependencies likely shipping to runtime", tag, sizeMB, maxMB)
}

func TestE2E_GoSimple_BuildsImage(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	dir := filepath.Join(examplesDir(t), "go-simple")
	df := generateDockerfile(t, dir, nil)
	tag := "theopacks-e2e-go-simple:test"
	defer removeImage(tag)

	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))
	// Distroless runtime has no `ls`. The COPY --from=build /app/server step
	// in the Dockerfile already fails the build if the binary is missing, so
	// `imageExists` is sufficient proof that /app/server is in the image.
	requireBinaryAt(t, tag, "/app/server")
}

func TestE2E_NodeNpm_BuildsImage(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	dir := filepath.Join(examplesDir(t), "node-npm")
	df := generateDockerfile(t, dir, nil)
	tag := "theopacks-e2e-node-npm:test"
	defer removeImage(tag)

	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))

	// Verify node is available and package.json was copied
	output, err := exec.Command("docker", "run", "--rm", tag, "node", "-e", "console.log('ok')").CombinedOutput()
	require.NoError(t, err, "node not working: %s", string(output))
	require.Contains(t, string(output), "ok")

	// Phase 6: lock the size win from npm prune --omit=dev. node:20-bookworm-slim
	// is ~210 MB; a hello-world app + prod-only deps should fit comfortably
	// under 280 MB. If devDependencies leaked through, we'd see ~150 MB more.
	requireSizeLessThan(t, tag, 280)
}

func TestE2E_PythonFlask_BuildsImage(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	dir := filepath.Join(examplesDir(t), "python-flask")
	df := generateDockerfile(t, dir, map[string]string{
		"THEOPACKS_START_CMD": "python -c 'print(1)'",
	})
	tag := "theopacks-e2e-python-flask:test"
	defer removeImage(tag)

	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))

	// Verify flask was installed
	output, err := exec.Command("docker", "run", "--rm", tag,
		"python", "-c", "import flask; print(flask.__version__)").CombinedOutput()
	require.NoError(t, err, "flask not installed: %s", string(output))
	require.NotEmpty(t, strings.TrimSpace(string(output)))

	// Phase 6: python:3.12-slim-bookworm is ~125 MB; flask + gunicorn add a
	// modest amount. Cap at 280 MB to catch regressions where __pycache__
	// or .venv start leaking into the runtime image.
	requireSizeLessThan(t, tag, 280)
}

func TestE2E_StaticFile_BuildsImage(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	dir := filepath.Join(examplesDir(t), "staticfile")
	df := generateDockerfile(t, dir, nil)
	tag := "theopacks-e2e-staticfile:test"
	defer removeImage(tag)

	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))

	// Verify index.html was copied
	output, err := exec.Command("docker", "run", "--rm", tag, "cat", "/app/index.html").CombinedOutput()
	require.NoError(t, err, "index.html not found: %s", string(output))
	require.Contains(t, string(output), "html")
}

func TestE2E_ShellScript_BuildsImage(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	dir := filepath.Join(examplesDir(t), "shell-script")
	df := generateDockerfile(t, dir, map[string]string{
		"THEOPACKS_START_CMD": "bash start.sh",
	})
	tag := "theopacks-e2e-shell:test"
	defer removeImage(tag)

	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))

	// Verify start.sh was copied
	output, err := exec.Command("docker", "run", "--rm", tag, "cat", "/app/start.sh").CombinedOutput()
	require.NoError(t, err, "start.sh not found: %s", string(output))
	require.NotEmpty(t, strings.TrimSpace(string(output)))
}

func TestE2E_FullstackMixed_AllServicesBuild(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	services := []struct {
		subdir string
		env    map[string]string
		verify func(t *testing.T, tag string)
	}{
		{
			subdir: "services/api",
			env:    nil,
			verify: func(t *testing.T, tag string) {
				// Go runtime is now distroless (no shell) — use docker cp.
				requireBinaryAt(t, tag, "/app/server")
			},
		},
		{
			subdir: "services/web",
			env:    nil,
			verify: func(t *testing.T, tag string) {
				output, err := exec.Command("docker", "run", "--rm", tag, "node", "-e", "console.log('ok')").CombinedOutput()
				require.NoError(t, err, "node not working: %s", string(output))
			},
		},
		{
			subdir: "services/worker",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: func(t *testing.T, tag string) {
				output, err := exec.Command("docker", "run", "--rm", tag, "python", "-c", "print('ok')").CombinedOutput()
				require.NoError(t, err, "python not working: %s", string(output))
			},
		},
	}

	for _, svc := range services {
		t.Run(svc.subdir, func(t *testing.T) {
			dir := filepath.Join(examplesDir(t), "fullstack-mixed", svc.subdir)
			df := generateDockerfile(t, dir, svc.env)
			tag := fmt.Sprintf("theopacks-e2e-fullstack-%s:test",
				strings.ReplaceAll(filepath.Base(svc.subdir), "/", "-"))
			defer removeImage(tag)

			buildImage(t, dir, df, tag)
			require.True(t, imageExists(tag))
			svc.verify(t, tag)
		})
	}
}

func TestE2E_GoWorkspace_BuildsImage(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}

	dir := filepath.Join(examplesDir(t), "go-workspaces")
	df := generateDockerfile(t, dir, nil)
	tag := "theopacks-e2e-go-workspaces:test"
	defer removeImage(tag)

	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))
	requireBinaryAt(t, tag, "/app/server")
}

// runE2EBuild is a small helper for the simpler "build & assert exists" cases
// to keep new-language tests succinct. It lets each test focus on the assertion
// that's specific to that language.
func runE2EBuild(t *testing.T, exampleName, tag string, env map[string]string) string {
	t.Helper()
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	dir := filepath.Join(examplesDir(t), exampleName)
	df := generateDockerfile(t, dir, env)
	t.Cleanup(func() { removeImage(tag) })
	buildImage(t, dir, df, tag)
	require.True(t, imageExists(tag))
	return dir
}

func TestE2E_RustAxum_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-rust-axum:test"
	runE2EBuild(t, "rust-axum", tag, nil)
	requireBinaryAt(t, tag, "/app/server")
}

func TestE2E_RustWorkspace_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-rust-workspace:test"
	runE2EBuild(t, "rust-workspace", tag, map[string]string{"THEOPACKS_APP_NAME": "api"})
	requireBinaryAt(t, tag, "/app/server")
}

func TestE2E_JavaSpringGradle_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-java-spring-gradle:test"
	runE2EBuild(t, "java-spring-gradle", tag, nil)

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/app.jar").CombinedOutput()
	require.NoError(t, err, "fat JAR missing: %s", string(output))
}

func TestE2E_JavaGradleWorkspace_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-java-gradle-workspace:test"
	runE2EBuild(t, "java-gradle-workspace", tag, map[string]string{"THEOPACKS_APP_NAME": "api"})

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/app.jar").CombinedOutput()
	require.NoError(t, err, "workspace fat JAR missing: %s", string(output))
}

func TestE2E_DotnetAspnet_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-dotnet-aspnet:test"
	runE2EBuild(t, "dotnet-aspnet", tag, nil)

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/publish").CombinedOutput()
	require.NoError(t, err, "publish output missing: %s", string(output))
	require.Contains(t, string(output), "dotnet-aspnet.dll")
}

func TestE2E_DotnetSolution_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-dotnet-solution:test"
	runE2EBuild(t, "dotnet-solution", tag, nil)

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/publish").CombinedOutput()
	require.NoError(t, err, "solution publish missing: %s", string(output))
}

func TestE2E_RubySinatra_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-ruby-sinatra:test"
	runE2EBuild(t, "ruby-sinatra", tag, nil)

	// `bundle info sinatra` prints gem metadata without loading sinatra's
	// code. We can't `require "sinatra"` here because Sinatra's classic-style
	// main.rb registers an at_exit hook that starts the web server when the
	// process exits — that would hang docker run indefinitely.
	output, err := exec.Command("docker", "run", "--rm", tag, "bundle", "info", "sinatra").CombinedOutput()
	require.NoError(t, err, "sinatra not installed: %s", string(output))
	require.Contains(t, string(output), "sinatra")
}

func TestE2E_RubyMonorepo_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-ruby-monorepo:test"
	runE2EBuild(t, "ruby-monorepo", tag, map[string]string{"THEOPACKS_APP_NAME": "api"})

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "apps/api/config.ru").CombinedOutput()
	require.NoError(t, err, "monorepo source missing: %s", string(output))
}

func TestE2E_PhpSlim_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-php-slim:test"
	runE2EBuild(t, "php-slim", tag, nil)

	output, err := exec.Command("docker", "run", "--rm", tag, "php", "--version").CombinedOutput()
	require.NoError(t, err, "php not present: %s", string(output))
	require.Contains(t, string(output), "PHP")
}

func TestE2E_PhpMonorepo_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-php-monorepo:test"
	runE2EBuild(t, "php-monorepo", tag, map[string]string{"THEOPACKS_APP_NAME": "api"})

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "apps/api/public/index.php").CombinedOutput()
	require.NoError(t, err, "monorepo entry missing: %s", string(output))
}

func TestE2E_DenoHono_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-deno-hono:test"
	runE2EBuild(t, "deno-hono", tag, nil)

	output, err := exec.Command("docker", "run", "--rm", tag, "deno", "--version").CombinedOutput()
	require.NoError(t, err, "deno not present: %s", string(output))
	require.Contains(t, string(output), "deno")
}

func TestE2E_DenoWorkspace_BuildsImage(t *testing.T) {
	tag := "theopacks-e2e-deno-workspace:test"
	runE2EBuild(t, "deno-workspace", tag, map[string]string{"THEOPACKS_APP_NAME": "api"})

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "apps/api/main.ts").CombinedOutput()
	require.NoError(t, err, "workspace member entry missing: %s", string(output))
}
