// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package php implements language detection and Dockerfile build planning
// for PHP projects (Composer, Laravel, Slim). Stub registered now; real logic
// in subsequent commits.
package php

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type PhpProvider struct{}

func (p *PhpProvider) Name() string {
	return "php"
}

func (p *PhpProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return false, nil
}

func (p *PhpProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *PhpProvider) Plan(ctx *generate.GenerateContext) error {
	return nil
}

func (p *PhpProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *PhpProvider) StartCommandHelp() string {
	return "PHP support is not implemented yet."
}
