// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package dotnet

import (
	"github.com/usetheo/theopacks/core/generate"
)

// globalJson is the subset of global.json we need.
type globalJson struct {
	Sdk struct {
		Version string `json:"version"`
	} `json:"sdk"`
}

// detectDotnetVersion picks the .NET SDK version, in priority order:
//  1. Config packages (theopacks.json / THEOPACKS_PACKAGES).
//  2. THEOPACKS_DOTNET_VERSION env var.
//  3. global.json sdk.version (canonical pinning mechanism in .NET).
//  4. Project file <TargetFramework> (net8.0 → 8.0).
//  5. DefaultDotnetVersion (LTS 8.0).
func detectDotnetVersion(ctx *generate.GenerateContext, primary *Project) (version string, source string) {
	if pkg := ctx.Resolver.Get("dotnet"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajorMinor(pkg.Version), pkg.Source
	}

	if envVersion, varName := ctx.Env.GetConfigVariable("DOTNET_VERSION"); envVersion != "" {
		return generate.NormalizeToMajorMinor(envVersion), varName
	}

	if ctx.App.HasFile("global.json") {
		var gj globalJson
		if err := ctx.App.ReadJSON("global.json", &gj); err == nil && gj.Sdk.Version != "" {
			return generate.NormalizeToMajorMinor(gj.Sdk.Version), "global.json"
		}
	}

	if primary != nil {
		if tfm := primary.TargetFramework(); tfm != "" {
			if v := TfmToVersion(tfm); v != "" {
				return v, "TargetFramework"
			}
		}
	}

	return generate.DefaultDotnetVersion, "default"
}
