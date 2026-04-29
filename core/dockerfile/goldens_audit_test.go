// Corpus-level audit tests that lock invariants across every golden
// Dockerfile. Goldens are regenerated frequently (UPDATE_GOLDEN=true),
// so without these checks a regression in the renderer or a provider
// could re-introduce a banned pattern (bash CMD, double sh-c, etc.)
// silently — the relevant per-example assertEqual would fail, but a
// reviewer accepting a UPDATE_GOLDEN diff might miss the implication.
//
// These tests scan every *.dockerfile under testdata/ and assert the
// invariants that must hold for ALL generated Dockerfiles, regardless
// of language, framework, or example.

package dockerfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// goldenFiles returns paths to every *.dockerfile under testdata/.
func goldenFiles(t *testing.T) []string {
	t.Helper()
	dir := goldenDir(t)
	matches, err := filepath.Glob(filepath.Join(dir, "*.dockerfile"))
	require.NoError(t, err)
	require.NotEmpty(t, matches, "no golden files found under %s", dir)
	return matches
}

// readGolden returns the contents of a golden file with normalized line
// endings so platform-specific checkouts don't trip the assertions.
func readGolden(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// TestGoldens_NoBashCMD prevents `CMD ["/bin/bash", "-c", ...]` from
// reappearing. /bin/bash is absent from debian:bookworm-slim and
// distroless images, so emitting it would crash the container at start.
func TestGoldens_NoBashCMD(t *testing.T) {
	for _, path := range goldenFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content := readGolden(t, path)
			require.NotContains(t, content, "/bin/bash",
				"golden uses /bin/bash; renderer must use exec form or /bin/sh -c (T1.1)")
		})
	}
}

// TestGoldens_NoDoubleShC prevents the `sh -c 'sh -c '...”` quoting
// collision that bit Java/Ruby/PHP. The renderer wraps in sh -c via
// CommandKindShell exactly once; pre-wrapping in providers would
// re-introduce the bug.
func TestGoldens_NoDoubleShC(t *testing.T) {
	needle := "sh -c 'sh -c '"
	for _, path := range goldenFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content := readGolden(t, path)
			require.NotContains(t, content, needle,
				"golden has nested sh -c — provider is pre-wrapping a body that the renderer also wraps (T1.2)")
		})
	}
}

// TestGoldens_NoQuoteInQuoteBundleConfig ensures the Ruby
// `bundle config without 'development test'` form (single-quote inside
// single-quote) doesn't sneak back in.
func TestGoldens_NoQuoteInQuoteBundleConfig(t *testing.T) {
	needle := `bundle config set --local without 'development test'`
	for _, path := range goldenFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			content := readGolden(t, path)
			require.NotContains(t, content, needle,
				"golden has the broken bundle config form; use `without development:test` (T1.3)")
		})
	}
}

// TestGoldens_DistrolessDeploysHaveNoUSERLine — distroless :nonroot
// runtimes (Go, Rust) already run as nonroot UID 65532 and don't need
// a USER directive. resolveDeployUser returns empty for those.
//
// Only integration_* goldens are checked; the bare unit-test fixtures
// like go_simple.dockerfile are hand-crafted with arbitrary base images
// and don't represent real provider output.
func TestGoldens_DistrolessDeploysHaveNoUSERLine(t *testing.T) {
	for _, path := range goldenFiles(t) {
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "integration_go_") &&
			!strings.HasPrefix(base, "integration_rust_") {
			continue
		}
		t.Run(base, func(t *testing.T) {
			content := readGolden(t, path)
			require.NotContains(t, content, "\nUSER ",
				"distroless :nonroot golden has a USER directive; it should rely on the image's built-in UID 65532 (T5.1)")
		})
	}
}

