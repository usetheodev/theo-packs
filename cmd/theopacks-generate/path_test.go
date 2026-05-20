package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClampPath_AllowsDot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := clampPath(root, ".")
	require.NoError(t, err)
	require.Equal(t, root, got)
}

func TestClampPath_AllowsNested(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := clampPath(root, "apps/api")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "apps", "api"), got)
}

func TestClampPath_AllowsDotPrefix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got, err := clampPath(root, "./apps")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "apps"), got)
}

func TestClampPath_RejectsParentTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := clampPath(root, "../etc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes")
}

func TestClampPath_RejectsDeepTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := clampPath(root, "apps/../../etc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes")
}

func TestClampPath_RejectsAbsolutePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := clampPath(root, "/etc/passwd")
	require.Error(t, err)
	require.Contains(t, err.Error(), "absolute")
}

// TestMain_RejectsAppPathTraversal_ExitCode2 — integration: --app-path
// that escapes --source must exit with code 2.
func TestMain_RejectsAppPathTraversal_ExitCode2(t *testing.T) {
	t.Parallel()
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

// FuzzClampPath_NeverEscapes — T3.3 adversarial defense.
//
// Invariant: for any (root, sub) where clampPath returns (p, nil), p
// must be inside rootAbs (p == rootAbs or strings.HasPrefix(p+sep,
// rootAbs+sep)). Also: clampPath must never panic.
//
// Corpus persisted under testdata/fuzz/FuzzClampPath_NeverEscapes/.
func FuzzClampPath_NeverEscapes(f *testing.F) {
	f.Add("/workspace", "apps/api")
	f.Add("/workspace", "..")
	f.Add("/workspace", "../etc")
	f.Add("/workspace", "./apps/../../../etc")
	f.Add("/workspace", "/etc/passwd")
	f.Add(".", "foo")
	f.Add("/", "")
	f.Add("/workspace", strings.Repeat("../", 200)+"etc")

	f.Fuzz(func(t *testing.T, root, sub string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on (root=%q, sub=%q): %v", root, sub, r)
			}
		}()
		p, err := clampPath(root, sub)
		if err != nil {
			return // rejection is fine
		}
		rootAbs, absErr := filepath.Abs(root)
		if absErr != nil {
			t.Fatalf("clampPath returned ok for unresolvable root %q", root)
		}
		sep := string(filepath.Separator)
		if p != rootAbs && !strings.HasPrefix(p+sep, rootAbs+sep) {
			t.Fatalf("INVARIANT VIOLATED: clampPath(%q, %q) = %q escapes %q",
				root, sub, p, rootAbs)
		}
	})
}
