// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package rust

import (
	"github.com/usetheo/theopacks/core/app"
)

// CargoToml is the subset of Cargo.toml fields theo-packs needs. The TOML
// decoder ignores unknown fields, so adding more rarely causes breakage.
type CargoToml struct {
	Package   *CargoPackage    `toml:"package"`
	Workspace *CargoWorkspace  `toml:"workspace"`
	Bin       []CargoBinTarget `toml:"bin"`
	Lib       *CargoLibTarget  `toml:"lib"`
}

type CargoPackage struct {
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Edition     string `toml:"edition"`
	RustVersion string `toml:"rust-version"`
}

type CargoWorkspace struct {
	Members []string `toml:"members"`
}

type CargoBinTarget struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type CargoLibTarget struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// parseCargoToml reads and decodes Cargo.toml from the given relative path
// (typically "Cargo.toml" for the root, or "apps/api/Cargo.toml" for a workspace member).
func parseCargoToml(a *app.App, path string) (*CargoToml, error) {
	var cargo CargoToml
	if err := a.ReadTOML(path, &cargo); err != nil {
		return nil, err
	}
	return &cargo, nil
}

// PrimaryBinary returns the binary target Theo will build:
//   - If [[bin]] entries exist, returns the first one (or the one matching the
//     package name, when present, since that's the conventional default).
//   - Otherwise, falls back to package.name + src/main.rs (Cargo's default
//     binary when no explicit [[bin]] is declared).
//
// Returns nil only when the crate is library-only (no [[bin]] AND no package).
func (c *CargoToml) PrimaryBinary() *CargoBinTarget {
	if len(c.Bin) > 0 {
		// Prefer a [[bin]] whose name matches the package name (idiomatic).
		if c.Package != nil && c.Package.Name != "" {
			for i := range c.Bin {
				if c.Bin[i].Name == c.Package.Name {
					return &c.Bin[i]
				}
			}
		}
		return &c.Bin[0]
	}

	if c.Package != nil && c.Package.Name != "" {
		// Cargo's default: package name == binary name, src/main.rs is the entry.
		return &CargoBinTarget{Name: c.Package.Name, Path: "src/main.rs"}
	}

	return nil
}

// IsWorkspace reports whether this Cargo.toml declares a workspace.
// Virtual workspaces (no [package], only [workspace]) and mixed workspaces
// (both [package] and [workspace]) both return true.
func (c *CargoToml) IsWorkspace() bool {
	return c.Workspace != nil && len(c.Workspace.Members) > 0
}

// IsLibraryOnly reports whether this is a pure library crate with no binary
// to deploy (no [[bin]], explicit [lib] section, no [package] for default
// binary fallback).
func (c *CargoToml) IsLibraryOnly() bool {
	if len(c.Bin) > 0 {
		return false
	}
	if c.Package == nil {
		// No package and no bin → can't deploy anything from this crate.
		return true
	}
	// A [package] alone usually means default binary. Library-only is signaled
	// by an explicit [lib] section together with absence of src/main.rs.
	if c.Lib != nil {
		return true
	}
	return false
}
