// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package deno implements language detection and Dockerfile build planning
// for Deno projects (deno.json/deno.jsonc, Fresh, Hono, Deno workspaces).
// This provider is registered BEFORE Node so projects shipping both deno.json
// and a npm-compat package.json route correctly. Stub registered now; real
// logic in subsequent commits.
package deno

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type DenoProvider struct{}

func (p *DenoProvider) Name() string {
	return "deno"
}

func (p *DenoProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return false, nil
}

func (p *DenoProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *DenoProvider) Plan(ctx *generate.GenerateContext) error {
	return nil
}

func (p *DenoProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *DenoProvider) StartCommandHelp() string {
	return "Deno support is not implemented yet."
}
