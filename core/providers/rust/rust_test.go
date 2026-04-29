// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package rust

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
)

func createTempApp(t *testing.T, files map[string]string) *app.App {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}
	a, err := app.NewApp(dir)
	require.NoError(t, err)
	return a
}

func createTestContext(t *testing.T, a *app.App, envVars map[string]string) *generate.GenerateContext {
	t.Helper()
	var envPtr *map[string]string
	if envVars != nil {
		envPtr = &envVars
	}
	env := app.NewEnvironment(envPtr)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)
	return ctx
}

const minimalCargoToml = `[package]
name = "myapp"
version = "0.1.0"
edition = "2021"

[dependencies]
`

const minimalMainRs = `fn main() { println!("hello"); }`

func TestRustProvider_Name(t *testing.T) {
	require.Equal(t, "rust", (&RustProvider{}).Name())
}

func TestRustProvider_Detect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{
			name:     "detects Cargo.toml",
			files:    map[string]string{"Cargo.toml": minimalCargoToml},
			expected: true,
		},
		{
			name:     "no Cargo.toml",
			files:    map[string]string{"package.json": `{"name":"test"}`},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)
			detected, err := (&RustProvider{}).Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, detected)
		})
	}
}

func TestRustProvider_StartCommandHelp(t *testing.T) {
	help := (&RustProvider{}).StartCommandHelp()
	require.NotEmpty(t, help)
	require.Contains(t, help, "[[bin]]")
	require.Contains(t, help, "THEOPACKS_APP_NAME")
}

func TestRustProvider_PlanSimple(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.NoError(t, err)

	require.Equal(t, "/app/server", ctx.Deploy.StartCmd)
	require.Len(t, ctx.Steps, 2)
	require.Equal(t, "install", ctx.Steps[0].Name())
	require.Equal(t, "build", ctx.Steps[1].Name())
	require.Contains(t, ctx.Deploy.AptPackages, "ca-certificates")
}

func TestRustProvider_PlanSimple_WithCargoLock(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"Cargo.lock":  "# cargo lockfile\n",
		"src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Len(t, ctx.Steps, 2)
}

func TestRustProvider_PlanSimple_LibraryOnlyReturnsError(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[package]
name = "mylib"
version = "0.1.0"

[lib]
name = "mylib"
path = "src/lib.rs"

[dependencies]
`,
		"src/lib.rs": "pub fn add(a: i32, b: i32) -> i32 { a + b }",
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "library-only")
}

func TestRustProvider_PlanSimple_BinTargetUsesPackageNameWhenMatching(t *testing.T) {
	// When [[bin]] has a name matching package.name, use that one.
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[package]
name = "myapp"
version = "0.1.0"

[[bin]]
name = "helper"
path = "src/bin/helper.rs"

[[bin]]
name = "myapp"
path = "src/main.rs"
`,
		"src/main.rs":       minimalMainRs,
		"src/bin/helper.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "/app/server", ctx.Deploy.StartCmd)
}

func TestRustProvider_PlanWorkspace_SingleMember(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api"]
`,
		"apps/api/Cargo.toml": `[package]
name = "api"
version = "0.1.0"
edition = "2021"
`,
		"apps/api/src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "/app/server", ctx.Deploy.StartCmd)
}

func TestRustProvider_PlanWorkspace_MultipleMembersRequiresAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api", "apps/worker"]
`,
		"apps/api/Cargo.toml": `[package]
name = "api"
version = "0.1.0"
`,
		"apps/api/src/main.rs": minimalMainRs,
		"apps/worker/Cargo.toml": `[package]
name = "worker"
version = "0.1.0"
`,
		"apps/worker/src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "THEOPACKS_APP_NAME")
	require.Contains(t, err.Error(), "api")
	require.Contains(t, err.Error(), "worker")
}

func TestRustProvider_PlanWorkspace_AppNameSelectsMember(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api", "apps/worker"]
`,
		"apps/api/Cargo.toml": `[package]
name = "api"
version = "0.1.0"
`,
		"apps/api/src/main.rs": minimalMainRs,
		"apps/worker/Cargo.toml": `[package]
name = "worker"
version = "0.1.0"
`,
		"apps/worker/src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "api"})

	err := (&RustProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "/app/server", ctx.Deploy.StartCmd)
}

func TestRustProvider_PlanWorkspace_UnknownAppNameError(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api"]
`,
		"apps/api/Cargo.toml": `[package]
name = "api"
version = "0.1.0"
`,
		"apps/api/src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "nope"})

	err := (&RustProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no member named")
	require.Contains(t, err.Error(), "api")
}

func TestRustProvider_PlanWorkspace_GlobMembers(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/*"]
`,
		"apps/api/Cargo.toml": `[package]
name = "api"
version = "0.1.0"
`,
		"apps/api/src/main.rs": minimalMainRs,
		"apps/web/Cargo.toml": `[package]
name = "web"
version = "0.1.0"
`,
		"apps/web/src/main.rs": minimalMainRs,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "web"})

	err := (&RustProvider{}).Plan(ctx)
	require.NoError(t, err)
}

func TestRustProvider_PlanInvalidCargoToml(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": "this is not valid TOML at all !!!",
	})
	ctx := createTestContext(t, a, nil)

	err := (&RustProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Cargo.toml")
}

func TestRustProvider_CleansePlan_NoOp(t *testing.T) {
	(&RustProvider{}).CleansePlan(nil)
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"myapp", "myapp"},
		{"my-app", "my-app"},
		{"my_app", "my_app"},
		{"app123", "app123"},
		{"bad;rm -rf /", "badrm-rf"},
		{"$(evil)", "evil"},
		{"", ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.out, shellEscape(tt.in), "input: %q", tt.in)
	}
}
