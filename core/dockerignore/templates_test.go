// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package dockerignore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultFor_Common(t *testing.T) {
	out := DefaultFor("")
	require.Contains(t, out, ".git/")
	require.Contains(t, out, ".DS_Store")
}

func TestDefaultFor_Node(t *testing.T) {
	out := DefaultFor("node")
	require.Contains(t, out, ".git/", "must include common base")
	require.Contains(t, out, "node_modules/")
	require.Contains(t, out, ".next/cache/")
}

func TestDefaultFor_Python(t *testing.T) {
	out := DefaultFor("python")
	require.Contains(t, out, "__pycache__/")
	require.Contains(t, out, ".venv/")
	require.Contains(t, out, ".env")
}

func TestDefaultFor_Java(t *testing.T) {
	out := DefaultFor("java")
	require.Contains(t, out, "target/")
	require.Contains(t, out, ".gradle/")
}

func TestDefaultFor_Rust(t *testing.T) {
	out := DefaultFor("rust")
	require.Contains(t, out, "target/")
}

func TestDefaultFor_Dotnet(t *testing.T) {
	out := DefaultFor("dotnet")
	require.Contains(t, out, "bin/")
	require.Contains(t, out, "obj/")
}

func TestDefaultFor_Ruby(t *testing.T) {
	out := DefaultFor("ruby")
	require.Contains(t, out, ".bundle/")
	require.Contains(t, out, "vendor/bundle/")
}

func TestDefaultFor_Php(t *testing.T) {
	out := DefaultFor("php")
	require.Contains(t, out, "vendor/")
}

func TestDefaultFor_Deno(t *testing.T) {
	out := DefaultFor("deno")
	require.Contains(t, out, ".deno/")
}

func TestDefaultFor_Go(t *testing.T) {
	out := DefaultFor("go")
	require.Contains(t, out, "vendor/")
}

func TestDefaultFor_Unknown(t *testing.T) {
	// Unknown name still returns the common base (.git is excluded).
	out := DefaultFor("unknown-language")
	require.Contains(t, out, ".git/")
	require.NotContains(t, out, "node_modules/")
	require.NotContains(t, out, "__pycache__/")
}

func TestDefaultFor_TrailingNewline(t *testing.T) {
	out := DefaultFor("node")
	require.True(t, strings.HasSuffix(out, "\n"), "output must end with newline")
	require.False(t, strings.HasSuffix(out, "\n\n\n"), "no excessive trailing newlines")
}

func TestDefaultFor_StaticfileShell(t *testing.T) {
	// Static and shell providers get only the common base — no language-
	// specific exclusions. Confirm both behave consistently.
	staticOut := DefaultFor("staticfile")
	shellOut := DefaultFor("shell")
	require.Equal(t, staticOut, shellOut)
	require.Contains(t, staticOut, ".git/")
}
