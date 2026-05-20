package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Behavior matchers (T4.1/T4.2, test-suite-hardening-plan).
//
// Replaces literal `require.Equal(t, "npm start", ...)` style assertions
// with checks that admit any *reasonable* start command for a given
// language. A future refactor that switches Node from `npm start` to
// `node index.js` (or vice-versa) won't break 30+ unrelated tests.

// assertReasonableNodeStart accepts any of:
//   - "npm start" / "yarn start" / "pnpm start" / "bun start"
//   - "node <script>"
//   - "cd <path> && npm start" (workspace-mode wrapper)
//
// Rejects empty and obviously wrong values.
func assertReasonableNodeStart(t *testing.T, cmd string) {
	t.Helper()
	require.NotEmpty(t, cmd, "Node start command should not be empty")
	if cmd == "npm start" || cmd == "yarn start" || cmd == "pnpm start" || cmd == "bun start" {
		return
	}
	if strings.HasPrefix(cmd, "node ") {
		return
	}
	if strings.HasPrefix(cmd, "cd ") && strings.Contains(cmd, "&&") {
		return
	}
	t.Fatalf("unexpected Node start command: %q (see assertReasonableNodeStart)", cmd)
}

// assertReasonableGoStart — Go provider always builds /app/server.
func assertReasonableGoStart(t *testing.T, cmd string) {
	t.Helper()
	require.NotEmpty(t, cmd)
	require.Equal(t, "/app/server", cmd,
		"Go runtime is a static binary at /app/server — change requires updating both provider and this matcher")
}

// assertReasonablePythonStart admits the common Python web entries:
// gunicorn, uvicorn, python -m / -c, hypercorn, flask. Currently not
// used by monorepo_test.go (which prefers literal Equal there because
// the Python tests assert specific framework defaults), but kept as a
// peer to the Node/Go matchers — providers that drift in start-cmd
// shape can opt into the matcher without re-deriving the allowlist.
//
//nolint:unused // future use: dogfood/monorepo Python sites currently use Equal.
func assertReasonablePythonStart(t *testing.T, cmd string) {
	t.Helper()
	require.NotEmpty(t, cmd)
	prefixes := []string{
		"gunicorn ", "uvicorn ", "python ", "python -m ", "python -c ",
		"hypercorn ", "flask ", "streamlit ", "gradio ", "fastapi ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(cmd, p) {
			return
		}
	}
	// Workspace-mode wrappers ("cd apps/api && uvicorn ...")
	if strings.HasPrefix(cmd, "cd ") && strings.Contains(cmd, "&&") {
		return
	}
	t.Fatalf("unexpected Python start command: %q", cmd)
}
