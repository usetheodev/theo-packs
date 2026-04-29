// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package rust implements language detection and Dockerfile build planning
// for Rust projects. Detection is anchored on Cargo.toml. Single-crate and
// Cargo workspace layouts are both supported; workspace target selection uses
// THEOPACKS_APP_NAME (matches a [package] name from one of the members).
//
// Build strategy: a multi-stage Dockerfile that uses a versioned rust:<v>-bookworm
// image to compile a release binary and a minimal debian:bookworm-slim runtime
// image. Cargo's dependency cache is warmed up in a separate "install" step
// (Docker layer caching) and the binary is staged at /app/server in deploy.
package rust

import (
	"fmt"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type RustProvider struct{}

func (p *RustProvider) Name() string {
	return "rust"
}

func (p *RustProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("Cargo.toml"), nil
}

func (p *RustProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *RustProvider) Plan(ctx *generate.GenerateContext) error {
	cargo, err := parseCargoToml(ctx.App, "Cargo.toml")
	if err != nil {
		return fmt.Errorf("failed to parse Cargo.toml: %w", err)
	}

	version, source := detectRustVersion(ctx, cargo)
	ref := ctx.Resolver.Default("rust", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	if cargo.IsWorkspace() {
		return p.planWorkspace(ctx, cargo, version)
	}
	return p.planSimple(ctx, cargo, version)
}

func (p *RustProvider) planSimple(ctx *generate.GenerateContext, cargo *CargoToml, version string) error {
	if cargo.IsLibraryOnly() {
		return fmt.Errorf("rust crate is library-only (no [[bin]] target and no src/main.rs); add a binary target or use theopacks.json to override the build")
	}

	bin := cargo.PrimaryBinary()
	if bin == nil {
		return fmt.Errorf("could not determine Rust binary target — Cargo.toml needs a [package] name or a [[bin]] entry")
	}

	// Install step: copy manifests and warm up dependency cache.
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.RustBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cargo/registry", "")
	installStep.AddCacheMount("/root/.cargo/git", "")
	installStep.AddCommand(plan.NewCopyCommand("Cargo.toml", "./"))
	if ctx.App.HasFile("Cargo.lock") {
		installStep.AddCommand(plan.NewCopyCommand("Cargo.lock", "./"))
	}
	installStep.AddCommand(plan.NewExecShellCommand("cargo fetch"))

	// Build step: copy source and compile in offline mode (deps already fetched).
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/root/.cargo/registry", "")
	buildStep.AddCacheMount("/root/.cargo/git", "")
	buildStep.AddCacheMount("/app/target", "")
	buildStep.AddCommand(plan.NewExecShellCommand("cargo build --release --offline"))
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("cp target/release/%s /app/server", shellEscape(bin.Name)),
	))

	configureDeploy(ctx)
	return nil
}

func (p *RustProvider) planWorkspace(ctx *generate.GenerateContext, cargo *CargoToml, version string) error {
	ws := DetectWorkspace(ctx.App, ctx.Logger)
	if ws == nil || len(ws.MemberPaths) == 0 {
		return fmt.Errorf("cargo workspace has no resolvable members")
	}

	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	name, _, ok := ws.SelectMember(appName)
	if !ok {
		if appName == "" {
			return fmt.Errorf("cargo workspace has multiple members; set THEOPACKS_APP_NAME to one of: %s", strings.Join(ws.MemberNames(), ", "))
		}
		return fmt.Errorf("cargo workspace has no member named %q; available members: %s", appName, strings.Join(ws.MemberNames(), ", "))
	}

	// Install step: copy the entire root manifest tree (Cargo.toml + Cargo.lock
	// at root, plus member Cargo.toml files) so cargo can resolve the workspace.
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.RustBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cargo/registry", "")
	installStep.AddCacheMount("/root/.cargo/git", "")
	installStep.AddCommand(plan.NewCopyCommand("Cargo.toml", "./"))
	if ctx.App.HasFile("Cargo.lock") {
		installStep.AddCommand(plan.NewCopyCommand("Cargo.lock", "./"))
	}
	for _, memberPath := range ws.MemberPaths {
		installStep.AddCommand(plan.NewCopyCommand(memberPath+"/Cargo.toml", memberPath+"/"))
	}
	installStep.AddCommand(plan.NewExecShellCommand("cargo fetch"))

	// Build step.
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/root/.cargo/registry", "")
	buildStep.AddCacheMount("/root/.cargo/git", "")
	buildStep.AddCacheMount("/app/target", "")
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("cargo build --release --offline -p %s", shellEscape(name)),
	))
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("cp target/release/%s /app/server", shellEscape(name)),
	))

	configureDeploy(ctx)
	return nil
}

// configureDeploy sets the runtime image, the start command, and the binary
// copy filter. Both single-crate and workspace flows produce a binary at
// /app/server, so the deploy block is shared.
//
// The runtime image (RustRuntimeImage = distroless/cc-debian12:nonroot) ships
// with glibc + ca-certificates already, so we don't need an apt step like
// the previous debian-slim base did.
func configureDeploy(ctx *generate.GenerateContext) {
	ctx.Deploy.Base = plan.NewImageLayer(generate.RustRuntimeImage)
	ctx.Deploy.StartCmd = "/app/server"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
	})
}

// shellEscape rejects characters that are not valid in Cargo target names
// and would let a malicious manifest inject shell metacharacters. Cargo's
// own validation already prevents this, but defense in depth is cheap.
func shellEscape(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (p *RustProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *RustProvider) StartCommandHelp() string {
	return "Add a [[bin]] target to Cargo.toml or include src/main.rs. For workspaces, set THEOPACKS_APP_NAME to the package you want to deploy."
}
