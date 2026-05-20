//go:build e2e
// +build e2e

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// theo-stacks template gating (T70-T74,
// docs/theo-stacks-compatibility.md).
//
// theo-packs is the Dockerfile generator for `create-theo` upstream
// (https://github.com/usetheodev/theo-stacks). Every template in
// theo-stacks/templates/ MUST be detect-able + plan-able by some
// provider in theo-packs. This gate runs the CLI binary against each
// template and asserts:
//   - it exits successfully
//   - the generated Dockerfile carries the expected provider name in
//     the defensive header
//   - hadolint accepts it
//
// Skips cleanly when the upstream checkout is missing (env
// THEO_STACKS_DIR overrides the default /tmp/theo-stacks/templates).

const theoStacksEnv = "THEO_STACKS_DIR"

func theoStacksDir(t *testing.T) string {
	t.Helper()
	if d := os.Getenv(theoStacksEnv); d != "" {
		return d
	}
	return "/tmp/theo-stacks/templates"
}

type templateExpectation struct {
	template   string
	provider   string
	appName    string
	appPath    string
	skipReason string
}

var templateExpectations = []templateExpectation{
	{template: "fullstack-nextjs", provider: "node"},
	{template: "go-api", provider: "go"},
	{template: "java-spring", provider: "java"},
	{template: "monorepo-go", provider: "go", appName: "api", appPath: "apps/api"},
	{template: "monorepo-java", provider: "java", appName: "api", appPath: "apps/api"},
	{template: "monorepo-php", provider: "php", appName: "api", appPath: "apps/api"},
	{template: "monorepo-python", provider: "python", appName: "api", appPath: "apps/api"},
	{template: "monorepo-ruby", provider: "ruby", appName: "api", appPath: "apps/api"},
	{template: "monorepo-rust", provider: "rust", appName: "api", appPath: "apps/api"},
	{template: "monorepo-turbo", provider: "node", appName: "api", appPath: "apps/api"},
	{template: "node-express", provider: "node"},
	{template: "node-fastify", provider: "node"},
	{template: "node-nestjs", provider: "node"},
	{template: "node-nextjs", provider: "node"},
	{template: "node-worker", provider: "node"},
	{template: "php-slim", provider: "php"},
	{template: "python-fastapi", provider: "python"},
	{template: "ruby-sinatra", provider: "ruby"},
	{template: "rust-axum", provider: "rust"},
}

// renderTemplate copies src → dst recursively, replacing
// "{{project-name}}" with "demo" and renaming `dockerignore` /
// `gitignore` to their dotfile equivalents (create-theo does this at
// scaffold time).
func renderTemplate(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		base := filepath.Base(rel)
		dir := filepath.Dir(rel)
		if base == "dockerignore" {
			rel = filepath.Join(dir, ".dockerignore")
		} else if base == "gitignore" {
			rel = filepath.Join(dir, ".gitignore")
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out := strings.ReplaceAll(string(data), "{{project-name}}", "demo")
		return os.WriteFile(target, []byte(out), info.Mode())
	})
	require.NoError(t, err)
}

// theoStacksBin compiles the binary once per TestMain-equivalent invocation
// of this file. Cached in a package var so 19 subtests share it.
var (
	theoStacksBinOnce sync.Once
	theoStacksBinPath string
)

func sharedTheoStacksBin(t *testing.T) string {
	t.Helper()
	theoStacksBinOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		require.True(t, ok)
		cmdDir := filepath.Join(filepath.Dir(thisFile), "..", "cmd", "theopacks-generate")
		dir, err := os.MkdirTemp("", "theo-stacks-bin-")
		require.NoError(t, err)
		theoStacksBinPath = filepath.Join(dir, "theopacks-generate")
		build := exec.Command("go", "build", "-o", theoStacksBinPath, ".")
		build.Dir = cmdDir
		build.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
		out, err := build.CombinedOutput()
		require.NoError(t, err, "build theo-stacks bin: %s", string(out))
	})
	return theoStacksBinPath
}

func runTheopacksOnTemplate(t *testing.T, rendered, appName, appPath string) string {
	t.Helper()
	bin := sharedTheoStacksBin(t)
	outFile := filepath.Join(t.TempDir(), "Dockerfile")

	args := []string{"--source", rendered, "--output", outFile}
	if appPath == "" {
		args = append(args, "--app-path", ".")
	} else {
		args = append(args, "--app-path", appPath)
	}
	if appName != "" {
		args = append(args, "--app-name", appName)
	}

	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "theopacks-generate failed for %s:\n%s", rendered, string(out))

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	return string(data)
}

// TestE2E_TheoStacksTemplates iterates every theo-stacks template and
// asserts theo-packs can produce a valid Dockerfile for it.
func TestE2E_TheoStacksTemplates(t *testing.T) {
	stacksDir := theoStacksDir(t)
	if _, err := os.Stat(stacksDir); err != nil {
		t.Skipf("theo-stacks templates not at %s — clone https://github.com/usetheodev/theo-stacks or set %s",
			stacksDir, theoStacksEnv)
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

			rendered := t.TempDir()
			renderTemplate(t, srcDir, rendered)

			df := runTheopacksOnTemplate(t, rendered, exp.appName, exp.appPath)

			// Lint output.
			runHadolint(t, df)

			require.True(t,
				strings.HasPrefix(df, "# syntax=docker/dockerfile:1"),
				"template %s must produce Dockerfile with syntax directive", exp.template)

			expected := `for provider "` + exp.provider + `"`
			require.Contains(t, df, expected,
				"template %s should be detected by provider %q; header line:\n%s",
				exp.template, exp.provider, headerLine(df))
		})
	}
}

func headerLine(df string) string {
	for _, line := range strings.Split(df, "\n") {
		if strings.Contains(line, "for provider") {
			return line
		}
	}
	return ""
}
