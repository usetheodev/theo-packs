package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildBinary compiles the theopacks-generate binary into a temporary directory
// and returns its path. Uses GOWORK=off to avoid workspace interference.
func buildBinary(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	binPath := filepath.Join(dir, "theopacks-generate")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = filepath.Join(projectRoot(t), "cmd", "theopacks-generate")
	cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build binary: %s", string(out))

	return binPath
}

// projectRoot returns the repo root (two levels up from cmd/theopacks-generate/).
func projectRoot(t *testing.T) string {
	t.Helper()

	// We're in cmd/theopacks-generate/, go up two levels.
	wd, err := os.Getwd()
	require.NoError(t, err)

	root := filepath.Join(wd, "..", "..")
	_, err = os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err, "could not find repo root go.mod from %s", root)

	return root
}

func TestGenerateNodeNpm(t *testing.T) {
	// Arrange
	bin := buildBinary(t)
	root := projectRoot(t)
	source := filepath.Join(root, "examples", "node-npm")
	outputDir := t.TempDir()
	outputFile := filepath.Join(outputDir, "Dockerfile")

	// Act
	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "node-npm",
		"--output", outputFile,
	)
	stdout, err := cmd.CombinedOutput()
	require.NoError(t, err, "binary failed: %s", string(stdout))

	// Assert
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	dockerfile := string(content)
	require.Contains(t, dockerfile, "FROM", "Dockerfile must have FROM instruction")
	require.Contains(t, dockerfile, "CMD", "Dockerfile must have CMD instruction")

	// Node start command should reference npm or node
	cmdLine := extractCMDLine(dockerfile)
	require.True(t,
		strings.Contains(cmdLine, "npm") || strings.Contains(cmdLine, "node"),
		"CMD should contain npm or node, got: %s", cmdLine,
	)

	// stdout should contain the Dockerfile (logged for Loki)
	require.Contains(t, string(stdout), "--- Generated Dockerfile for node-npm ---")
}

func TestGenerateGoMod(t *testing.T) {
	// Arrange
	bin := buildBinary(t)
	root := projectRoot(t)
	source := filepath.Join(root, "examples", "go-simple")
	outputDir := t.TempDir()
	outputFile := filepath.Join(outputDir, "Dockerfile")

	// Act
	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "go-app",
		"--output", outputFile,
	)
	stdout, err := cmd.CombinedOutput()
	require.NoError(t, err, "binary failed: %s", string(stdout))

	// Assert
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	dockerfile := string(content)
	require.Contains(t, dockerfile, "FROM", "Dockerfile must have FROM instruction")
	require.Contains(t, dockerfile, "CMD", "Dockerfile must have CMD instruction")

	// Go builds produce a binary — CMD should reference it
	cmdLine := extractCMDLine(dockerfile)
	require.NotEmpty(t, cmdLine, "CMD line should not be empty")
}

func TestUserProvidedDockerfileTakesPrecedence(t *testing.T) {
	// Arrange: create a temp project with a user-provided Dockerfile
	bin := buildBinary(t)
	sourceDir := t.TempDir()

	userContent := "FROM alpine:3.20\nCMD [\"echo\", \"user-provided\"]\n"
	err := os.WriteFile(filepath.Join(sourceDir, "Dockerfile"), []byte(userContent), 0644)
	require.NoError(t, err)

	outputDir := t.TempDir()
	outputFile := filepath.Join(outputDir, "Dockerfile")

	// Act
	cmd := exec.Command(bin,
		"--source", sourceDir,
		"--app-path", ".",
		"--app-name", "custom",
		"--output", outputFile,
	)
	stdout, err := cmd.CombinedOutput()
	require.NoError(t, err, "binary failed: %s", string(stdout))

	// Assert: output should be the user's Dockerfile, not generated
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	require.Equal(t, userContent, string(content))

	require.Contains(t, string(stdout), "User-provided Dockerfile")
}

func TestFailsWithActionableMessageForMissingSource(t *testing.T) {
	// Arrange
	bin := buildBinary(t)
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	// Act: point to a non-existent directory
	cmd := exec.Command(bin,
		"--source", "/nonexistent/path/that/does/not/exist",
		"--app-path", ".",
		"--app-name", "ghost",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()

	// Assert: should fail with exit code 1 and an actionable message
	require.Error(t, err, "should fail for missing source")
	output := string(out)
	require.True(t,
		strings.Contains(output, "Make sure the app path is correct") ||
			strings.Contains(output, "Failed to analyze source"),
		"error should contain actionable guidance, got: %s", output,
	)
}

func TestOutputGoesToStdout(t *testing.T) {
	// Arrange
	bin := buildBinary(t)
	root := projectRoot(t)
	source := filepath.Join(root, "examples", "node-npm")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	// Act: capture stdout separately from stderr
	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "test-app",
		"--output", outputFile,
	)

	stdoutBytes, err := cmd.Output()
	require.NoError(t, err, "binary failed")

	// Assert: stdout contains the Dockerfile content (for Loki capture)
	stdout := string(stdoutBytes)
	require.Contains(t, stdout, "--- Generated Dockerfile for test-app ---")
	require.Contains(t, stdout, "--- End Dockerfile ---")
	require.Contains(t, stdout, "FROM")
}

func TestFailsWhenOutputFlagMissing(t *testing.T) {
	// Arrange
	bin := buildBinary(t)

	// Act: run without --output
	cmd := exec.Command(bin, "--source", "/tmp")
	out, err := cmd.CombinedOutput()

	// Assert
	require.Error(t, err, "should fail without --output")
	require.Contains(t, string(out), "--output is required")
}

// extractCMDLine returns the CMD instruction from a Dockerfile string.
func extractCMDLine(dockerfile string) string {
	for _, line := range strings.Split(dockerfile, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "CMD ") {
			return trimmed
		}
	}
	return ""
}
