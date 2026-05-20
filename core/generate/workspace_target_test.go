package generate

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
)

// TestResolveAppName_PrefersWorkspaceTargetOverEnv — when both the typed
// WorkspaceTarget and the legacy env-var bridge are populated, the
// typed value wins.
func TestResolveAppName_PrefersWorkspaceTargetOverEnv(t *testing.T) {
	ctx := &GenerateContext{
		Env: app.NewEnvironment(&map[string]string{
			"THEOPACKS_APP_NAME": "legacy-name",
		}),
		WorkspaceTarget: &WorkspaceTarget{AppName: "typed-name"},
	}
	require.Equal(t, "typed-name", ctx.ResolveAppName())
}

// TestResolveAppName_FallsBackToEnv — single-app projects don't set
// WorkspaceTarget; the legacy env-var bridge must still work for
// backward compat.
func TestResolveAppName_FallsBackToEnv(t *testing.T) {
	ctx := &GenerateContext{
		Env: app.NewEnvironment(&map[string]string{
			"THEOPACKS_APP_NAME": "env-name",
		}),
	}
	require.Equal(t, "env-name", ctx.ResolveAppName())
}

// TestResolveAppName_EmptyWhenUnset — neither typed nor env: empty.
func TestResolveAppName_EmptyWhenUnset(t *testing.T) {
	ctx := &GenerateContext{Env: app.NewEnvironment(nil)}
	require.Equal(t, "", ctx.ResolveAppName())
}

// TestResolveAppPath_PrefersWorkspaceTargetOverEnv — mirror of the above
// for AppPath.
func TestResolveAppPath_PrefersWorkspaceTargetOverEnv(t *testing.T) {
	ctx := &GenerateContext{
		Env: app.NewEnvironment(&map[string]string{
			"THEOPACKS_APP_PATH": "legacy/path",
		}),
		WorkspaceTarget: &WorkspaceTarget{AppPath: "typed/path"},
	}
	require.Equal(t, "typed/path", ctx.ResolveAppPath())
}

func TestResolveAppPath_FallsBackToEnv(t *testing.T) {
	ctx := &GenerateContext{
		Env: app.NewEnvironment(&map[string]string{
			"THEOPACKS_APP_PATH": "apps/api",
		}),
	}
	require.Equal(t, "apps/api", ctx.ResolveAppPath())
}

// TestResolveAppName_EmptyWorkspaceTargetUsesEnv — an empty AppName on
// the typed target should NOT shadow the env-var fallback.
func TestResolveAppName_EmptyWorkspaceTargetUsesEnv(t *testing.T) {
	ctx := &GenerateContext{
		Env: app.NewEnvironment(&map[string]string{
			"THEOPACKS_APP_NAME": "env-name",
		}),
		WorkspaceTarget: &WorkspaceTarget{AppName: ""},
	}
	require.Equal(t, "env-name", ctx.ResolveAppName())
}
