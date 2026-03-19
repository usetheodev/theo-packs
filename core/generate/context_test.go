package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/logger"
)

func TestNewGenerateContext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "context-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)
	require.NotNil(t, ctx)
	require.NotNil(t, ctx.Deploy)
	require.NotNil(t, ctx.Caches)
	require.NotNil(t, ctx.Metadata)
	require.NotNil(t, ctx.Resolver)
}

func TestGenerateContextWithDockerignore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "context-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	dockerignoreContent := `node_modules
*.log
`
	err = os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte(dockerignoreContent), 0644)
	require.NoError(t, err)

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)
	require.Equal(t, "true", ctx.Metadata.Get("dockerIgnore"))
}

func TestGenerateContextSubContext(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "context-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	require.Equal(t, "build", ctx.GetStepName("build"))

	ctx.EnterSubContext("node")
	require.Equal(t, "build:node", ctx.GetStepName("build"))

	ctx.ExitSubContext()
	require.Equal(t, "build", ctx.GetStepName("build"))
}

func TestGenerateContextWithoutDockerignore(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)
	require.Empty(t, ctx.Metadata.Get("dockerIgnore"))
}

func TestGenerateContextNewLocalLayer(t *testing.T) {
	t.Run("without dockerignore", func(t *testing.T) {
		tempDir := t.TempDir()

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		layer := ctx.NewLocalLayer()
		require.True(t, layer.Local)
		require.Empty(t, layer.Exclude)
	})

	t.Run("with dockerignore", func(t *testing.T) {
		tempDir := t.TempDir()

		err := os.WriteFile(filepath.Join(tempDir, ".dockerignore"), []byte("node_modules\n.git\n"), 0644)
		require.NoError(t, err)

		testApp, err := app.NewApp(tempDir)
		require.NoError(t, err)
		env := app.NewEnvironment(nil)
		cfg := config.EmptyConfig()
		log := logger.NewLogger()

		ctx, err := NewGenerateContext(testApp, env, cfg, log)
		require.NoError(t, err)

		layer := ctx.NewLocalLayer()
		require.True(t, layer.Local)
		require.Contains(t, layer.Exclude, "node_modules")
		require.Contains(t, layer.Exclude, ".git")
	})
}

func TestGenerateContextGetStepByName(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	require.Nil(t, ctx.GetStepByName("build"))

	ctx.NewCommandStep("build")
	require.NotNil(t, ctx.GetStepByName("build"))
	require.Nil(t, ctx.GetStepByName("install"))
}

func TestExitSubContextOnEmpty(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		ctx.ExitSubContext()
	})
	require.Empty(t, ctx.SubContexts)
}

func TestGenerateContextGetAppSource(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)
	require.Equal(t, tempDir, ctx.GetAppSource())
}

func TestGenerateContextGetLogger(t *testing.T) {
	tempDir := t.TempDir()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)
	require.Equal(t, log, ctx.GetLogger())
}

func TestNewAptInstallCommand(t *testing.T) {
	options := &BuildStepOptions{
		Caches: NewCacheContext(),
	}

	cmd := options.NewAptInstallCommand([]string{"curl", "wget", "curl"})
	require.NotNil(t, cmd)
}

func TestGenerateContextGenerate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "context-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	testApp, err := app.NewApp(tempDir)
	require.NoError(t, err)
	env := app.NewEnvironment(nil)
	cfg := config.EmptyConfig()
	cfg.Deploy.StartCmd = "node server.js"
	log := logger.NewLogger()

	ctx, err := NewGenerateContext(testApp, env, cfg, log)
	require.NoError(t, err)

	buildPlan, resolvedPackages, err := ctx.Generate()
	require.NoError(t, err)
	require.NotNil(t, buildPlan)
	require.NotNil(t, resolvedPackages)
	require.Equal(t, "node server.js", buildPlan.Deploy.StartCmd)
}
