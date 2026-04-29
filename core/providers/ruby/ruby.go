// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package ruby implements language detection and Dockerfile build planning
// for Ruby projects. Detection is anchored on Gemfile. Single-app and
// apps/+packages/ monorepo layouts are both supported. Rails, Sinatra, and
// plain Rack are auto-detected for the start command; the Procfile `web:`
// line takes precedence when present.
//
// Build/runtime image: ruby:<v>-bookworm-slim (single image — Ruby is
// interpreted, so the bundle and source ship into the same final stage).
package ruby

import (
	"fmt"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type RubyProvider struct{}

func (p *RubyProvider) Name() string {
	return "ruby"
}

func (p *RubyProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("Gemfile"), nil
}

func (p *RubyProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *RubyProvider) Plan(ctx *generate.GenerateContext) error {
	version, source := detectRubyVersion(ctx)
	ref := ctx.Resolver.Default("ruby", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	if ws := DetectWorkspace(ctx.App); ws != nil {
		return p.planWorkspace(ctx, ws, version)
	}

	return p.planSimple(ctx, version)
}

func (p *RubyProvider) planSimple(ctx *generate.GenerateContext, version string) error {
	fw := detectFramework(ctx.App)
	startCmd := frameworkStartCommand(ctx.App, fw)
	if startCmd == "" {
		startCmd = ctx.Env.GetVariable("THEOPACKS_START_CMD")
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.RubyImageForVersion(version)))
	installStep.AddCommand(plan.NewCopyCommand("Gemfile", "./"))
	if ctx.App.HasFile("Gemfile.lock") {
		installStep.AddCommand(plan.NewCopyCommand("Gemfile.lock", "./"))
	}
	// Use bundler's colon-separated multi-group form so the command body has
	// no single quotes — avoids the quote-in-quote collision once the
	// renderer wraps it in `sh -c '...'`.
	installStep.AddCommand(plan.NewExecShellCommand("bundle config set --local without development:test"))
	installStep.AddCommand(plan.NewExecShellCommand("bundle install --jobs 4 --retry 3"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	if fw == FrameworkRails && ctx.App.HasFile("app/assets") {
		// Skip if Node.js is required but not installed in our slim base.
		// Users with full Rails asset pipelines should set
		// theopacks.json buildAptPackages: ["nodejs"] or precompile locally.
		buildStep.AddCommand(plan.NewExecShellCommand(
			"bundle exec rake assets:precompile RAILS_ENV=production || echo 'asset precompile skipped — install nodejs via theopacks.json buildAptPackages if your app needs it'",
		))
	}

	configureRubyDeploy(ctx, version, startCmd)
	return nil
}

func (p *RubyProvider) planWorkspace(ctx *generate.GenerateContext, ws *WorkspaceInfo, version string) error {
	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	name, path, ok := ws.SelectApp(appName)
	if !ok {
		if appName == "" {
			return fmt.Errorf("ruby workspace has multiple apps; set THEOPACKS_APP_NAME to one of: %s", strings.Join(ws.AppNames(), ", "))
		}
		return fmt.Errorf("ruby workspace has no app named %q; available: %s", appName, strings.Join(ws.AppNames(), ", "))
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.RubyImageForVersion(version)))
	installStep.AddCommand(plan.NewCopyCommand("Gemfile", "./"))
	if ctx.App.HasFile("Gemfile.lock") {
		installStep.AddCommand(plan.NewCopyCommand("Gemfile.lock", "./"))
	}
	// Use bundler's colon-separated multi-group form so the command body has
	// no single quotes — avoids the quote-in-quote collision once the
	// renderer wraps it in `sh -c '...'`.
	installStep.AddCommand(plan.NewExecShellCommand("bundle config set --local without development:test"))
	installStep.AddCommand(plan.NewExecShellCommand("bundle install --jobs 4 --retry 3"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	startCmd := procfileWebCommand(ctx.App)
	if startCmd == "" {
		// Convention: per-app Procfile or default ruby entrypoint.
		if ctx.App.HasFile(path + "/config.ru") {
			startCmd = fmt.Sprintf("cd %s && bundle exec rackup -p ${PORT:-4567} -o 0.0.0.0", path)
		} else {
			startCmd = fmt.Sprintf("cd %s && bundle exec ruby app.rb", path)
		}
	} else {
		// Procfile lines like `api: cd apps/api && ruby app.rb` are NOT what
		// we want — Procfile here is at the workspace root. Override only
		// when the `web:` line is set; otherwise fall back to the per-app
		// convention above.
		_ = name
	}

	configureRubyDeploy(ctx, version, startCmd)
	return nil
}

// configureRubyDeploy bakes the runtime base image, deploys the bundle plus
// the source tree, and pins BUNDLE_PATH so `bundle exec` can find gems
// without a separate setup command at runtime.
func configureRubyDeploy(ctx *generate.GenerateContext, version, startCmd string) {
	ctx.Deploy.Base = plan.NewImageLayer(generate.RubyImageForVersion(version))
	if ctx.Deploy.Variables == nil {
		ctx.Deploy.Variables = map[string]string{}
	}
	ctx.Deploy.Variables["BUNDLE_DEPLOYMENT"] = "true"
	ctx.Deploy.Variables["BUNDLE_WITHOUT"] = "development:test"
	ctx.Deploy.StartCmd = startCmd
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})
}

func (p *RubyProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *RubyProvider) StartCommandHelp() string {
	return "Ruby apps need a Gemfile. Rails / Sinatra / Rack are auto-detected; otherwise add a `web:` line to a Procfile or set THEOPACKS_START_CMD. For monorepos, set THEOPACKS_APP_NAME to an apps/<name> directory leaf."
}
