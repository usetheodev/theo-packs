package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCLIInput_AcceptsGoodInputs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                          string
		source, appPath, appName, out string
	}{
		{"dot-app-path", "/workspace", ".", "", "/out/Dockerfile"},
		{"nested-app-path", "/workspace", "apps/api", "api", "/out/Dockerfile"},
		{"scoped-npm-name", "/workspace", "apps/api", "@theo/api", "/out/Dockerfile"},
		{"underscore-name", "/workspace", "apps/api", "my_api", "/out/Dockerfile"},
		{"relative-source", "examples/node-npm", ".", "demo", "/tmp/df"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateCLIInput(c.source, c.appPath, c.appName, c.out)
			require.NoError(t, err)
		})
	}
}

func TestValidateCLIInput_RejectsAppPathTraversalChars(t *testing.T) {
	t.Parallel()
	// `..` is allowed textually (clampPath handles escape detection), but
	// shell metas must be rejected here.
	err := validateCLIInput("/workspace", "apps;rm -rf /", "", "/out/D")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--app-path")
}

func TestValidateCLIInput_RejectsAppNameInjection(t *testing.T) {
	t.Parallel()
	err := validateCLIInput("/workspace", ".", "api;rm -rf /", "/out/D")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--app-name")
}

func TestValidateCLIInput_RejectsAppNameLeadingDash(t *testing.T) {
	t.Parallel()
	// A leading dash would look like a flag to downstream tools.
	err := validateCLIInput("/workspace", ".", "-bad", "/out/D")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--app-name")
}

func TestValidateCLIInput_RejectsEmptySource(t *testing.T) {
	t.Parallel()
	err := validateCLIInput("", ".", "", "/out/D")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--source")
}

func TestValidateCLIInput_RejectsEmptyAppPath(t *testing.T) {
	t.Parallel()
	err := validateCLIInput("/workspace", "", "", "/out/D")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--app-path")
}

func TestValidateCLIInput_RejectsEmptyOutput(t *testing.T) {
	t.Parallel()
	err := validateCLIInput("/workspace", ".", "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--output")
}

func TestValidateCLIInput_RejectsWhitespaceInPath(t *testing.T) {
	t.Parallel()
	err := validateCLIInput("/workspace", "apps with space", "", "/out/D")
	require.Error(t, err)
}

func TestValidateCLIInput_RejectsShellMetaInSource(t *testing.T) {
	t.Parallel()
	err := validateCLIInput("/workspace`whoami`", ".", "", "/out/D")
	require.Error(t, err)
}

func TestValidateCLIInput_AllowsDotsAndParents(t *testing.T) {
	t.Parallel()
	// `..` is allowed textually — clampPath enforces the escape check.
	err := validateCLIInput("/workspace", "../foo", "", "/out/D")
	require.NoError(t, err)
}

// TestMain_RejectsBadAppPath_ExitCode2 verifies the integration: when the
// binary receives an invalid --app-path, it must exit with code 2 and
// stderr identifies the offending field.
func TestMain_RejectsBadAppPath_ExitCode2(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", "/workspace",
		"--app-path", "apps;rm -rf /",
		"--app-name", "",
		"--output", out,
	)
	combined, err := cmd.CombinedOutput()
	require.Error(t, err)

	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	require.Equal(t, 2, ee.ExitCode(), "expected exit code 2 for input invariant violation, got %d", ee.ExitCode())
	require.Contains(t, string(combined), "--app-path")

	_, statErr := os.Stat(out)
	require.True(t, os.IsNotExist(statErr), "no Dockerfile should be written on invalid input")
}

// TestMain_RejectsBadAppName_ExitCode2 mirrors the above for --app-name.
func TestMain_RejectsBadAppName_ExitCode2(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t)
	out := filepath.Join(t.TempDir(), "Dockerfile")

	cmd := exec.Command(bin,
		"--source", "/workspace",
		"--app-path", ".",
		"--app-name", "api;ls",
		"--output", out,
	)
	combined, err := cmd.CombinedOutput()
	require.Error(t, err)

	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	require.Equal(t, 2, ee.ExitCode())
	require.Contains(t, string(combined), "--app-name")
}

// FuzzValidateCLIInput_NeverPanics — T3.2 adversarial defense.
//
// The function must reject or accept every input, never panic. Captures
// regex backtracking bugs, nil deref on edge cases, and similar
// failure modes that the seed table can't cover. Corpus persisted under
// testdata/fuzz/FuzzValidateCLIInput_NeverPanics/.
func FuzzValidateCLIInput_NeverPanics(f *testing.F) {
	f.Add("/workspace", ".", "", "/out/Dockerfile")
	f.Add("/workspace", "apps/api", "api", "/out/Df")
	f.Add("", "", "", "")
	f.Add("../../etc", ".", "", "/out")
	f.Add("/workspace", "../../../../../etc/passwd", "evil;rm", "/out")
	f.Add("\x00", "\x00", "\x00", "\x00")
	f.Add(strings.Repeat("a", 4096), "x", "y", "z")
	f.Add("/workspace", strings.Repeat("../", 100)+"etc", "", "/out")

	f.Fuzz(func(t *testing.T, source, appPath, appName, output string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on source=%q appPath=%q appName=%q output=%q: %v",
					source, appPath, appName, output, r)
			}
		}()
		_ = validateCLIInput(source, appPath, appName, output)
	})
}
