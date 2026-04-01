package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/dockerfile"
)

// E2E tests build real Docker images from example projects.
// They require Docker to be running and are skipped if unavailable.
// Run with: go test -tags e2e ./e2e/ -timeout 600s

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
func imageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	return cmd.Run() == nil
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

	// Verify the binary exists in the image
	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/server").CombinedOutput()
	require.NoError(t, err, "binary not found: %s", string(output))
	require.Contains(t, string(output), "/app/server")
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
				output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/server").CombinedOutput()
				require.NoError(t, err, "go binary missing: %s", string(output))
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

	output, err := exec.Command("docker", "run", "--rm", tag, "ls", "/app/server").CombinedOutput()
	require.NoError(t, err, "go workspace binary missing: %s", string(output))
}
