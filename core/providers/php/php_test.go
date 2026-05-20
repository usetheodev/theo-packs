// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package php

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

const slimComposer = `{
  "name": "theo/slim",
  "require": {
    "php": ">=8.1",
    "slim/slim": "^4.0",
    "slim/psr7": "^1.0"
  }
}`

const laravelComposer = `{
  "name": "theo/laravel",
  "require": {
    "php": "^8.2",
    "laravel/framework": "^11.0"
  }
}`

const symfonyComposer = `{
  "name": "theo/symfony",
  "require": {
    "php": "^8.2",
    "symfony/framework-bundle": "^7.0"
  }
}`

const genericComposer = `{
  "name": "theo/generic",
  "require": {
    "php": "^8.3",
    "monolog/monolog": "^3.0"
  }
}`

func TestPhpProvider_Name(t *testing.T) {
	require.Equal(t, "php", (&PhpProvider{}).Name())
}

func TestPhpProvider_Detect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"composer.json", map[string]string{"composer.json": slimComposer}, true},
		{"none", map[string]string{"package.json": "{}"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)
			got, err := (&PhpProvider{}).Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestPhpProvider_StartCommandHelp(t *testing.T) {
	help := (&PhpProvider{}).StartCommandHelp()
	require.Contains(t, help, "composer.json")
	require.Contains(t, help, "THEOPACKS_APP_NAME")
}

func TestParseComposer_Slim(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": slimComposer})
	c, err := parseComposer(a)
	require.NoError(t, err)
	require.Equal(t, "theo/slim", c.Name)
	require.True(t, c.HasPackage("slim/slim"))
	require.False(t, c.HasPackage("laravel/framework"))
}

func TestComposer_HasPackage_NilSafe(t *testing.T) {
	var c *ComposerJson
	require.False(t, c.HasPackage("anything"))
}

func TestDetectFramework_Laravel(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json": laravelComposer,
		"artisan":       "#!/usr/bin/env php\n",
	})
	c, _ := parseComposer(a)
	require.Equal(t, FrameworkLaravel, detectFramework(a, c))
}

func TestDetectFramework_LaravelWithoutArtisan(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": laravelComposer})
	c, _ := parseComposer(a)
	// laravel/framework without an artisan binary is unusual; we don't claim Laravel.
	require.NotEqual(t, FrameworkLaravel, detectFramework(a, c))
}

func TestDetectFramework_Slim(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": slimComposer})
	c, _ := parseComposer(a)
	require.Equal(t, FrameworkSlim, detectFramework(a, c))
}

func TestDetectFramework_Symfony(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": symfonyComposer})
	c, _ := parseComposer(a)
	require.Equal(t, FrameworkSymfony, detectFramework(a, c))
}

func TestDetectFramework_Generic(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": genericComposer})
	c, _ := parseComposer(a)
	require.Equal(t, FrameworkGeneric, detectFramework(a, c))
}

func TestProcfileWebCommand_Php(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Procfile": "web: php public/index.php\nworker: php worker.php",
	})
	require.Equal(t, "php public/index.php", procfileWebCommand(a))
}

func TestFrameworkStartCommand_LaravelDefault(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": laravelComposer, "artisan": ""})
	require.Contains(t, frameworkStartCommand(a, FrameworkLaravel), "artisan serve")
}

func TestFrameworkStartCommand_SlimDefault(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": slimComposer, "public/index.php": "<?php"})
	require.Contains(t, frameworkStartCommand(a, FrameworkSlim), "-t public")
}

func TestFrameworkStartCommand_GenericPublic(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json":    genericComposer,
		"public/index.php": "<?php",
	})
	cmd := frameworkStartCommand(a, FrameworkGeneric)
	require.Contains(t, cmd, "-t public")
}

func TestFrameworkStartCommand_GenericRoot(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json": genericComposer,
		"index.php":     "<?php",
	})
	cmd := frameworkStartCommand(a, FrameworkGeneric)
	require.NotContains(t, cmd, "-t public")
}

func TestFrameworkStartCommand_ProcfileWins(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json": laravelComposer,
		"artisan":       "",
		"Procfile":      "web: nginx -g 'daemon off;'",
	})
	require.Equal(t, "nginx -g 'daemon off;'", frameworkStartCommand(a, FrameworkLaravel))
}

func TestDetectPhpVersion_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": `{}`})
	ctx := createTestContext(t, a, nil)
	v, src := detectPhpVersion(ctx, &ComposerJson{})
	require.Equal(t, "8.3", v)
	require.Equal(t, "default", src)
}

func TestDetectPhpVersion_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": slimComposer})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_PHP_VERSION": "8.2"})
	v, src := detectPhpVersion(ctx, &ComposerJson{})
	require.Equal(t, "8.2", v)
	require.Equal(t, "THEOPACKS_PHP_VERSION", src)
}

func TestDetectPhpVersion_DotPhpVersion(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json": slimComposer,
		".php-version":  "8.2.10\n",
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectPhpVersion(ctx, &ComposerJson{})
	require.Equal(t, "8.2", v)
	require.Equal(t, ".php-version", src)
}

func TestDetectPhpVersion_ComposerRequire(t *testing.T) {
	a := createTempApp(t, map[string]string{"composer.json": slimComposer})
	ctx := createTestContext(t, a, nil)
	c, _ := parseComposer(a)
	v, src := detectPhpVersion(ctx, c)
	require.Equal(t, "8.1", v)
	require.Equal(t, "composer.json", src)
}

func TestPlanSimple_Slim(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json":    slimComposer,
		"public/index.php": "<?php",
	})
	ctx := createTestContext(t, a, nil)
	err := (&PhpProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "-t public")
}

func TestPlanSimple_Laravel(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json": laravelComposer,
		"artisan":       "",
	})
	ctx := createTestContext(t, a, nil)
	err := (&PhpProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "artisan serve")
}

func TestDetectWorkspace_AppsTree(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json":         `{}`,
		"apps/api/index.php":    "<?php",
		"apps/worker/index.php": "<?php",
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Contains(t, ws.AppPaths, "api")
	require.Contains(t, ws.AppPaths, "worker")
}

func TestDetectWorkspace_NoComposer(t *testing.T) {
	a := createTempApp(t, map[string]string{"apps/api/index.php": ""})
	require.Nil(t, DetectWorkspace(a))
}

func TestPlanWorkspace_Selects(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json":             `{}`,
		"apps/api/public/index.php": "<?php",
		"apps/worker/worker.php":    "<?php",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "api"})
	err := (&PhpProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "apps/api/public")
}

func TestPlanWorkspace_AmbiguousNoEnv(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json":     `{}`,
		"apps/api/i.php":    "",
		"apps/worker/i.php": "",
	})
	ctx := createTestContext(t, a, nil)
	err := (&PhpProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "THEOPACKS_APP_NAME")
}

func TestPlanWorkspace_BadAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"composer.json":  `{}`,
		"apps/api/i.php": "",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "ghost"})
	err := (&PhpProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no app named")
}

func TestSelectApp_Found(t *testing.T) {
	ws := &WorkspaceInfo{AppPaths: map[string]string{"api": "apps/api"}}
	name, path, ok := ws.SelectApp("api")
	require.True(t, ok)
	require.Equal(t, "api", name)
	require.Equal(t, "apps/api", path)
}

func TestAppNames_Sorted_Php(t *testing.T) {
	ws := &WorkspaceInfo{AppPaths: map[string]string{"z": "apps/z", "a": "apps/a"}}
	require.Equal(t, []string{"a", "z"}, ws.AppNames())
}
