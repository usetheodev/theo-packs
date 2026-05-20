// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package rust

import (
	"strings"

	"github.com/usetheo/theopacks/core/generate"
)

// rustToolchainToml is the subset of rust-toolchain.toml we need.
type rustToolchainToml struct {
	Toolchain struct {
		Channel string `toml:"channel"`
	} `toml:"toolchain"`
}

// detectRustVersion picks the Rust toolchain version, in priority order:
//  1. Config packages (theopacks.json or THEOPACKS_PACKAGES) — highest.
//  2. THEOPACKS_RUST_VERSION env var.
//  3. rust-toolchain.toml (toolchain.channel).
//  4. rust-toolchain (single-line channel, legacy format).
//  5. Cargo.toml package.rust-version (minimum Rust the crate supports).
//  6. DefaultRustVersion (rolling stable).
//
// The returned source string is used for build logs (e.g., "rust-toolchain.toml",
// "Cargo.toml", "THEOPACKS_RUST_VERSION", "default").
func detectRustVersion(ctx *generate.GenerateContext, cargo *CargoToml) (version string, source string) {
	// 1. Config packages (highest priority).
	if pkg := ctx.Resolver.Get("rust"); pkg != nil && pkg.Source != "theopacks default" {
		return pkg.Version, pkg.Source
	}

	// 2. THEOPACKS_RUST_VERSION env var.
	if envVersion, varName := ctx.Env.GetConfigVariable("RUST_VERSION"); envVersion != "" {
		return envVersion, varName
	}

	// 3. rust-toolchain.toml. Symbolic channels ("stable"/"beta"/"nightly")
	// fall through to the next priority because we can't pin them to a
	// concrete image tag.
	if ctx.App.HasFile("rust-toolchain.toml") {
		var tc rustToolchainToml
		if err := ctx.App.ReadTOML("rust-toolchain.toml", &tc); err == nil {
			if v := normalizeChannel(tc.Toolchain.Channel); v != "" {
				return v, "rust-toolchain.toml"
			}
		}
	}

	// 4. rust-toolchain (single-line legacy format), same fall-through rule.
	if ctx.App.HasFile("rust-toolchain") {
		if content, err := ctx.App.ReadFile("rust-toolchain"); err == nil {
			if v := normalizeChannel(content); v != "" {
				return v, "rust-toolchain"
			}
		}
	}

	// 5. Cargo.toml package.rust-version.
	if cargo != nil && cargo.Package != nil && cargo.Package.RustVersion != "" {
		return cargo.Package.RustVersion, "Cargo.toml"
	}

	return generate.DefaultRustVersion, "default"
}

// normalizeChannel turns rustup channel strings into Docker tag-friendly
// version numbers. "stable" / "nightly" / "beta" map to the default rolling
// tag (the empty string), letting RustBuildImageForVersion fall back to
// DefaultRustVersion. Pinned channels like "1.75.0" pass through.
func normalizeChannel(channel string) string {
	channel = strings.TrimSpace(channel)
	switch channel {
	case "", "stable", "nightly", "beta":
		return ""
	}
	return channel
}