// TestGoldens_NonDistrolessDeploysHaveUSERLine — non-distroless
// runtimes get an explicit USER directive in the deploy stage.
func TestGoldens_NonDistrolessDeploysHaveUSERLine(t *testing.T) {
	// Provider-language prefixes that produce non-distroless deploys.
	mustHaveUSER := []string{
		"integration_java_",   // eclipse-temurin:<v>-jre
		"integration_dotnet_", // mcr.microsoft.com/dotnet/aspnet | runtime-alpine
		"integration_ruby_",   // ruby:3.3-bookworm-slim
		"integration_php_",    // php:8.1-cli-bookworm
		"integration_deno_",   // denoland/deno:2
		"integration_node_",   // node:20-bookworm-slim
		"integration_python_", // python:3.12-slim-bookworm
	}

	for _, path := range goldenFiles(t) {
		base := filepath.Base(path)
		match := false
		for _, prefix := range mustHaveUSER {
			if strings.HasPrefix(base, prefix) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		t.Run(base, func(t *testing.T) {
			content := readGolden(t, path)
			require.Contains(t, content, "\nUSER ",
				"non-distroless golden missing USER directive (T5.1)")
		})
	}
}

// TestGoldens_HTTPFrameworksHaveHEALTHCHECK — Spring Boot, ASP.NET,
// Rails/Sinatra/Rack, Laravel/Slim/Symfony, Fresh/Hono all set
// HealthcheckPath, so the renderer emits HEALTHCHECK. The integration
// goldens for these examples must include it.
func TestGoldens_HTTPFrameworksHaveHEALTHCHECK(t *testing.T) {
	mustHaveHealthcheck := []string{
		"integration_java_spring_gradle.dockerfile",
		"integration_java_spring_maven.dockerfile",
		"integration_java_gradle_workspace.dockerfile",
		"integration_dotnet_aspnet.dockerfile",
		"integration_dotnet_solution.dockerfile",
		"integration_ruby_sinatra.dockerfile",
		"integration_ruby_rails.dockerfile",
		"integration_php_slim.dockerfile",
		"integration_php_laravel.dockerfile",
		"integration_deno_hono.dockerfile",
		"integration_deno_fresh.dockerfile",
	}
	dir := goldenDir(t)
	for _, name := range mustHaveHealthcheck {
		t.Run(name, func(t *testing.T) {
			content := readGolden(t, filepath.Join(dir, name))
			require.Contains(t, content, "HEALTHCHECK ",
				"%s should declare a HEALTHCHECK (T5.2)", name)
		})
	}
}

// TestGoldens_PackageManagerStepsHaveCacheMounts — every install/build
// RUN that invokes a known package manager must have a corresponding
// cache mount. We sample one golden per language — the providers are
// shared across goldens of the same language so this is sufficient.
//
// Ruby is intentionally absent: bundler must install gems into a path that
// ends up in the final image (we use `bundle config --local path vendor/bundle`
// so gems live under /app/vendor/bundle, which is COPYed to the deploy stage).
// A cache mount at /usr/local/bundle would silently drop the gems because
// BuildKit cache mounts are not part of the resulting image layer.
func TestGoldens_PackageManagerStepsHaveCacheMounts(t *testing.T) {
	cases := []struct {
		golden string
		mount  string
	}{
		{"integration_go_simple.dockerfile", "target=/go/pkg/mod"},
		{"integration_rust_axum.dockerfile", "target=/root/.cargo/registry"},
		{"integration_java_spring_gradle.dockerfile", "target=/root/.gradle"},
		{"integration_java_spring_maven.dockerfile", "target=/root/.m2"},
		{"integration_dotnet_aspnet.dockerfile", "target=/root/.nuget/packages"},
		{"integration_php_slim.dockerfile", "target=/root/.composer/cache"},
		{"integration_node_npm.dockerfile", "target=/root/.npm"},
		{"integration_python_flask.dockerfile", "target=/root/.cache/pip"},
		{"integration_deno_hono.dockerfile", "target=/deno-dir"},
	}
	dir := goldenDir(t)
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			content := readGolden(t, filepath.Join(dir, c.golden))
			require.Contains(t, content, c.mount,
				"%s missing expected cache mount (T3.x)", c.golden)
		})
	}
}
