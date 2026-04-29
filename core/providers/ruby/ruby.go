// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package ruby implements language detection and Dockerfile build planning
// for Ruby projects (Bundler, Rails, Sinatra). Stub registered now; real logic
// in subsequent commits.
package ruby

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type RubyProvider struct{}

func (p *RubyProvider) Name() string {
	return "ruby"
}

func (p *RubyProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return false, nil
}

func (p *RubyProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *RubyProvider) Plan(ctx *generate.GenerateContext) error {
	return nil
}

func (p *RubyProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *RubyProvider) StartCommandHelp() string {
	return "Ruby support is not implemented yet."
}
