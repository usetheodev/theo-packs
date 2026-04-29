// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package java implements language detection and Dockerfile build planning
// for Java projects (Gradle and Maven). Stub registered now; real logic in
// subsequent commits.
package java

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type JavaProvider struct{}

func (p *JavaProvider) Name() string {
	return "java"
}

func (p *JavaProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return false, nil
}

func (p *JavaProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *JavaProvider) Plan(ctx *generate.GenerateContext) error {
	return nil
}

func (p *JavaProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *JavaProvider) StartCommandHelp() string {
	return "Java support is not implemented yet."
}
