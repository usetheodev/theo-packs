// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package php implements language detection and Dockerfile build planning
// for PHP projects. Detection is anchored on composer.json. Laravel, Slim,
// and Symfony frameworks are auto-detected for the start command; generic
// composer projects fall through to PHP's built-in server.
//
// Build/runtime image: php:<v>-cli-bookworm. Composer is brought in via a
// multi-stage COPY from composer:2 — small and reliably current.
package php

import (
	"fmt"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type PhpProvider struct{}

func (p *PhpProvider) Name() string {
	return "php"
}

func (p *PhpProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("composer.json"), nil
}

func (p *PhpProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *PhpProvider) Plan(ctx *generate.GenerateContext) error {
	composer, err := parseComposer(ctx.App)
	if err != nil {
		return fmt.Errorf("failed to parse composer.json: %w", err)
	}

	version, source := detectPhpVersion(ctx, composer)
	ref := ctx.Resolver.Default("php", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	if ws := DetectWorkspace(ctx.App); ws != nil {
		return p.planWorkspace(ctx, ws, version)
	}

	return p.planSimple(ctx, composer, version)
}

func (p *PhpProvider) planSimple(ctx *generate.GenerateContext, composer *ComposerJson, version string) error {
	fw := detectFramework(ctx.App, composer)
	startCmd := frameworkStartCommand(ctx.App, fw)
	if startCmd == "" {
		startCmd = ctx.Env.GetVariable("THEOPACKS_START_CMD")
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PhpImageForVersion(version)))
	// Bring composer in via multi-stage. We embed the COPY into a shell
	// command that also runs composer; theo-packs' plan model doesn't have
	// first-class multi-stage tool stamping, so we use the cached binary
	// from the composer:2 image and copy it in.
	installStep.AddCommand(plan.NewExecShellCommand(
		"apt-get update && apt-get install -y --no-install-recommends git unzip ca-certificates && rm -rf /var/lib/apt/lists/* && curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer",
	))
	installStep.AddCommand(plan.NewCopyCommand("composer.json", "./"))
	if ctx.App.HasFile("composer.lock") {
		installStep.AddCommand(plan.NewCopyCommand("composer.lock", "./"))
	}
	installStep.AddCommand(plan.NewExecShellCommand(
		"composer install --no-dev --no-scripts --prefer-dist --optimize-autoloader --no-progress",
	))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	if fw == FrameworkLaravel {
		// Laravel's optimize task caches config/routes/views; run after the
		// full source tree is in place but tolerate failures because some
		// optimize subtasks fail without database access at build time.
		buildStep.AddCommand(plan.NewExecShellCommand(
			"php artisan config:cache || true; php artisan route:cache || true; php artisan view:cache || true",
		))
	}

	configurePhpDeploy(ctx, version, startCmd)
	return nil
}

func (p *PhpProvider) planWorkspace(ctx *generate.GenerateContext, ws *WorkspaceInfo, version string) error {
	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	name, path, ok := ws.SelectApp(appName)
	if !ok {
		if appName == "" {
			return fmt.Errorf("php workspace has multiple apps; set THEOPACKS_APP_NAME to one of: %s", strings.Join(ws.AppNames(), ", "))
		}
		return fmt.Errorf("php workspace has no app named %q; available: %s", appName, strings.Join(ws.AppNames(), ", "))
	}
	_ = name

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PhpImageForVersion(version)))
	installStep.AddCommand(plan.NewExecShellCommand(
		"apt-get update && apt-get install -y --no-install-recommends git unzip ca-certificates && rm -rf /var/lib/apt/lists/* && curl -fsSL https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer",
	))
	installStep.AddCommand(plan.NewCopyCommand("composer.json", "./"))
	if ctx.App.HasFile("composer.lock") {
		installStep.AddCommand(plan.NewCopyCommand("composer.lock", "./"))
	}
	installStep.AddCommand(plan.NewExecShellCommand(
		"composer install --no-dev --no-scripts --prefer-dist --optimize-autoloader --no-progress",
	))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	startCmd := procfileWebCommand(ctx.App)
	if startCmd == "" {
		if ctx.App.HasFile(path + "/public/index.php") {
			startCmd = fmt.Sprintf("php -S 0.0.0.0:${PORT:-8000} -t %s/public", path)
		} else {
			startCmd = fmt.Sprintf("php -S 0.0.0.0:${PORT:-8000} -t %s", path)
		}
	}

	configurePhpDeploy(ctx, version, startCmd)
	return nil
}

func configurePhpDeploy(ctx *generate.GenerateContext, version, startCmd string) {
	ctx.Deploy.Base = plan.NewImageLayer(generate.PhpImageForVersion(version))
	ctx.Deploy.StartCmd = startCmd
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})
}

func (p *PhpProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *PhpProvider) StartCommandHelp() string {
	return "PHP apps need a composer.json. Laravel / Slim / Symfony are auto-detected; otherwise add a `web:` line to a Procfile or set THEOPACKS_START_CMD. For monorepos, set THEOPACKS_APP_NAME to an apps/<name> directory leaf."
}
