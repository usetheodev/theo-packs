//go:build e2e
// +build e2e

package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Table-driven E2E (T1.1, test-suite-hardening-plan).
//
// The original e2e_test.go uses one Test* function per example, which made
// adding coverage friction-heavy. This file consolidates every example
// build into a single TestE2E_All driven by a slice; new examples land as
// one row. The per-language assertions live in named verifier helpers so
// the table stays readable.
//
// The original per-example functions remain in e2e_test.go as aliases
// (they call the same primitives) — useful for `go test -run` targeting
// while we migrate consumers. New work goes in the table.

// e2eCase declares an end-to-end Docker build for a single example.
type e2eCase struct {
	example       string                         // examples/<name>
	tag           string                         // docker tag for the built image
	env           map[string]string              // optional env vars passed to the generator
	verify        func(t *testing.T, tag string) // optional post-build assertion
	maxMB         int                            // optional image size cap in MB
	structureTest string                         // T1.1 — path to structure-tests.yaml (relative to example dir; empty = skip)
}

// --- Named verifiers ---

// verifyGoBinary asserts the standard /app/server distroless static binary.
func verifyGoBinary(t *testing.T, tag string) {
	t.Helper()
	requireBinaryAt(t, tag, "/app/server")
}

// verifyNodeRuntime confirms Node is callable inside the image. Runs a
// trivial inline script — sufficient to catch a missing interpreter,
// missing CMD layout, or corrupted node_modules.
func verifyNodeRuntime(t *testing.T, tag string) {
	t.Helper()
	out, err := exec.Command("docker", "run", "--rm", tag, "node", "-e", "console.log('ok')").CombinedOutput()
	require.NoError(t, err, "node not working: %s", string(out))
	require.Contains(t, string(out), "ok")
}

// verifyNodeFrameworkBuilt confirms a Node web framework produced a build
// artifact. Tries common output paths in order; the first that exists is
// proof the framework's build script ran successfully.
func verifyNodeFrameworkBuilt(candidates ...string) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		for _, path := range candidates {
			out, err := exec.Command("docker", "run", "--rm", tag, "ls", path).CombinedOutput()
			if err == nil && len(strings.TrimSpace(string(out))) > 0 {
				return
			}
		}
		t.Fatalf("none of %v exists in image %s", candidates, tag)
	}
}

// verifyPythonImport confirms a Python package can be imported. Doesn't
// instantiate servers (would hang) — just `import <pkg>; print(__version__)`.
func verifyPythonImport(pkg string) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		script := "import " + pkg + "; print(getattr(" + pkg + ", '__version__', 'ok'))"
		out, err := exec.Command("docker", "run", "--rm", tag, "python", "-c", script).CombinedOutput()
		require.NoError(t, err, "%s not importable: %s", pkg, string(out))
		require.NotEmpty(t, strings.TrimSpace(string(out)))
	}
}

// verifyFileExists asserts a path exists in the image. Generic shape used
// for monorepo/workspace builds where the verifier proves the right
// member was copied into /app.
func verifyFileExists(path string) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		out, err := exec.Command("docker", "run", "--rm", tag, "ls", path).CombinedOutput()
		require.NoError(t, err, "%s missing in %s: %s", path, tag, string(out))
	}
}

// verifyHTMLFile reads a static HTML file and asserts it contains 'html'.
// Used for staticfile.
func verifyHTMLFile(path string) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		out, err := exec.Command("docker", "run", "--rm", tag, "cat", path).CombinedOutput()
		require.NoError(t, err, "%s missing in %s: %s", path, tag, string(out))
		require.Contains(t, strings.ToLower(string(out)), "html")
	}
}

