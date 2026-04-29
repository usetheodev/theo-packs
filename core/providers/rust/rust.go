// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package rust implements language detection and Dockerfile build planning
// for Rust projects. The full implementation lands in subsequent commits;
// this stub satisfies the Provider interface so it can be registered.
package rust

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type RustProvider struct{}

func (p *RustProvider) Name() string {
	return "rust"
}

func (p *RustProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return false, nil
}

func (p *RustProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *RustProvider) Plan(ctx *generate.GenerateContext) error {
	return nil
}

func (p *RustProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *RustProvider) StartCommandHelp() string {
	return "Rust support is not implemented yet."
}
