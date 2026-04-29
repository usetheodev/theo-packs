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

// copyExampleToTemp copies examples/<name>/ to a fresh temp dir and returns
// the destination path. Tests use this to isolate CLI side effects (the CLI
// writes .dockerignore into the source dir, which would otherwise pollute
// the working tree).
func copyExampleToTemp(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join(projectRoot(t), "examples", name)
	dst := t.TempDir()

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	require.NoError(t, err, "failed to copy example %q", name)
	return dst
}

func TestGenerateNodeNpm(t *testing.T) {
	// Arrange
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "node-npm")
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
	source := copyExampleToTemp(t, "go-simple")
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
	source := copyExampleToTemp(t, "node-npm")
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

// --- Phase 4: .dockerignore default generation ---

func TestCLI_WritesDockerignore_WhenAbsent(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "node-npm")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	// Sanity: the temp copy starts WITHOUT a .dockerignore.
	_, err := os.Stat(filepath.Join(source, ".dockerignore"))
	require.True(t, os.IsNotExist(err), ".dockerignore must not exist before CLI runs")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "node-npm",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "binary failed: %s", string(out))

	// CLI must have written a Node-flavored .dockerignore.
	content, err := os.ReadFile(filepath.Join(source, ".dockerignore"))
	require.NoError(t, err, "CLI did not write .dockerignore")
	require.Contains(t, string(content), "node_modules/")
	require.Contains(t, string(content), ".git/")

	// stderr should announce the write.
	require.Contains(t, string(out), "Wrote default .dockerignore")
}

func TestCLI_PreservesUserDockerignore(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "node-npm")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	userIgnore := "# user-controlled\n!important.txt\n"
	err := os.WriteFile(filepath.Join(source, ".dockerignore"), []byte(userIgnore), 0644)
	require.NoError(t, err)

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "node-npm",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "binary failed: %s", string(out))

	// User content must be untouched.
	content, err := os.ReadFile(filepath.Join(source, ".dockerignore"))
	require.NoError(t, err)
	require.Equal(t, userIgnore, string(content))

	// stderr should announce the skip.
	require.Contains(t, string(out), "User-provided .dockerignore found")
}

func TestCLI_HandlesReadonlySourceGracefully(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "node-npm")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	// Make the source dir read-only AFTER copying. CLI should log but still
	// produce the Dockerfile.
	require.NoError(t, os.Chmod(source, 0555))
	t.Cleanup(func() { _ = os.Chmod(source, 0755) })

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "node-npm",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()

	// Read-only dir means the Dockerfile-write attempt to source/Dockerfile
	// might also fail — but our CLI writes to --output (a separate temp dir).
	// The .dockerignore write is the only thing that touches source/. Either
	// the binary succeeded with a logged warning OR it succeeded silently
	// (file simply wasn't written). We just confirm no .dockerignore was
	// written and the binary did produce a Dockerfile in --output.
	require.NoError(t, err, "binary should not abort on .dockerignore write failure: %s", string(out))

	_, statErr := os.Stat(filepath.Join(source, ".dockerignore"))
	require.True(t, os.IsNotExist(statErr), "no .dockerignore should have been written to read-only source")

	df, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	require.Contains(t, string(df), "FROM")
}

// --- --app-name → THEOPACKS_APP_NAME bridge for non-Node workspaces ---

// TestCLI_BridgesAppNameToCargoWorkspace verifies the regression path: a
// Cargo workspace previously failed because --app-name was only bridged to
// THEOPACKS_APP_NAME for Node-shaped workspaces. With the bridge unified,
// rust/ruby/php/dotnet/deno workspaces all work the same way Node does.
func TestCLI_BridgesAppNameToCargoWorkspace(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "rust-workspace")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "api",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Cargo workspace should accept --app-name=api: %s", string(out))

	df, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	require.Contains(t, string(df), "cargo build --release -p api",
		"Dockerfile must build the selected workspace member")
}

func TestCLI_BridgesAppNameToRubyMonorepo(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "ruby-monorepo")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "api",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Ruby monorepo should accept --app-name=api: %s", string(out))

	df, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	require.Contains(t, string(df), "apps/api",
		"Dockerfile must reference the selected app")
}

func TestCLI_BridgesAppNameToPhpMonorepo(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "php-monorepo")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "api",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "PHP monorepo should accept --app-name=api: %s", string(out))

	df, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	require.Contains(t, string(df), "apps/api",
		"Dockerfile must reference the selected app")
}

// TestCLI_NoBridgeWhenAppNameEmpty asserts the empty-default behavior: a
// single-app project (no monorepo) with no --app-name flag is unaffected,
// because we don't bridge an empty string into THEOPACKS_APP_NAME (which
// would otherwise look for a literal app named "").
func TestCLI_NoBridgeWhenAppNameEmpty(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "go-simple")
	outputFile := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		// no --app-name
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "single-app project should not require --app-name: %s", string(out))

	df, readErr := os.ReadFile(outputFile)
	require.NoError(t, readErr)
	require.Contains(t, string(df), "FROM")
}
