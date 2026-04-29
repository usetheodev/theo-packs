// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package rust

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectRustVersion_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	require.Equal(t, "1", version)
	require.Equal(t, "default", source)
}

func TestDetectRustVersion_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_RUST_VERSION": "1.75"})
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	require.Equal(t, "1.75", version)
	require.Equal(t, "THEOPACKS_RUST_VERSION", source)
}

func TestDetectRustVersion_RustToolchainToml(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
		"rust-toolchain.toml": `[toolchain]
channel = "1.74.0"
`,
	})
	ctx := createTestContext(t, a, nil)
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	require.Equal(t, "1.74.0", version)
	require.Equal(t, "rust-toolchain.toml", source)
}

func TestDetectRustVersion_RustToolchainTomlStableNormalizesAway(t *testing.T) {
	// "stable" channel returns "" → falls through to next priority (Cargo.toml or default).
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
		"rust-toolchain.toml": `[toolchain]
channel = "stable"
`,
	})
	ctx := createTestContext(t, a, nil)
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	// Cargo.toml has no rust-version, so we fall to default.
	require.Equal(t, "1", version)
	require.Equal(t, "default", source)
}

func TestDetectRustVersion_LegacyRustToolchain(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":     minimalCargoToml,
		"src/main.rs":    minimalMainRs,
		"rust-toolchain": "1.73.0\n",
	})
	ctx := createTestContext(t, a, nil)
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	require.Equal(t, "1.73.0", version)
	require.Equal(t, "rust-toolchain", source)
}

func TestDetectRustVersion_CargoTomlRustVersion(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[package]
name = "myapp"
version = "0.1.0"
rust-version = "1.70"
`,
		"src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	require.Equal(t, "1.70", version)
	require.Equal(t, "Cargo.toml", source)
}

func TestDetectRustVersion_PriorityEnvOverridesToolchain(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
		"rust-toolchain.toml": `[toolchain]
channel = "1.74.0"
`,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_RUST_VERSION": "1.80"})
	cargo, err := parseCargoToml(a, "Cargo.toml")
	require.NoError(t, err)

	version, source := detectRustVersion(ctx, cargo)
	require.Equal(t, "1.80", version)
	require.Equal(t, "THEOPACKS_RUST_VERSION", source)
}

func TestNormalizeChannel(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"", ""},
		{"stable", ""},
		{"nightly", ""},
		{"beta", ""},
		{"1.75.0", "1.75.0"},
		{"1.75", "1.75"},
		{"  1.75  ", "1.75"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.out, normalizeChannel(tt.in), "input: %q", tt.in)
	}
}
