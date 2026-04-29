// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package deno

import (
	"github.com/usetheo/theopacks/core/generate"
)

// detectDenoVersion picks the Deno major version, in priority order:
//  1. Config packages (theopacks.json / THEOPACKS_PACKAGES).
//  2. THEOPACKS_DENO_VERSION env var.
//  3. DefaultDenoVersion ("2" — current major).
//
// Deno doesn't have a canonical version-pinning file. .nvmrc / .tool-versions
// support could be added later if needed; it's rare in the wild.
func detectDenoVersion(ctx *generate.GenerateContext) (version string, source string) {
	if pkg := ctx.Resolver.Get("deno"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajor(pkg.Version), pkg.Source
	}

	if envVersion, varName := ctx.Env.GetConfigVariable("DENO_VERSION"); envVersion != "" {
		return generate.NormalizeToMajor(envVersion), varName
	}

	return generate.DefaultDenoVersion, "default"
}