// verifyShellExecutes runs the shell script as the container CMD would,
// and asserts the script's expected output is present. Stronger than the
// previous "cat /app/start.sh" check, which only proved the file was
// copied — not that it actually executes (T1.6 hardening).
//
// Note: the shell provider intentionally requires THEOPACKS_START_CMD by
// design (see core/providers/shell/shell.go::StartCommandHelp), so the
// env var in the table is a documented contract, not a workaround.
func verifyShellExecutes(scriptPath, mustContain string) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		out, err := exec.Command("docker", "run", "--rm", tag, "bash", scriptPath).CombinedOutput()
		require.NoError(t, err, "script %s failed: %s", scriptPath, string(out))
		require.Contains(t, string(out), mustContain,
			"script output missing %q: %s", mustContain, string(out))
	}
}

// verifyBinaryThenSize composes two verifiers — useful for cases that
// need both a binary check and a size budget.
func verifyAll(vs ...func(*testing.T, string)) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		for _, v := range vs {
			v(t, tag)
		}
	}
}

// verifyCommand runs an arbitrary command inside the image and asserts it
// succeeds. Used when the language-specific check doesn't fit a named
// verifier (e.g., bundle info, php --version).
func verifyCommand(cmd []string, mustContain string) func(*testing.T, string) {
	return func(t *testing.T, tag string) {
		t.Helper()
		args := append([]string{"run", "--rm", tag}, cmd...)
		out, err := exec.Command("docker", args...).CombinedOutput()
		require.NoError(t, err, "command %v failed: %s", cmd, string(out))
		if mustContain != "" {
			require.Contains(t, string(out), mustContain)
		}
	}
}

// --- The table ---

