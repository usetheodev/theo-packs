// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package dotnet implements language detection and Dockerfile build planning
// for .NET projects (.csproj/.fsproj/.vbproj single-project and .sln solutions).
// Stub registered now; real logic in subsequent commits.
package dotnet

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type DotnetProvider struct{}

func (p *DotnetProvider) Name() string {
	return "dotnet"
}

func (p *DotnetProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return false, nil
}

func (p *DotnetProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *DotnetProvider) Plan(ctx *generate.GenerateContext) error {
	return nil
}

func (p *DotnetProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *DotnetProvider) StartCommandHelp() string {
	return ".NET support is not implemented yet."
}
