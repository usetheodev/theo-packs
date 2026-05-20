package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestUserDockerfile_RejectsSymlink — defense against TOCTOU /
// info-disclosure via a Dockerfile symlinked to a sensitive path.
// The CLI must Lstat (not Stat) and reject any symlink before reading.
func TestUserDockerfile_RejectsSymlink(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	source := t.TempDir()

	// Symlink the "user Dockerfile" to /etc/hostname (always exists on
	// Linux) — if the CLI follows the link, behavior would change based
	// on the target's content, leaking it.
	require.NoError(t, os.Symlink("/etc/hostname", filepath.Join(source, "Dockerfile")))

	output := filepath.Join(t.TempDir(), "Dockerfile.out")
	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "test",
		"--output", output,
	)
	combined, err := cmd.CombinedOutput()
	require.Error(t, err)

	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	require.Equal(t, 2, ee.ExitCode())
	require.Contains(t, string(combined), "symbolic link")

	_, statErr := os.Stat(output)
	require.True(t, os.IsNotExist(statErr), "output must NOT be written when symlink is detected")
}

// TestDockerignore_SkipsSymlink — a .dockerignore symlink must not be
// followed nor overwritten. The CLI logs "symlink" and proceeds without
// writing a default.
func TestDockerignore_SkipsSymlink(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	source := copyExampleToTemp(t, "node-npm")

	require.NoError(t, os.Symlink("/etc/hostname", filepath.Join(source, ".dockerignore")))

	outputFile := filepath.Join(t.TempDir(), "Dockerfile")
	cmd := exec.Command(bin,
		"--source", source,
		"--app-path", ".",
		"--app-name", "node-npm",
		"--output", outputFile,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "binary must succeed despite symlink dockerignore: %s", string(out))

	// .dockerignore must remain the symlink (not overwritten).
	info, lstatErr := os.Lstat(filepath.Join(source, ".dockerignore"))
	require.NoError(t, lstatErr)
	require.NotZero(t, info.Mode()&os.ModeSymlink, ".dockerignore must remain a symlink")

	require.Contains(t, string(out), "symlink")
}