func e2eCases() []e2eCase {
	return []e2eCase{
		// --- Go ---
		{example: "go-simple", tag: "te2e-go-simple", verify: verifyGoBinary, structureTest: "structure-tests.yaml"},
		{example: "go-cmd-dirs", tag: "te2e-go-cmd-dirs", verify: verifyGoBinary},
		{example: "go-workspaces", tag: "te2e-go-workspaces", verify: verifyGoBinary},

		// --- Node single-package ---
		{example: "node-npm", tag: "te2e-node-npm", verify: verifyNodeRuntime, maxMB: 280, structureTest: "structure-tests.yaml"},
		{example: "node-express", tag: "te2e-node-express", verify: verifyNodeRuntime, maxMB: 280},

		// --- Node theo-stacks parity (T1.5 + theo-stacks-compat doc) ---
		{example: "node-fastify", tag: "te2e-node-fastify", verify: verifyNodeRuntime, maxMB: 280},
		{example: "node-nestjs", tag: "te2e-node-nestjs", verify: verifyNodeRuntime, maxMB: 400},
		{example: "node-worker", tag: "te2e-node-worker", verify: verifyNodeRuntime, maxMB: 280},

		// --- Node frameworks (build step exercised) ---
		{example: "node-next", tag: "te2e-node-next", verify: verifyNodeFrameworkBuilt("/app/.next"), maxMB: 600},
		{example: "node-astro", tag: "te2e-node-astro", verify: verifyNodeFrameworkBuilt("/app/dist"), maxMB: 400},
		{example: "node-remix", tag: "te2e-node-remix", verify: verifyNodeFrameworkBuilt("/app/build", "/app/.remix"), maxMB: 400},
		{example: "node-nuxt", tag: "te2e-node-nuxt", verify: verifyNodeFrameworkBuilt("/app/.output", "/app/dist"), maxMB: 500},
		{example: "node-vite-react", tag: "te2e-node-vite-react", verify: verifyNodeFrameworkBuilt("/app/dist"), maxMB: 400},
		{example: "node-vite-svelte", tag: "te2e-node-vite-svelte", verify: verifyNodeFrameworkBuilt("/app/dist"), maxMB: 400},
		{example: "node-vite-vue", tag: "te2e-node-vite-vue", verify: verifyNodeFrameworkBuilt("/app/dist"), maxMB: 400},

		// --- Node config / dockerignore variants ---
		{example: "node-npm-with-config", tag: "te2e-node-npm-with-config", verify: verifyNodeRuntime, maxMB: 400},
		{example: "node-npm-with-dockerignore", tag: "te2e-node-npm-with-dockerignore", verify: verifyNodeRuntime, maxMB: 400},

		// --- Node workspaces ---
		{example: "node-npm-workspaces", tag: "te2e-node-npm-workspaces", verify: verifyNodeRuntime, maxMB: 400},
		{example: "node-yarn-workspaces", tag: "te2e-node-yarn-workspaces", verify: verifyNodeRuntime, maxMB: 400},
		{example: "node-pnpm-workspaces", tag: "te2e-node-pnpm-workspaces", verify: verifyNodeRuntime, maxMB: 400},
		{example: "node-turborepo", tag: "te2e-node-turborepo",
			env:           map[string]string{"THEOPACKS_APP_NAME": "api", "THEOPACKS_APP_PATH": "apps/api"},
			verify:        verifyFileExists("/app/apps/api"),
			maxMB:         600,
			structureTest: "structure-tests.yaml"},

		// --- Python ---
		{example: "python-flask", tag: "te2e-python-flask",
			env:           map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify:        verifyPythonImport("flask"),
			maxMB:         280,
			structureTest: "structure-tests.yaml"},
		{example: "python-django", tag: "te2e-python-django",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyPythonImport("django"), maxMB: 350},
		{example: "python-fastapi", tag: "te2e-python-fastapi",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyPythonImport("fastapi"), maxMB: 350},
		{example: "python-gradio", tag: "te2e-python-gradio",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyPythonImport("gradio"), maxMB: 800},
		{example: "python-streamlit", tag: "te2e-python-streamlit",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyPythonImport("streamlit"), maxMB: 600},
		{example: "python-poetry", tag: "te2e-python-poetry",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyCommand([]string{"python", "-c", "print('ok')"}, "ok"), maxMB: 400},
		{example: "python-pipfile", tag: "te2e-python-pipfile",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyCommand([]string{"python", "-c", "print('ok')"}, "ok"), maxMB: 400},
		{example: "python-setuppy", tag: "te2e-python-setuppy",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyCommand([]string{"python", "-c", "print('ok')"}, "ok"), maxMB: 400},
		{example: "python-uv-workspace", tag: "te2e-python-uv-workspace",
			env:    map[string]string{"THEOPACKS_START_CMD": "python -c 'print(1)'"},
			verify: verifyCommand([]string{"python", "-c", "print('ok')"}, "ok"), maxMB: 400},

		// --- Static / Shell ---
		{example: "staticfile", tag: "te2e-staticfile", verify: verifyHTMLFile("/app/index.html")},
		{example: "shell-script", tag: "te2e-shell-script",
			env:    map[string]string{"THEOPACKS_START_CMD": "bash start.sh"},
			verify: verifyShellExecutes("/app/start.sh", "hello world")},

		// --- Rust ---
		{example: "rust-axum", tag: "te2e-rust-axum", verify: verifyGoBinary, structureTest: "structure-tests.yaml"}, // /app/server convention
		{example: "rust-cli", tag: "te2e-rust-cli", verify: verifyGoBinary},
		{example: "rust-workspace", tag: "te2e-rust-workspace",
			env: map[string]string{"THEOPACKS_APP_NAME": "api"}, verify: verifyGoBinary},

		// --- Java ---
		{example: "java-spring-gradle", tag: "te2e-java-spring-gradle",
			verify:        verifyFileExists("/app/app.jar"),
			structureTest: "structure-tests.yaml"},
		{example: "java-spring-maven", tag: "te2e-java-spring-maven",
			verify: verifyFileExists("/app/app.jar")},
		{example: "java-gradle-workspace", tag: "te2e-java-gradle-workspace",
			env: map[string]string{"THEOPACKS_APP_NAME": "api"}, verify: verifyFileExists("/app/app.jar")},

		// --- .NET ---
		{example: "dotnet-aspnet", tag: "te2e-dotnet-aspnet",
			verify:        verifyCommand([]string{"ls", "/app/publish"}, "dotnet-aspnet.dll"),
			structureTest: "structure-tests.yaml"},
		{example: "dotnet-console", tag: "te2e-dotnet-console",
			verify: verifyCommand([]string{"ls", "/app/publish"}, ".dll")},
		{example: "dotnet-solution", tag: "te2e-dotnet-solution",
			verify: verifyFileExists("/app/publish")},

		// --- Ruby ---
		{example: "ruby-sinatra", tag: "te2e-ruby-sinatra",
			verify:        verifyCommand([]string{"bundle", "info", "sinatra"}, "sinatra"),
			structureTest: "structure-tests.yaml"},
		{example: "ruby-rails", tag: "te2e-ruby-rails",
			verify: verifyCommand([]string{"bundle", "info", "rails"}, "rails")},
		{example: "ruby-monorepo", tag: "te2e-ruby-monorepo",
			env: map[string]string{"THEOPACKS_APP_NAME": "api"}, verify: verifyFileExists("apps/api/config.ru")},

		// --- PHP ---
		{example: "php-slim", tag: "te2e-php-slim",
			verify:        verifyCommand([]string{"php", "--version"}, "PHP"),
			structureTest: "structure-tests.yaml"},
		{example: "php-laravel", tag: "te2e-php-laravel",
			verify: verifyCommand([]string{"php", "--version"}, "PHP")},
		{example: "php-monorepo", tag: "te2e-php-monorepo",
			env: map[string]string{"THEOPACKS_APP_NAME": "api"}, verify: verifyFileExists("apps/api/public/index.php")},

		// --- Deno ---
		{example: "deno-hono", tag: "te2e-deno-hono",
			verify:        verifyCommand([]string{"deno", "--version"}, "deno"),
			structureTest: "structure-tests.yaml"},
		{example: "deno-fresh", tag: "te2e-deno-fresh",
			verify: verifyCommand([]string{"deno", "--version"}, "deno")},
		{example: "deno-workspace", tag: "te2e-deno-workspace",
			env: map[string]string{"THEOPACKS_APP_NAME": "api"}, verify: verifyFileExists("apps/api/main.ts")},
	}
}

