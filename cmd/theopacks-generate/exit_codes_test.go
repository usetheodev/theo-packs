package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFileForTest(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee, "expected *exec.ExitError, got %T: %v", err, err)
	return ee.ExitCode()
}

// TestExitCode_Success — happy path returns 0.
func TestExitCode_Success(t *testing.T) {
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "go-simple")
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "demo",
		"--output", out,
	)
	combined, err := cmd.CombinedOutput()
	require.NoError(t, err, "expected exit 0: %s", string(combined))
	require.Equal(t, 0, exitCode(t, err))
}

// TestExitCode_InputInvariant_BadFlag — sanitization rejects → exit 2.
func TestExitCode_InputInvariant_BadFlag(t *testing.T) {
	bin := buildBinary(t)
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", "/workspace",
		"--app-path", "apps;rm",
		"--output", out,
	)
	_, err := cmd.CombinedOutput()
	require.Equal(t, 2, exitCode(t, err))
}

// TestExitCode_InputInvariant_Traversal — clampPath rejects → exit 2.
func TestExitCode_InputInvariant_Traversal(t *testing.T) {
	bin := buildBinary(t)
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", t.TempDir(),
		"--app-path", "../etc",
		"--output", out,
	)
	_, err := cmd.CombinedOutput()
	require.Equal(t, 2, exitCode(t, err))
}

// TestExitCode_InputInvariant_UserDockerfile — hard-fail user Dockerfile → exit 2.
func TestExitCode_InputInvariant_UserDockerfile(t *testing.T) {
	bin := buildBinary(t)
	source := t.TempDir()
	require.NoError(t, writeFileForTest(filepath.Join(source, "Dockerfile"), "FROM alpine\n"))

	out := filepath.Join(t.TempDir(), "Dockerfile")
	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--output", out,
	)
	_, err := cmd.CombinedOutput()
	require.Equal(t, 2, exitCode(t, err))
}

// TestExitCode_GenericFailure_NoProvider — provider detection fails on
// an empty source → exit 1, not 2 (the source is structurally valid;
// just nothing to build).
func TestExitCode_GenericFailure_NoProvider(t *testing.T) {
	bin := buildBinary(t)
	source := t.TempDir() // empty dir
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--output", out,
	)
	_, err := cmd.CombinedOutput()
	require.Equal(t, 1, exitCode(t, err))
}
