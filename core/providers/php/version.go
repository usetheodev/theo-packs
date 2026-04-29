// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package php

import (
	"strings"

	"github.com/usetheo/theopacks/core/generate"
)

// detectPhpVersion picks the PHP major.minor version, in priority order:
//  1. Config packages (theopacks.json / THEOPACKS_PACKAGES).
//  2. THEOPACKS_PHP_VERSION env var.
//  3. .php-version file.
//  4. composer.json `require.php` (e.g., "^8.2", ">=8.1").
//  5. DefaultPhpVersion (8.3 — current stable).
func detectPhpVersion(ctx *generate.GenerateContext, composer *ComposerJson) (version string, source string) {
	if pkg := ctx.Resolver.Get("php"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajorMinor(pkg.Version), pkg.Source
	}

	if envVersion, varName := ctx.Env.GetConfigVariable("PHP_VERSION"); envVersion != "" {
		return generate.NormalizeToMajorMinor(envVersion), varName
	}

	if ctx.App.HasFile(".php-version") {
		if content, err := ctx.App.ReadFile(".php-version"); err == nil {
			if v := strings.TrimSpace(content); v != "" {
				return generate.NormalizeToMajorMinor(v), ".php-version"
			}
		}
	}

	if composer != nil {
		if php, ok := composer.Require["php"]; ok && php != "" {
			return generate.NormalizeToMajorMinor(php), "composer.json"
		}
	}

	return generate.DefaultPhpVersion, "default"
}
