// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package dotnet implements language detection and Dockerfile build planning
// for .NET projects. Single-project (.csproj/.fsproj/.vbproj) and solution
// (.sln) layouts are both supported; ASP.NET Core projects route to the
// `aspnet:<v>` runtime image while plain console / worker projects use
// `runtime:<v>` for a smaller deployable.
package dotnet

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type DotnetProvider struct{}

func (p *DotnetProvider) Name() string {
	return "dotnet"
}

func (p *DotnetProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	if findSolutionFile(ctx.App) != "" {
		return true, nil
	}
	for _, ext := range []string{"*.csproj", "*.fsproj", "*.vbproj"} {
		if ctx.App.HasMatch(ext) {
			return true, nil
		}
		// Detect projects in nested src/<project> layouts too.
		matches, err := ctx.App.FindFiles("**/" + ext)
		if err == nil && len(matches) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (p *DotnetProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *DotnetProvider) Plan(ctx *generate.GenerateContext) error {
	if sln := findSolutionFile(ctx.App); sln != "" {
		return p.planSolution(ctx, sln)
	}
	return p.planSingleProject(ctx)
}

func (p *DotnetProvider) planSingleProject(ctx *generate.GenerateContext) error {
	projects := findProjectFiles(ctx.App)
	if len(projects) == 0 {
		return fmt.Errorf("no .NET project file (*.csproj, *.fsproj, *.vbproj) found")
	}

	target, err := pickProject(ctx, projects)
	if err != nil {
		return err
	}

	proj, err := parseProject(ctx.App, target)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", target, err)
	}

	return p.emitPlan(ctx, proj, target)
}

func (p *DotnetProvider) planSolution(ctx *generate.GenerateContext, slnPath string) error {
	entries, err := parseSolution(ctx.App, slnPath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", slnPath, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("solution file %s lists no projects", slnPath)
	}

	target, err := pickSolutionEntry(ctx, entries)
	if err != nil {
		return err
	}

	proj, err := parseProject(ctx.App, target.Path)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", target.Path, err)
	}

	return p.emitPlan(ctx, proj, target.Path)
}

// pickProject resolves the build target when the project root has just one
// or more loose project files (no .sln). Rules:
//   - Exactly one project → use it.
//   - Multiple projects → require THEOPACKS_APP_NAME matching a project file
//     stem (e.g., "MyApi" picks "src/MyApi/MyApi.csproj").
func pickProject(ctx *generate.GenerateContext, projects []string) (string, error) {
	if len(projects) == 1 {
		return projects[0], nil
	}

	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	if appName == "" {
		names := projectNames(projects)
		return "", fmt.Errorf("multiple .NET projects found; set THEOPACKS_APP_NAME to one of: %s", strings.Join(names, ", "))
	}

	for _, p := range projects {
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if stem == appName {
			return p, nil
		}
	}
	return "", fmt.Errorf("THEOPACKS_APP_NAME=%q does not match any .NET project; available: %s", appName, strings.Join(projectNames(projects), ", "))
}

// pickSolutionEntry resolves the build target from a .sln. Rules:
//   - THEOPACKS_APP_NAME matches an entry name → use it.
//   - Exactly one ASP.NET project in the solution → auto-select it.
//   - Otherwise: error listing entries.
func pickSolutionEntry(ctx *generate.GenerateContext, entries []SolutionEntry) (*SolutionEntry, error) {
	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")

	if appName != "" {
		for i := range entries {
			if entries[i].Name == appName {
				return &entries[i], nil
			}
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name)
		}
		return nil, fmt.Errorf("THEOPACKS_APP_NAME=%q does not match any solution project; available: %s", appName, strings.Join(names, ", "))
	}

	// Auto-select: prefer the single ASP.NET project if there's exactly one.
	var aspNetProjects []*SolutionEntry
	for i := range entries {
		proj, err := parseProject(ctx.App, entries[i].Path)
		if err != nil {
			continue
		}
		if proj.IsAspNet() {
			aspNetProjects = append(aspNetProjects, &entries[i])
		}
	}
	if len(aspNetProjects) == 1 {
		return aspNetProjects[0], nil
	}

	if len(entries) == 1 {
		return &entries[0], nil
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return nil, fmt.Errorf("solution has multiple projects; set THEOPACKS_APP_NAME to one of: %s", strings.Join(names, ", "))
}

func projectNames(projects []string) []string {
	out := make([]string, len(projects))
	for i, p := range projects {
		out[i] = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	return out
}

func (p *DotnetProvider) emitPlan(ctx *generate.GenerateContext, proj *Project, projPath string) error {
	if !proj.IsExecutable() {
		return fmt.Errorf("project %s does not produce a runnable binary (OutputType is Library); set <OutputType>Exe</OutputType> or use Microsoft.NET.Sdk.Web", projPath)
	}

	version, source := detectDotnetVersion(ctx, proj)
	ref := ctx.Resolver.Default("dotnet", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	assemblyName := proj.AssemblyName()
	if assemblyName == "" {
		assemblyName = strings.TrimSuffix(filepath.Base(projPath), filepath.Ext(projPath))
	}

	// Install step: copy project file(s) only and restore. This is the
	// expensive step; isolating it lets Docker cache it across source-only
	// changes.
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.DotnetSdkImageForVersion(version)))
	installStep.AddCacheMount("/root/.nuget/packages", "")
	installStep.AddCommand(plan.NewCopyCommand(projPath, projPath))
	if ctx.App.HasFile("global.json") {
		installStep.AddCommand(plan.NewCopyCommand("global.json", "./"))
	}
	if sln := findSolutionFile(ctx.App); sln != "" {
		installStep.AddCommand(plan.NewCopyCommand(sln, "./"))
	}
	installStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("dotnet restore %s", shellSafe(projPath)),
	))

	// Build step: copy the rest and publish.
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/root/.nuget/packages", "")
	// -p:DebugType=None -p:DebugSymbols=false strips PDB files (~25% smaller publish).
	// -p:PublishTrimmed and AOT are NOT enabled by default because they break
	// reflection-heavy code (EF Core, AutoMapper). Users can opt in via
	// theopacks.json buildArgs once we support that.
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("dotnet publish %s -c Release -o /app/publish --no-restore -p:DebugType=None -p:DebugSymbols=false", shellSafe(projPath)),
	))

	// Deploy: pick the runtime image based on whether the project is ASP.NET.
	if proj.IsAspNet() {
		ctx.Deploy.Base = plan.NewImageLayer(generate.DotnetAspnetImageForVersion(version))
		// ASP.NET Core ships UseHealthChecks() middleware; convention is /healthz.
		ctx.Deploy.HealthcheckPath = "/healthz"
	} else {
		ctx.Deploy.Base = plan.NewImageLayer(generate.DotnetRuntimeImageForVersion(version))
	}
	ctx.Deploy.StartCmd = fmt.Sprintf("dotnet /app/%s.dll", assemblyName)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/publish"}}),
	})

	return nil
}

// shellSafe rejects characters outside the .NET project-path / glob domain
// (alphanumeric, dot, slash, dash, underscore). MSBuild already constrains
// these, but defense in depth is cheap.
func shellSafe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '/' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (p *DotnetProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *DotnetProvider) StartCommandHelp() string {
	return ".NET projects must produce an executable assembly (Sdk=\"Microsoft.NET.Sdk.Web\" or <OutputType>Exe</OutputType>). For solutions or multi-project trees, set THEOPACKS_APP_NAME to the project name (file stem)."
}
