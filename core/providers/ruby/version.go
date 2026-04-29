// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package ruby

import (
	"strings"

	"github.com/usetheo/theopacks/core/generate"
)

// detectRubyVersion picks the Ruby major.minor version, in priority order:
//  1. Config packages (theopacks.json / THEOPACKS_PACKAGES).
//  2. THEOPACKS_RUBY_VERSION env var.
//  3. .ruby-version file (rbenv / chruby standard).
//  4. Gemfile `ruby "X.Y"` directive.
//  5. DefaultRubyVersion (3.3).
func detectRubyVersion(ctx *generate.GenerateContext) (version string, source string) {
	if pkg := ctx.Resolver.Get("ruby"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajorMinor(pkg.Version), pkg.Source
	}

	if envVersion, varName := ctx.Env.GetConfigVariable("RUBY_VERSION"); envVersion != "" {
		return generate.NormalizeToMajorMinor(envVersion), varName
	}

	if ctx.App.HasFile(".ruby-version") {
		if content, err := ctx.App.ReadFile(".ruby-version"); err == nil {
			if v := strings.TrimSpace(content); v != "" {
				return generate.NormalizeToMajorMinor(v), ".ruby-version"
			}
		}
	}

	if ctx.App.HasFile("Gemfile") {
		if content, err := ctx.App.ReadFile("Gemfile"); err == nil {
			if v := rubyVersionFromGemfile(content); v != "" {
				return generate.NormalizeToMajorMinor(v), "Gemfile"
			}
		}
	}

	return generate.DefaultRubyVersion, "default"
}