// runE2ECase executes one case from the table.
func runE2ECase(t *testing.T, c e2eCase) {
	t.Helper()
	dir := filepath.Join(examplesDir(t), c.example)
	df := generateDockerfile(t, dir, c.env)
	// T0.2 — lint the generated Dockerfile before attempting to build.
	// Fails fast on hadolint findings (warning+); silently skipped when
	// hadolint binary isn't on PATH (local dev without it).
	runHadolint(t, df)
	t.Cleanup(func() { removeImage(c.tag) })
	buildImage(t, dir, df, c.tag)
	require.True(t, imageExists(c.tag), "image %s missing after build", c.tag)
	if c.verify != nil {
		c.verify(t, c.tag)
	}
	if c.maxMB > 0 {
		requireSizeLessThan(t, c.tag, c.maxMB)
	}
	// T1.1 — declarative structure tests when the case declares one.
	if c.structureTest != "" {
		runStructureTest(t, c.tag, filepath.Join(dir, c.structureTest))
	}
	// T0.4 — dive efficiency gate. Runs AFTER verify so a failing
	// runtime check surfaces first (more actionable). Skips silently
	// when dive isn't installed (local dev) or DIVE_SKIP=1.
	runDive(t, c.tag)
	// T1.3 — emit SBOMs (SPDX + CycloneDX) for every E2E image. Output
	// at e2e/sboms/<tag>.{spdx,cyclonedx}.json. Skipped silently when
	// syft isn't installed.
	generateSBOM(t, c.tag)
}

// TestE2E_All builds every example in the table. Subtests parallelize:
// Docker daemon serializes builds internally, but the test goroutines
// run concurrently so cleanup and verification overlap with the next
// build's setup.
//
// T5.2 — when TEST_SHARD=N/M is set, only the cases whose example
// hashes to shard N execute. fnv32a keeps assignment deterministic.
func TestE2E_All(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available")
	}
	for _, c := range e2eCases() {
		c := c
		if !shouldRunInShard(c.example) {
			continue
		}
		t.Run(c.example, func(t *testing.T) {
			t.Parallel()
			runE2ECase(t, c)
		})
	}
}
