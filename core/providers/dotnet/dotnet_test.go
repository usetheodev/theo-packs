// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package dotnet

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

const aspnetCsproj = `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`

const consoleCsproj = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`

const libraryCsproj = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Library</OutputType>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`

func TestDotnetProvider_Name(t *testing.T) {
	require.Equal(t, "dotnet", (&DotnetProvider{}).Name())
}

func TestDotnetProvider_Detect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"csproj", map[string]string{"app.csproj": aspnetCsproj}, true},
		{"fsproj", map[string]string{"app.fsproj": aspnetCsproj}, true},
		{"vbproj", map[string]string{"app.vbproj": aspnetCsproj}, true},
		{"sln", map[string]string{
			"app.sln":                ``,
			"src/MyApi/MyApi.csproj": aspnetCsproj,
		}, true},
		{"nested csproj", map[string]string{"src/Api/Api.csproj": aspnetCsproj}, true},
		{"none", map[string]string{"package.json": "{}"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)
			got, err := (&DotnetProvider{}).Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestDetectDotnetVersion_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj": consoleCsproj,
	})
	ctx := createTestContext(t, a, nil)
	proj, _ := parseProject(a, "app.csproj")
	v, src := detectDotnetVersion(ctx, proj)
	require.Equal(t, "8.0", v)
	require.Equal(t, "TargetFramework", src)
}

func TestDetectDotnetVersion_GlobalJson(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj":  consoleCsproj,
		"global.json": `{"sdk":{"version":"6.0.402"}}`,
	})
	ctx := createTestContext(t, a, nil)
	proj, _ := parseProject(a, "app.csproj")
	v, src := detectDotnetVersion(ctx, proj)
	require.Equal(t, "6.0", v)
	require.Equal(t, "global.json", src)
}

func TestDetectDotnetVersion_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{"app.csproj": consoleCsproj})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_DOTNET_VERSION": "9.0"})
	proj, _ := parseProject(a, "app.csproj")
	v, src := detectDotnetVersion(ctx, proj)
	require.Equal(t, "9.0", v)
	require.Equal(t, "THEOPACKS_DOTNET_VERSION", src)
}

func TestDetectDotnetVersion_FallbackDefault(t *testing.T) {
	a := createTempApp(t, map[string]string{})
	ctx := createTestContext(t, a, nil)
	v, src := detectDotnetVersion(ctx, nil)
	require.Equal(t, "8.0", v)
	require.Equal(t, "default", src)
}

func TestParseSolution(t *testing.T) {
	sln := `Microsoft Visual Studio Solution File, Format Version 12.00
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "MyApi", "src\MyApi\MyApi.csproj", "{B2A7D5C0-1234-5678-90AB-CDEF12345678}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Worker", "src\Worker\Worker.csproj", "{C3B8E6D1-2345-6789-01BC-DEF234567890}"
EndProject
`
	a := createTempApp(t, map[string]string{"app.sln": sln})
	entries, err := parseSolution(a, "app.sln")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "MyApi", entries[0].Name)
	require.Equal(t, "src/MyApi/MyApi.csproj", entries[0].Path)
	require.Equal(t, "Worker", entries[1].Name)
}

func TestParseSolution_FiltersNonProjects(t *testing.T) {
	sln := `Project("{2150E333-8FDC-42A3-9474-1A3956D46DE8}") = "SolutionFolder", "SolutionFolder", "{...}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Real", "Real\Real.csproj", "{...}"
EndProject
`
	a := createTempApp(t, map[string]string{"app.sln": sln})
	entries, err := parseSolution(a, "app.sln")
	require.NoError(t, err)
	// The solution-folder entry has no .csproj/.fsproj/.vbproj path so it's filtered.
	require.Len(t, entries, 1)
	require.Equal(t, "Real", entries[0].Name)
}

func TestPlanSingleProject_Aspnet(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj": aspnetCsproj,
		"Program.cs": "// minimal program",
	})
	ctx := createTestContext(t, a, nil)
	err := (&DotnetProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "dotnet /app/app.dll", ctx.Deploy.StartCmd)
}

func TestPlanSingleProject_Console(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.csproj": consoleCsproj,
		"Program.cs": "// minimal",
	})
	ctx := createTestContext(t, a, nil)
	err := (&DotnetProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "dotnet /app/app.dll", ctx.Deploy.StartCmd)
}

func TestPlanSingleProject_LibraryReturnsError(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"lib.csproj": libraryCsproj,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DotnetProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Library")
}

func TestPlanSingleProject_MultipleNoEnvErrors(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"src/A/A.csproj": consoleCsproj,
		"src/B/B.csproj": consoleCsproj,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DotnetProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "THEOPACKS_APP_NAME")
}

func TestPlanSingleProject_AppNameSelects(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"src/A/A.csproj": consoleCsproj,
		"src/B/B.csproj": consoleCsproj,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "B"})
	err := (&DotnetProvider{}).Plan(ctx)
	require.NoError(t, err)
}

func TestPlanSolution_SingleAspNetAutoSelects(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.sln": `Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "MyApi", "src\MyApi\MyApi.csproj", "{...}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Worker", "src\Worker\Worker.csproj", "{...}"
EndProject
`,
		"src/MyApi/MyApi.csproj":   aspnetCsproj,
		"src/Worker/Worker.csproj": consoleCsproj,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DotnetProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "dotnet /app/MyApi.dll", ctx.Deploy.StartCmd)
}

func TestPlanSolution_AppNameOverride(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.sln": `Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Api", "Api/Api.csproj", "{...}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Worker", "Worker/Worker.csproj", "{...}"
EndProject
`,
		"Api/Api.csproj":       aspnetCsproj,
		"Worker/Worker.csproj": consoleCsproj,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "Worker"})
	err := (&DotnetProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "dotnet /app/Worker.dll", ctx.Deploy.StartCmd)
}

func TestPlanSolution_BadAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.sln": `Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "Api", "Api/Api.csproj", "{...}"
EndProject
`,
		"Api/Api.csproj": aspnetCsproj,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "Ghost"})
	err := (&DotnetProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Ghost")
}

func TestPlanSolution_AmbiguousNoAspNet(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"app.sln": `Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "A", "A/A.csproj", "{...}"
EndProject
Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "B", "B/B.csproj", "{...}"
EndProject
`,
		"A/A.csproj": consoleCsproj,
		"B/B.csproj": consoleCsproj,
	})
	ctx := createTestContext(t, a, nil)
	err := (&DotnetProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "THEOPACKS_APP_NAME")
}

func TestShellSafe(t *testing.T) {
	require.Equal(t, "src/App.csproj", shellSafe("src/App.csproj"))
	require.Equal(t, "AB", shellSafe("A;B"))
	// Shell metacharacters dropped, surviving alphanumerics retained — not
	// a safety hazard because the result still has to be a valid path that
	// `dotnet restore` accepts; arbitrary substrings won't.
	require.Equal(t, "rm", shellSafe("$(rm)"))
	require.Equal(t, "abc-123_def.csproj", shellSafe("abc-123_def.csproj"))
}
