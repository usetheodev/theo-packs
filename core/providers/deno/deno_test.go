// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package deno

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

const freshDenoJson = `{
  "imports": {
    "$fresh/": "https://deno.land/x/fresh@1.6.5/"
  },
  "tasks": {
    "start": "deno run -A --watch main.ts",
    "build": "deno task manifest"
  }
}`

const honoDenoJson = `{
  "imports": {
    "hono": "jsr:@hono/hono@4"
  }
}`

const genericDenoJson = `{
  "imports": {},
  "tasks": {}
}`

func TestDenoProvider_Name(t *testing.T) {
	require.Equal(t, "deno", (&DenoProvider{}).Name())
}

func TestDenoProvider_Detect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"deno.json", map[string]string{"deno.json": "{}"}, true},
		{"deno.jsonc", map[string]string{"deno.jsonc": "{ /* hi */ }"}, true},
		{"node only", map[string]string{"package.json": "{}"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)
			got, err := (&DenoProvider{}).Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestDenoProvider_StartCommandHelp(t *testing.T) {
	help := (&DenoProvider{}).StartCommandHelp()
	require.Contains(t, help, "deno.json")
	require.Contains(t, help, "THEOPACKS_APP_NAME")
}

func TestReadDenoConfig_Json(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": freshDenoJson})
	cfg, name, err := readDenoConfig(a)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "deno.json", name)
	require.Contains(t, cfg.Imports, "$fresh/")
	require.Contains(t, cfg.Tasks, "start")
}

func TestReadDenoConfig_Jsonc(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.jsonc": `{
  // a comment
  "imports": {"hono": "jsr:@hono/hono"}
}`,
	})
	cfg, name, err := readDenoConfig(a)
	require.NoError(t, err)
	require.Equal(t, "deno.jsonc", name)
	require.Contains(t, cfg.Imports, "hono")
}

func TestReadDenoConfig_Missing(t *testing.T) {
	a := createTempApp(t, map[string]string{})
	cfg, name, err := readDenoConfig(a)
	require.NoError(t, err)
	require.Nil(t, cfg)
	require.Equal(t, "", name)
}

func TestImportsContain_CaseInsensitive(t *testing.T) {
	c := &DenoConfig{Imports: map[string]string{"Hono": "jsr:@hono/hono"}}
	require.True(t, c.importsContain("hono"))
	require.True(t, c.importsContain("HONO"))
	require.False(t, c.importsContain("react"))
}

func TestDetectFramework_Fresh(t *testing.T) {
	c := &DenoConfig{Imports: map[string]string{"$fresh/": "https://deno.land/x/fresh"}}
	require.Equal(t, FrameworkFresh, detectFramework(c))
}

func TestDetectFramework_Hono(t *testing.T) {
	c := &DenoConfig{Imports: map[string]string{"hono": "jsr:@hono/hono"}}
	require.Equal(t, FrameworkHono, detectFramework(c))
}

func TestDetectFramework_Generic(t *testing.T) {
	c := &DenoConfig{Imports: map[string]string{"std/": "https://deno.land/std/"}}
	require.Equal(t, FrameworkGeneric, detectFramework(c))
}

func TestDetectFramework_Unknown(t *testing.T) {
	c := &DenoConfig{}
	require.Equal(t, FrameworkUnknown, detectFramework(c))
}

func TestFrameworkStartCommand_TaskStart(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": freshDenoJson, "main.ts": ""})
	cfg, _, _ := readDenoConfig(a)
	require.Equal(t, "deno task start", frameworkStartCommand(a, cfg, FrameworkFresh))
}

func TestFrameworkStartCommand_HonoFallback(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": honoDenoJson, "main.ts": "console.log('ok')"})
	cfg, _, _ := readDenoConfig(a)
	cmd := frameworkStartCommand(a, cfg, FrameworkHono)
	require.Contains(t, cmd, "deno run")
	require.Contains(t, cmd, "main.ts")
}

func TestFrameworkStartCommand_GenericMainTs(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": genericDenoJson, "main.ts": ""})
	cfg, _, _ := readDenoConfig(a)
	cmd := frameworkStartCommand(a, cfg, FrameworkGeneric)
	require.Contains(t, cmd, "deno run -A main.ts")
}

func TestFrameworkStartCommand_GenericNoEntry(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": genericDenoJson})
	cfg, _, _ := readDenoConfig(a)
	require.Equal(t, "", frameworkStartCommand(a, cfg, FrameworkGeneric))
}

func TestFrameworkStartCommand_ProcfileWins(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": freshDenoJson,
		"Procfile":  "web: deno run -A custom.ts",
	})
	cfg, _, _ := readDenoConfig(a)
	require.Equal(t, "deno run -A custom.ts", frameworkStartCommand(a, cfg, FrameworkFresh))
}

func TestHasMainEntry(t *testing.T) {
	a := createTempApp(t, map[string]string{"main.ts": ""})
	name, ok := hasMainEntry(a)
	require.True(t, ok)
	require.Equal(t, "main.ts", name)
}

func TestHasMainEntry_None(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": "{}"})
	_, ok := hasMainEntry(a)
	require.False(t, ok)
}

func TestDetectDenoVersion_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": "{}"})
	ctx := createTestContext(t, a, nil)
	v, src := detectDenoVersion(ctx)
	require.Equal(t, "2", v)
	require.Equal(t, "default", src)
}

func TestDetectDenoVersion_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": "{}"})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_DENO_VERSION": "1"})
	v, src := detectDenoVersion(ctx)
	require.Equal(t, "1", v)
	require.Equal(t, "THEOPACKS_DENO_VERSION", src)
}

func TestPlanSimple_Fresh(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": freshDenoJson,
		"main.ts":   `console.log("fresh")`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DenoProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "deno task start", ctx.Deploy.StartCmd)
}

func TestPlanSimple_Hono(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": honoDenoJson,
		"main.ts":   `import {Hono} from "hono";`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DenoProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "deno run")
}

func TestPlanSimple_GenericNoTask(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": `{}`,
		"main.ts":   `console.log("hi")`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DenoProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "deno run -A main.ts")
}

func TestPlanSimple_BuildTaskRuns(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": freshDenoJson,
		"main.ts":   `// fresh app`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DenoProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(ctx.Steps), 2)
}

