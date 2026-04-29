// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package deno implements language detection and Dockerfile build planning
// for Deno projects. Detection is anchored on deno.json or deno.jsonc.
//
// This provider is registered BEFORE the Node provider so projects shipping
// both deno.json and a npm-compat package.json route correctly to Deno
// (ADR D3). Fresh and Hono are auto-detected for the start command; Deno 2
// `workspace` arrays are honored for monorepo target selection.
//
// Build image: denoland/deno:bin-<v>. Runtime image: denoland/deno:distroless-<v>
// (smaller and stable for Deno 2.x).
package deno

import (
	"fmt"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type DenoProvider struct{}

func (p *DenoProvider) Name() string {
	return "deno"
}

func (p *DenoProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return hasDenoManifest(ctx.App), nil
}

func (p *DenoProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *DenoProvider) Plan(ctx *generate.GenerateContext) error {
	cfg, _, err := readDenoConfig(ctx.App)
	if err != nil {
		return fmt.Errorf("failed to parse deno config: %w", err)
	}

	version, source := detectDenoVersion(ctx)
	ref := ctx.Resolver.Default("deno", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	if cfg != nil && len(cfg.Workspace) > 0 {
		return p.planWorkspace(ctx, cfg, version)
	}
	return p.planSimple(ctx, cfg, version)
}

func (p *DenoProvider) planSimple(ctx *generate.GenerateContext, cfg *DenoConfig, version string) error {
	fw := detectFramework(cfg)
	startCmd := frameworkStartCommand(ctx.App, cfg, fw)
	if startCmd == "" {
		startCmd = ctx.Env.GetVariable("THEOPACKS_START_CMD")
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.DenoImageForVersion(version)))
	for _, name := range []string{"deno.json", "deno.jsonc", "deno.lock", "import_map.json"} {
		if ctx.App.HasFile(name) {
			installStep.AddCommand(plan.NewCopyCommand(name, "./"))
		}
	}
	if entry, ok := hasMainEntry(ctx.App); ok {
		installStep.AddCommand(plan.NewCopyCommand(entry, "./"))
		installStep.AddCommand(plan.NewExecShellCommand(fmt.Sprintf("deno cache %s", entry)))
	}

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	if cfg != nil {
		if _, ok := cfg.Tasks["build"]; ok {
			buildStep.AddCommand(plan.NewExecShellCommand("deno task build"))
		}
	}

	configureDenoDeploy(ctx, version, startCmd)
	return nil
}

func (p *DenoProvider) planWorkspace(ctx *generate.GenerateContext, cfg *DenoConfig, version string) error {
	ws := DetectWorkspace(ctx.App, ctx.Logger)
	if ws == nil || len(ws.Members) == 0 {
		return fmt.Errorf("Deno workspace declares members but none could be resolved")
	}

	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	name, path, ok := ws.SelectMember(appName)
	if !ok {
		if appName == "" {
			return fmt.Errorf("Deno workspace has multiple members; set THEOPACKS_APP_NAME to one of: %s", strings.Join(ws.MemberNames(), ", "))
		}
		return fmt.Errorf("Deno workspace has no member named %q; available: %s", appName, strings.Join(ws.MemberNames(), ", "))
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.DenoImageForVersion(version)))
	for _, root := range []string{"deno.json", "deno.jsonc", "deno.lock"} {
		if ctx.App.HasFile(root) {
			installStep.AddCommand(plan.NewCopyCommand(root, "./"))
		}
	}
	for _, member := range []string{path + "/deno.json", path + "/deno.jsonc"} {
		if ctx.App.HasFile(member) {
			installStep.AddCommand(plan.NewCopyCommand(member, member))
		}
	}

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	startCmd := procfileWebCommand(ctx.App)
	if startCmd == "" {
		startCmd = fmt.Sprintf("deno task --config %s/deno.json start", path)
		// If the member has no `tasks.start`, fall back to running its main entry directly.
		// We try main.ts → main.js → mod.ts.
		for _, candidate := range []string{path + "/main.ts", path + "/main.js", path + "/mod.ts"} {
			if ctx.App.HasFile(candidate) {
				startCmd = fmt.Sprintf("deno run -A %s", candidate)
				break
			}
		}
	}
	_ = name
	_ = cfg

	configureDenoDeploy(ctx, version, startCmd)
	return nil
}

func configureDenoDeploy(ctx *generate.GenerateContext, version, startCmd string) {
	// Deno's distroless runtime is small but doesn't ship a shell; the deno
	// CLI is the entrypoint. theo-packs generates a CMD via /bin/bash today,
	// so we use the bin- variant for runtime to keep that contract working.
	// (Distroless can be a future opt-in via theopacks.json.)
	ctx.Deploy.Base = plan.NewImageLayer(generate.DenoImageForVersion(version))
	ctx.Deploy.StartCmd = startCmd
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})
}

func (p *DenoProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *DenoProvider) StartCommandHelp() string {
	return "Deno apps need deno.json or deno.jsonc. Add a `tasks.start` field, or have a main.ts/main.js entry, or set THEOPACKS_START_CMD. For Deno 2 workspaces, set THEOPACKS_APP_NAME to a member's leaf name."
}
