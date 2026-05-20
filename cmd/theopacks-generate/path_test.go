package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClampPath_AllowsDot(t *testing.T) {
	root := t.TempDir()
	got, err := clampPath(root, ".")
	require.NoError(t, err)
	require.Equal(t, root, got)
}

func TestClampPath_AllowsNested(t *testing.T) {
	root := t.TempDir()
	got, err := clampPath(root, "apps/api")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "apps", "api"), got)
}

func TestClampPath_AllowsDotPrefix(t *testing.T) {
	root := t.TempDir()
	got, err := clampPath(root, "./apps")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "apps"), got)
}

func TestClampPath_RejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	_, err := clampPath(root, "../etc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes")
}

func TestClampPath_RejectsDeepTraversal(t *testing.T) {
	root := t.TempDir()
	_, err := clampPath(root, "apps/../../etc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes")
}

func TestClampPath_RejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	_, err := clampPath(root, "/etc/passwd")
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute")
}

// TestMain_RejectsAppPathTraversal_ExitCode2 — integration: --app-path
// that escapes --source must exit with code 2.
func TestMain_RejectsAppPathTraversal_ExitCode2(t *testing.T) {
	bin := buildBinary(t)
	source := t.TempDir()
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", "../etc",
		"--app-name", "",
		"--output", out,
	)
	combined, err := cmd.CombinedOutput()
	require.Error(t, err)

	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	require.Equal(t, 2, ee.ExitCode(), "expected exit 2 for path traversal, got %d. output: %s", ee.ExitCode(), string(combined))
	require.Contains(t, string(combined), "escapes")

	_, statErr := os.Stat(out)
	require.True(t, os.IsNotExist(statErr))
}
