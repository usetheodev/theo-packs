package generate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
)

func TestCommandStepBuilder(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("install")
	require.Equal(t, "install", step.Name())

	step.AddInput(plan.NewImageLayer(DefaultRuntimeImage))
	step.AddCommand(plan.NewExecShellCommand("npm install"))
	step.AddEnvVars(map[string]string{"NODE_ENV": "production"})
	step.AddCache("npm")

	require.Len(t, step.Inputs, 1)
	require.Len(t, step.Commands, 1)
	require.Equal(t, "production", step.Variables["NODE_ENV"])
	require.Contains(t, step.Caches, "npm")
}

func TestCommandStepBuilderReplacesExisting(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	ctx.NewCommandStep("build")
	require.Len(t, ctx.Steps, 1)

	ctx.NewCommandStep("build")
	require.Len(t, ctx.Steps, 1, "should replace existing step with same name")
}

func TestCommandStepBuilderBuild(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("build")
	step.AddInput(plan.NewImageLayer(DefaultRuntimeImage))
	step.AddCommand(plan.NewExecShellCommand("make"))

	buildPlan := plan.NewBuildPlan()
	err = step.Build(buildPlan, &BuildStepOptions{
		Caches: NewCacheContext(),
	})
	require.NoError(t, err)

	require.Len(t, buildPlan.Steps, 1)
	require.Equal(t, "build", buildPlan.Steps[0].Name)
}

func TestCommandStepBuilderAddPaths(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("install")
	step.AddPaths([]string{"/usr/local/bin", "/app/bin"})

	require.Len(t, step.Commands, 2)
}

func TestCommandStepBuilderUseSecrets(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	envVars := map[string]string{
		"CI":               "true",
		"NPM_TOKEN":        "secret",
		"GITHUB_TOKEN":     "secret",
		"NPM_AUTH_TOKEN":   "secret",
		"OTHER_VAR":        "value",
	}
	env := app.NewEnvironment(&envVars)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("install")
	step.UseSecretsWithPrefix("NPM_")

	require.Contains(t, step.Secrets, "NPM_TOKEN")
	require.Contains(t, step.Secrets, "NPM_AUTH_TOKEN")
}

func TestCommandStepBuilderAddInputs(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("build")
	step.AddInputs([]plan.Layer{
		plan.NewImageLayer(DefaultRuntimeImage),
		plan.NewLocalLayer(),
	})
	require.Len(t, step.Inputs, 2)
}

func TestCommandStepBuilderAddVariables(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("build")
	step.AddVariables(map[string]string{"A": "1", "B": "2"})
	require.Equal(t, "1", step.Variables["A"])
	require.Equal(t, "2", step.Variables["B"])
}

func TestCommandStepBuilderUseSecretsWithPrefixes(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	envVars := map[string]string{
		"NPM_TOKEN":  "secret",
		"GH_TOKEN":   "secret",
		"OTHER":      "val",
	}
	env := app.NewEnvironment(&envVars)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("install")
	step.UseSecretsWithPrefixes([]string{"NPM_", "GH_"})
	require.Contains(t, step.Secrets, "NPM_TOKEN")
	require.Contains(t, step.Secrets, "GH_TOKEN")
}

func TestCommandStepBuilderUseSecretsCIMode(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	t.Run("with CI", func(t *testing.T) {
		envVars := map[string]string{"CI": "true"}
		env := app.NewEnvironment(&envVars)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		step := ctx.NewCommandStep("install")
		initialLen := len(step.Secrets)
		step.UseSecrets([]string{"MY_SECRET"})
		require.Len(t, step.Secrets, initialLen+1)
	})

	t.Run("without CI", func(t *testing.T) {
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		step := ctx.NewCommandStep("install")
		initialLen := len(step.Secrets)
		step.UseSecrets([]string{"MY_SECRET"})
		require.Len(t, step.Secrets, initialLen, "should not add secrets without CI")
	})
}

func TestCommandStepBuilderAddCacheEmpty(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)

	err = os.WriteFile(tempDir+"/test.txt", []byte("test"), 0644)
	require.NoError(t, err)

	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	step := ctx.NewCommandStep("build")
	initialLen := len(step.Caches)
	step.AddCache("")
	require.Equal(t, initialLen, len(step.Caches), "empty cache name should not be added")
}
