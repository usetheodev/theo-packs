// Tests for the renderer's internal helpers introduced in the
// Dockerfile correctness + efficiency PR (Phase 0/1/2/4/5):
//
//   - emitCMD                 (T1.1)
//   - needsShell              (T1.1)
//   - autoDetectSecrets       (T0.2)
//   - secretReferencedIn      (T0.2)
//   - resolveDeployUser       (T5.1)
//
// Goldens cover the integrated behavior; these unit tests pin the
// invariants in isolation so a regression surfaces with a focused failure
// before the goldens diverge.

package dockerfile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/plan"
)

// emitCMD ---------------------------------------------------------------

func TestEmitCMD_PlainExec(t *testing.T) {
	var b strings.Builder
	emitCMD(&b, "/app/server")
	require.Equal(t, "CMD [\"/app/server\"]\n", b.String())
}

func TestEmitCMD_MultiWordExec(t *testing.T) {
	var b strings.Builder
	emitCMD(&b, "npm start")
	require.Equal(t, "CMD [\"npm\", \"start\"]\n", b.String())
}

func TestEmitCMD_DotnetWithDll(t *testing.T) {
	var b strings.Builder
	emitCMD(&b, "dotnet /app/MyApp.dll")
	require.Equal(t, "CMD [\"dotnet\", \"/app/MyApp.dll\"]\n", b.String())
}

func TestEmitCMD_EnvVarFallsBackToShell(t *testing.T) {
	var b strings.Builder
	emitCMD(&b, "rails server -p ${PORT:-3000}")
	out := b.String()
	require.Contains(t, out, `CMD ["/bin/sh", "-c"`)
	require.NotContains(t, out, "/bin/bash",
		"never emit /bin/bash — bash is absent from slim/distroless images")
}

func TestEmitCMD_PipeFallsBackToShell(t *testing.T) {
	var b strings.Builder
	emitCMD(&b, "tail -f /var/log/app.log | grep ERROR")
	require.Contains(t, b.String(), `CMD ["/bin/sh", "-c"`)
}

func TestEmitCMD_EmptyEmitsNothing(t *testing.T) {
	var b strings.Builder
	emitCMD(&b, "")
	require.Empty(t, b.String())
	emitCMD(&b, "   ")
	require.Empty(t, b.String())
}

// needsShell ------------------------------------------------------------

func TestNeedsShell_PlainCommand(t *testing.T) {
	require.False(t, needsShell("/app/server"))
	require.False(t, needsShell("npm start"))
	require.False(t, needsShell("dotnet /app/MyApp.dll"))
}

func TestNeedsShell_DetectsAllSpecialChars(t *testing.T) {
	tests := []string{
		"echo $PORT", // var expansion
		"a; b",       // sequence
		"a && b",     // and
		"a || b",     // or
		"a | b",      // pipe
		"foo > bar",  // redirect
		"foo < bar",  // redirect
		"$(pwd)",     // command substitution
		"foo {a,b}",  // brace expansion
		`echo "hi"`,  // double quote
		"echo 'hi'",  // single quote
		"`pwd`",      // backtick
		"foo*",       // glob
		"foo?",       // glob
		"foo\\bar",   // escape
	}
	for _, cmd := range tests {
		require.True(t, needsShell(cmd), "should flag %q as needing shell", cmd)
	}
}

// autoDetectSecrets -----------------------------------------------------

func TestAutoDetectSecrets_NoMatch(t *testing.T) {
	got := autoDetectSecrets("pip install -r requirements.txt", []string{"FOO", "BAR"})
	require.Nil(t, got)
}

func TestAutoDetectSecrets_DollarReference(t *testing.T) {
	got := autoDetectSecrets("echo $FOO", []string{"FOO", "BAR"})
	require.Equal(t, []string{"FOO"}, got)
}

func TestAutoDetectSecrets_BraceReference(t *testing.T) {
	got := autoDetectSecrets("echo ${FOO}", []string{"FOO", "BAR"})
	require.Equal(t, []string{"FOO"}, got)
}

func TestAutoDetectSecrets_MultipleSecrets(t *testing.T) {
	got := autoDetectSecrets("echo $BAR && echo ${FOO}", []string{"FOO", "BAR"})
	require.Equal(t, []string{"BAR", "FOO"}, got, "result must be sorted")
}

func TestAutoDetectSecrets_TokenBoundary(t *testing.T) {
	// $FOOBAR should NOT match a secret named FOO.
	got := autoDetectSecrets("echo $FOOBAR", []string{"FOO"})
	require.Nil(t, got)
}

func TestAutoDetectSecrets_TokenBoundary_ReverseScenario(t *testing.T) {
	// $FOO followed by space is a valid match.
	got := autoDetectSecrets("echo $FOO bar", []string{"FOO"})
	require.Equal(t, []string{"FOO"}, got)
}

func TestAutoDetectSecrets_EmptySecretSkipped(t *testing.T) {
	got := autoDetectSecrets("echo $FOO", []string{"", "FOO"})
	require.Equal(t, []string{"FOO"}, got)
}