func TestDetectWorkspace_Members(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": `{"workspace": ["./apps/api", "./apps/worker"]}`,
		"apps/api/deno.json":    `{"name": "@theo/api"}`,
		"apps/worker/deno.json": `{"name": "@theo/worker"}`,
	})
	ws := DetectWorkspace(a, logger.NewLogger())
	require.NotNil(t, ws)
	require.Contains(t, ws.Members, "api")
	require.Contains(t, ws.Members, "worker")
	require.Equal(t, "apps/api", ws.Members["api"])
}

func TestDetectWorkspace_FallsBackToLeaf(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json":          `{"workspace": ["apps/api"]}`,
		"apps/api/deno.json": `{}`,
	})
	ws := DetectWorkspace(a, logger.NewLogger())
	require.NotNil(t, ws)
	require.Contains(t, ws.Members, "api")
}

func TestDetectWorkspace_NoWorkspace(t *testing.T) {
	a := createTempApp(t, map[string]string{"deno.json": `{}`})
	require.Nil(t, DetectWorkspace(a, logger.NewLogger()))
}

func TestPlanWorkspace_SelectsMember(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json": `{"workspace": ["apps/api", "apps/worker"]}`,
		"apps/api/deno.json":    `{"name": "@theo/api", "tasks": {"start": "deno run -A main.ts"}}`,
		"apps/api/main.ts":      "",
		"apps/worker/deno.json": `{"name": "@theo/worker"}`,
		"apps/worker/main.ts":   "",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "api"})
	err := (&DenoProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Contains(t, ctx.Deploy.StartCmd, "apps/api")
}

func TestPlanWorkspace_AmbiguousNoEnv(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json":             `{"workspace": ["apps/api", "apps/worker"]}`,
		"apps/api/deno.json":    `{"name": "api"}`,
		"apps/worker/deno.json": `{"name": "worker"}`,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DenoProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "THEOPACKS_APP_NAME")
}

func TestPlanWorkspace_BadAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"deno.json":          `{"workspace": ["apps/api"]}`,
		"apps/api/deno.json": `{"name": "api"}`,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "ghost"})
	err := (&DenoProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no member named")
}

func TestSelectMember_Single(t *testing.T) {
	ws := &WorkspaceInfo{Members: map[string]string{"only": "apps/only"}}
	name, _, ok := ws.SelectMember("")
	require.True(t, ok)
	require.Equal(t, "only", name)
}

func TestMemberNames_Sorted_Deno(t *testing.T) {
	ws := &WorkspaceInfo{Members: map[string]string{"z": "z", "a": "a"}}
	require.Equal(t, []string{"a", "z"}, ws.MemberNames())
}
