// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package rust

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCargo_PrimaryBinary_SingleBin(t *testing.T) {
	c := &CargoToml{
		Package: &CargoPackage{Name: "myapp"},
		Bin: []CargoBinTarget{
			{Name: "myapp", Path: "src/main.rs"},
		},
	}
	bin := c.PrimaryBinary()
	require.NotNil(t, bin)
	require.Equal(t, "myapp", bin.Name)
}

func TestCargo_PrimaryBinary_NoBinUsesPackageName(t *testing.T) {
	c := &CargoToml{
		Package: &CargoPackage{Name: "myapp"},
	}
	bin := c.PrimaryBinary()
	require.NotNil(t, bin)
	require.Equal(t, "myapp", bin.Name)
	require.Equal(t, "src/main.rs", bin.Path)
}

func TestCargo_PrimaryBinary_PrefersMatchingPackageName(t *testing.T) {
	c := &CargoToml{
		Package: &CargoPackage{Name: "api"},
		Bin: []CargoBinTarget{
			{Name: "helper", Path: "src/bin/helper.rs"},
			{Name: "api", Path: "src/main.rs"},
		},
	}
	bin := c.PrimaryBinary()
	require.NotNil(t, bin)
	require.Equal(t, "api", bin.Name)
}

func TestCargo_PrimaryBinary_FallsBackToFirstBinWhenNoMatch(t *testing.T) {
	c := &CargoToml{
		Package: &CargoPackage{Name: "api"},
		Bin: []CargoBinTarget{
			{Name: "helper", Path: "src/bin/helper.rs"},
		},
	}
	bin := c.PrimaryBinary()
	require.NotNil(t, bin)
	require.Equal(t, "helper", bin.Name)
}

func TestCargo_PrimaryBinary_ReturnsNilForVirtualWorkspace(t *testing.T) {
	c := &CargoToml{
		Workspace: &CargoWorkspace{Members: []string{"apps/api"}},
	}
	require.Nil(t, c.PrimaryBinary())
}

func TestCargo_IsWorkspace(t *testing.T) {
	tests := []struct {
		name string
		in   *CargoToml
		want bool
	}{
		{"plain package", &CargoToml{Package: &CargoPackage{Name: "x"}}, false},
		{"empty workspace", &CargoToml{Workspace: &CargoWorkspace{}}, false},
		{"workspace with members", &CargoToml{Workspace: &CargoWorkspace{Members: []string{"a"}}}, true},
		{"mixed package + workspace",
			&CargoToml{Package: &CargoPackage{Name: "x"}, Workspace: &CargoWorkspace{Members: []string{"a"}}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.in.IsWorkspace())
		})
	}
}

func TestCargo_IsLibraryOnly(t *testing.T) {
	tests := []struct {
		name string
		in   *CargoToml
		want bool
	}{
		{"package only — assume default bin", &CargoToml{Package: &CargoPackage{Name: "x"}}, false},
		{"explicit lib without bin", &CargoToml{Package: &CargoPackage{Name: "x"}, Lib: &CargoLibTarget{Name: "x"}}, true},
		{"lib + bin → not library only", &CargoToml{
			Package: &CargoPackage{Name: "x"},
			Bin:     []CargoBinTarget{{Name: "x"}},
			Lib:     &CargoLibTarget{Name: "x"},
		}, false},
		{"no package no bin", &CargoToml{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.in.IsLibraryOnly())
		})
	}
}

func TestParseCargoToml(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[package]
name = "myapp"
version = "0.1.0"
edition = "2021"
rust-version = "1.75"

[[bin]]
name = "server"
path = "src/main.rs"

[dependencies]
axum = "0.7"
`,
	})

	c, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)
	require.NotNil(t, c.Package)
	require.Equal(t, "myapp", c.Package.Name)
	require.Equal(t, "1.75", c.Package.RustVersion)
	require.Len(t, c.Bin, 1)
	require.Equal(t, "server", c.Bin[0].Name)
}

func TestParseCargoToml_Workspace(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api", "apps/worker", "packages/*"]
`,
	})

	c, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)
	require.NotNil(t, c.Workspace)
	require.Len(t, c.Workspace.Members, 3)
}