func TestSecretReferencedIn_BraceForm(t *testing.T) {
	require.True(t, secretReferencedIn("echo ${FOO}", "FOO"))
	require.True(t, secretReferencedIn(`echo "${FOO}_bar"`, "FOO"))
	require.False(t, secretReferencedIn("echo ${BAR}", "FOO"))
}

func TestSecretReferencedIn_DollarForm(t *testing.T) {
	require.True(t, secretReferencedIn("echo $FOO", "FOO"))
	require.True(t, secretReferencedIn("echo $FOO bar", "FOO"))
	require.True(t, secretReferencedIn("echo $FOO\n", "FOO"))
	require.False(t, secretReferencedIn("echo $FOOBAR", "FOO"))
}

// resolveSecrets (mode dispatch) ---------------------------------------

func TestResolveSecrets_WildcardMountsAll(t *testing.T) {
	got := resolveSecrets([]string{"*"}, []string{"A", "B"}, "echo hi")
	require.Equal(t, []string{"A", "B"}, got)
}

func TestResolveSecrets_ExplicitListIgnoresUsage(t *testing.T) {
	got := resolveSecrets([]string{"X"}, []string{"A", "B"}, "echo hi")
	require.Equal(t, []string{"X"}, got, "explicit list mounts those secrets even if not referenced")
}

func TestResolveSecrets_EmptyAutoDetects(t *testing.T) {
	got := resolveSecrets(nil, []string{"A", "B"}, "echo $A")
	require.Equal(t, []string{"A"}, got)
}

func TestResolveSecrets_NoPlanSecretsNoMounts(t *testing.T) {
	require.Nil(t, resolveSecrets(nil, nil, "echo hi"))
	require.Nil(t, resolveSecrets([]string{"*"}, nil, "echo hi"))
}

// resolveDeployUser ----------------------------------------------------

func TestResolveDeployUser_DistrolessNonroot(t *testing.T) {
	user, setup := resolveDeployUser(plan.NewImageLayer("gcr.io/distroless/static-debian12:nonroot"))
	require.Empty(t, user, "distroless :nonroot already runs as nonroot")
	require.Empty(t, setup)
}

func TestResolveDeployUser_DistrolessCcNonroot(t *testing.T) {
	user, setup := resolveDeployUser(plan.NewImageLayer("gcr.io/distroless/cc-debian12:nonroot"))
	require.Empty(t, user)
	require.Empty(t, setup)
}

func TestResolveDeployUser_DotnetAspnetUsesAppUser(t *testing.T) {
	user, setup := resolveDeployUser(plan.NewImageLayer("mcr.microsoft.com/dotnet/aspnet:8.0"))
	require.Equal(t, "app", user, ".NET images ship a built-in app user (UID 1654)")
	require.Empty(t, setup, "no useradd needed for MS .NET images")
}

func TestResolveDeployUser_DotnetRuntimeAlpineUsesAppUser(t *testing.T) {
	user, setup := resolveDeployUser(plan.NewImageLayer("mcr.microsoft.com/dotnet/runtime:8.0-alpine"))
	require.Equal(t, "app", user, "MS image takes precedence over alpine generic rule")
	require.Empty(t, setup)
}

func TestResolveDeployUser_AlpineUsesAdduser(t *testing.T) {
	user, setup := resolveDeployUser(plan.NewImageLayer("node:20-alpine"))
	require.Equal(t, "appuser", user)
	require.Contains(t, setup, "adduser -D")
}

func TestResolveDeployUser_DebianUsesUseradd(t *testing.T) {
	user, setup := resolveDeployUser(plan.NewImageLayer("ruby:3.3-bookworm-slim"))
	require.Equal(t, "appuser", user)
	require.Contains(t, setup, "useradd -r -u 1000")
}

func TestResolveDeployUser_StepBasedNoUser(t *testing.T) {
	// Deploy bases that point to a previous step (e.g., a packages-apt-runtime
	// stage) leave the user empty — the renderer doesn't try to inject a
	// USER directive on top of an inherited stage.
	user, setup := resolveDeployUser(plan.NewStepLayer("packages-apt-runtime"))
	require.Empty(t, user)
	require.Empty(t, setup)
}

// CommandKind dispatch in renderer -------------------------------------

func TestNewExecCommand_KindIsExec(t *testing.T) {
	cmd := plan.NewExecCommand("go mod download")
	exec, ok := cmd.(plan.ExecCommand)
	require.True(t, ok)
	require.Equal(t, plan.CommandKindExec, exec.Kind)
}

func TestNewExecShellCommand_KindIsShell(t *testing.T) {
	cmd := plan.NewExecShellCommand("apt-get update && apt-get install foo")
	exec, ok := cmd.(plan.ExecCommand)
	require.True(t, ok)
	require.Equal(t, plan.CommandKindShell, exec.Kind)
}

// AddCacheMount idempotency --------------------------------------------

func TestBuildKitCacheMount_DefaultsToLocked(t *testing.T) {
	m := plan.NewBuildKitCacheMount("/root/.cache/go-build")
	require.Equal(t, "/root/.cache/go-build", m.Target)
	require.Equal(t, plan.CacheTypeLocked, m.Sharing)
}
