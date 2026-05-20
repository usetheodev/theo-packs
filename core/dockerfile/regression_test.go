package dockerfile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
)

// Regression tests for CRITICAL findings closed by deep-review-hardening-
// plan. Each test asserts a property of the generated Dockerfile that
// would re-emerge if the corresponding fix were reverted.
//
// C1 (spurious secret mounts): T1.4 of deep-review-hardening filtered
// THEOPACKS_APP_NAME / APP_PATH / *_VERSION out of plan.Secrets so the
// renderer no longer emits `--mount=type=secret,id=THEOPACKS_*`. These
// tests gate the invariant END-TO-END (provider + context + renderer),
// catching regressions that unit tests on GenerateConfigFromEnvironment
// alone would miss (e.g., a provider opting back into Secrets = ["*"]).

const spuriousSecretIDPrefix = "--mount=type=secret,id=THEOPACKS_"

func generateTurborepo(t *testing.T, opts *core.GenerateBuildPlanOptions, env map[string]string) string {
	t.Helper()
	dir := filepath.Join(examplesDir(t), "node-turborepo")
	userApp, err := app.NewApp(dir)
	require.NoError(t, err)

	var envPtr *map[string]string
	if env != nil {
		envPtr = &env
	}
	appEnv := app.NewEnvironment(envPtr)

	if opts == nil {
		opts = &core.GenerateBuildPlanOptions{}
	}
	result := core.GenerateBuildPlan(userApp, appEnv, opts)
	require.True(t, result.Success, "plan failed: %v", result.Logs)

	df, err := Generate(result.Plan)
	require.NoError(t, err)
	return df
}

// TestRegression_Turborepo_OptionsBridge_NoSpuriousSecrets — the typed
// WorkspaceTarget path (T3.1 of hardening) must not produce spurious
// secret mounts. This is the new canonical path; if it leaks, the C1 fix
// is dead.
func TestRegression_Turborepo_OptionsBridge_NoSpuriousSecrets(t *testing.T) {
	t.Parallel()
	df := generateTurborepo(t,
		&core.GenerateBuildPlanOptions{
			WorkspaceTarget: &core.WorkspaceTarget{
				AppName: "api",
				AppPath: "apps/api",
			},
		},
		nil,
	)
	requireNoSpuriousSecretMounts(t, df)
}

// TestRegression_Turborepo_EnvBridge_NoSpuriousSecrets — the legacy
// env-var bridge (still supported during the deprecation window) must
// equally never leak THEOPACKS_* names into secret mounts.
func TestRegression_Turborepo_EnvBridge_NoSpuriousSecrets(t *testing.T) {
	t.Parallel()
	df := generateTurborepo(t, nil, map[string]string{
		"THEOPACKS_APP_NAME": "api",
		"THEOPACKS_APP_PATH": "apps/api",
	})
	requireNoSpuriousSecretMounts(t, df)
}

// TestRegression_Turborepo_BothBridges_NoSpuriousSecrets — defensive: if
// both bridges are populated simultaneously, the union must still be
// clean. Belt-and-suspenders against a future refactor that merges them
// wrong.
func TestRegression_Turborepo_BothBridges_NoSpuriousSecrets(t *testing.T) {
	t.Parallel()
	df := generateTurborepo(t,
		&core.GenerateBuildPlanOptions{
			WorkspaceTarget: &core.WorkspaceTarget{AppName: "api", AppPath: "apps/api"},
		},
		map[string]string{
			"THEOPACKS_APP_NAME": "api",
			"THEOPACKS_APP_PATH": "apps/api",
		},
	)
	requireNoSpuriousSecretMounts(t, df)
}

// TestRegression_Turborepo_LanguageVersionEnv_NoSpuriousSecrets — pinning
// node version via env var must also be filtered (THEOPACKS_*_VERSION
// family).
func TestRegression_Turborepo_LanguageVersionEnv_NoSpuriousSecrets(t *testing.T) {
	t.Parallel()
	df := generateTurborepo(t, nil, map[string]string{
		"THEOPACKS_NODE_VERSION": "20",
		"THEOPACKS_APP_NAME":     "api",
		"THEOPACKS_APP_PATH":     "apps/api",
	})
	requireNoSpuriousSecretMounts(t, df)
}

// requireNoSpuriousSecretMounts surfaces the offending line on failure
// so the diff in CI is actionable. Caller passes the full Dockerfile.
func requireNoSpuriousSecretMounts(t *testing.T, df string) {
	t.Helper()
	if !strings.Contains(df, spuriousSecretIDPrefix) {
		return
	}
	// Find the line and emit it.
	for _, line := range strings.Split(df, "\n") {
		if strings.Contains(line, spuriousSecretIDPrefix) {
			t.Fatalf(
				"spurious THEOPACKS_* secret mount detected — see C1 in deep-review-hardening-plan.md:\n  %s\n\nFull Dockerfile:\n%s",
				strings.TrimSpace(line), df,
			)
		}
	}
	// Defensive — Contains was true but no line matched? Bug in test.
	t.Fatalf("Contains() true but no line found; full Dockerfile:\n%s", df)
}
