// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package ruby

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
)

func createTempApp(t *testing.T, files map[string]string) *app.App {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}
	a, err := app.NewApp(dir)
	require.NoError(t, err)
	return a
}

func createTestContext(t *testing.T, a *app.App, envVars map[string]string) *generate.GenerateContext {
	t.Helper()
	var envPtr *map[string]string
	if envVars != nil {
		envPtr = &envVars
	}
	env := app.NewEnvironment(envPtr)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)
	return ctx
}

const sinatraGemfile = `source "https://rubygems.org"
ruby "3.3"

gem "sinatra", "~> 4.0"
gem "puma", "~> 6.0"
`

const railsGemfile = `source "https://rubygems.org"
ruby "3.3"

gem "rails", "~> 7.1"
gem "puma"
`

func TestRubyProvider_Name(t *testing.T) {
	require.Equal(t, "ruby", (&RubyProvider{}).Name())
}

func TestRubyProvider_Detect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"gemfile", map[string]string{"Gemfile": sinatraGemfile}, true},
		{"none", map[string]string{"package.json": "{}"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)
			got, err := (&RubyProvider{}).Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestRubyProvider_StartCommandHelp(t *testing.T) {
	help := (&RubyProvider{}).StartCommandHelp()
	require.Contains(t, help, "Gemfile")
	require.Contains(t, help, "THEOPACKS_APP_NAME")
}

func TestGemNames(t *testing.T) {
	got := gemNames(sinatraGemfile)
	require.ElementsMatch(t, []string{"sinatra", "puma"}, got)
}

func TestGemNames_DropsDuplicates(t *testing.T) {
	got := gemNames(`gem "puma"
gem "puma"`)
	require.Equal(t, []string{"puma"}, got)
}

func TestHasGem(t *testing.T) {
	require.True(t, hasGem(sinatraGemfile, "sinatra"))
	require.False(t, hasGem(sinatraGemfile, "rails"))
}

func TestRubyVersionFromGemfile(t *testing.T) {
	require.Equal(t, "3.3", rubyVersionFromGemfile(sinatraGemfile))
	require.Equal(t, "", rubyVersionFromGemfile(`gem "puma"`))
	require.Equal(t, "3.2.5", rubyVersionFromGemfile(`ruby "3.2.5"`))
}

func TestDetectRubyVersion_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{"Gemfile": `source "https://rubygems.org"`})
	ctx := createTestContext(t, a, nil)
	v, src := detectRubyVersion(ctx)
	require.Equal(t, "3.3", v)
	require.Equal(t, "default", src)
}

func TestDetectRubyVersion_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{"Gemfile": `source "https://rubygems.org"`})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_RUBY_VERSION": "3.2"})
	v, src := detectRubyVersion(ctx)
	require.Equal(t, "3.2", v)
	require.Equal(t, "THEOPACKS_RUBY_VERSION", src)
}

func TestDetectRubyVersion_DotRubyVersion(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":       `source "https://rubygems.org"`,
		".ruby-version": "3.2.5\n",
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectRubyVersion(ctx)
	require.Equal(t, "3.2", v)
	require.Equal(t, ".ruby-version", src)
}

func TestDetectRubyVersion_Gemfile(t *testing.T) {
	a := createTempApp(t, map[string]string{"Gemfile": sinatraGemfile})
	ctx := createTestContext(t, a, nil)
	v, src := detectRubyVersion(ctx)
	require.Equal(t, "3.3", v)
	require.Equal(t, "Gemfile", src)
}

func TestDetectFramework_Rails(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":               railsGemfile,
		"config/application.rb": `Rails.application.initialize!`,
	})
	require.Equal(t, FrameworkRails, detectFramework(a))
}

func TestDetectFramework_Sinatra(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":   sinatraGemfile,
		"config.ru": `require 'sinatra'`,
	})
	require.Equal(t, FrameworkSinatra, detectFramework(a))
}

func TestDetectFramework_Rack(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":   `gem "rack"`,
		"config.ru": `run lambda{}`,
	})
	require.Equal(t, FrameworkRack, detectFramework(a))
}

func TestDetectFramework_Unknown(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile": `gem "unrelated"`,
	})
	require.Equal(t, FrameworkUnknown, detectFramework(a))
}

func TestDetectFramework_NoGemfile(t *testing.T) {
	a := createTempApp(t, map[string]string{})
	require.Equal(t, FrameworkUnknown, detectFramework(a))
}

func TestProcfileWebCommand(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Procfile": "web: bundle exec puma -C config/puma.rb\nworker: bundle exec sidekiq",
	})
	require.Equal(t, "bundle exec puma -C config/puma.rb", procfileWebCommand(a))
}

func TestProcfileWebCommand_NoProcfile(t *testing.T) {
	a := createTempApp(t, map[string]string{})
	require.Equal(t, "", procfileWebCommand(a))
}

func TestFrameworkStartCommand_Rails(t *testing.T) {
	a := createTempApp(t, map[string]string{"Gemfile": railsGemfile, "config/application.rb": ""})
	cmd := frameworkStartCommand(a, FrameworkRails)
	require.Contains(t, cmd, "rails server")
}

func TestFrameworkStartCommand_Sinatra(t *testing.T) {
	a := createTempApp(t, map[string]string{"Gemfile": sinatraGemfile, "config.ru": ""})
	cmd := frameworkStartCommand(a, FrameworkSinatra)
	require.Contains(t, cmd, "rackup")
}

func TestFrameworkStartCommand_ProcfileWins(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":  railsGemfile,
		"Procfile": "web: bundle exec puma -t 5",
	})
	require.Equal(t, "bundle exec puma -t 5", frameworkStartCommand(a, FrameworkRails))
}

func TestPlanSimple_Sinatra(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":   sinatraGemfile,
		"config.ru": `run Sinatra::Application`,
		"app.rb":    `# minimal`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&RubyProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "rackup")
}

func TestPlanSimple_Rails(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":               railsGemfile,
		"config/application.rb": `module App; class Application < Rails::Application; end; end`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&RubyProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "rails server")
}

func TestPlanSimple_ProcfileOverride(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":  sinatraGemfile,
		"Procfile": "web: bundle exec puma -p ${PORT}",
	})
	ctx := createTestContext(t, a, nil)
	err := (&RubyProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "bundle exec puma -p ${PORT}", ctx.Deploy.StartCmd)
}

func TestDetectWorkspace_AppsTree(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":            `gem "rails"`,
		"apps/api/app.rb":    `# api`,
		"apps/worker/app.rb": `# worker`,
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Contains(t, ws.AppPaths, "api")
	require.Contains(t, ws.AppPaths, "worker")
}

func TestDetectWorkspace_NoGemfile(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"apps/api/app.rb": "",
	})
	require.Nil(t, DetectWorkspace(a))
}

func TestDetectWorkspace_NoAppsDir(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile": `gem "rack"`,
	})
	require.Nil(t, DetectWorkspace(a))
}

func TestSelectApp_Found(t *testing.T) {
	ws := &WorkspaceInfo{AppPaths: map[string]string{"api": "apps/api", "worker": "apps/worker"}}
	name, path, ok := ws.SelectApp("worker")
	require.True(t, ok)
	require.Equal(t, "worker", name)
	require.Equal(t, "apps/worker", path)
}

func TestSelectApp_SingleAuto(t *testing.T) {
	ws := &WorkspaceInfo{AppPaths: map[string]string{"only": "apps/only"}}
	name, _, ok := ws.SelectApp("")
	require.True(t, ok)
	require.Equal(t, "only", name)
}

func TestSelectApp_AmbiguousNoEnv(t *testing.T) {
	ws := &WorkspaceInfo{AppPaths: map[string]string{"a": "apps/a", "b": "apps/b"}}
	_, _, ok := ws.SelectApp("")
	require.False(t, ok)
}

func TestPlanWorkspace_SelectsApp(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":            `gem "puma"`,
		"apps/api/app.rb":    `# api`,
		"apps/api/config.ru": `run lambda {}`,
		"apps/worker/app.rb": `# worker`,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "api"})
	err := (&RubyProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "apps/api")
	require.Contains(t, ctx.Deploy.StartCmd, "rackup")
}

func TestPlanWorkspace_Worker(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":            `gem "puma"`,
		"apps/api/app.rb":    `# api`,
		"apps/worker/app.rb": `# worker`,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "worker"})
	err := (&RubyProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "apps/worker")
	require.Contains(t, ctx.Deploy.StartCmd, "ruby app.rb")
}

func TestPlanWorkspace_BadAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Gemfile":         `gem "puma"`,
		"apps/api/app.rb": `# api`,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "ghost"})
	err := (&RubyProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no app named")
}

func TestAppNames_Sorted(t *testing.T) {
	ws := &WorkspaceInfo{AppPaths: map[string]string{"z": "apps/z", "a": "apps/a", "m": "apps/m"}}
	require.Equal(t, []string{"a", "m", "z"}, ws.AppNames())
}
