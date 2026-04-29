// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package rust

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/logger"
)

func TestDetectWorkspace_NoCargoToml(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json": `{"name":"test"}`,
	})
	require.Nil(t, DetectWorkspace(a, logger.NewLogger()))
}

func TestDetectWorkspace_PlainPackage(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml":  minimalCargoToml,
		"src/main.rs": minimalMainRs,
	})
	require.Nil(t, DetectWorkspace(a, logger.NewLogger()))
}

func TestDetectWorkspace_LiteralMembers(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api", "apps/worker"]
`,
		"apps/api/Cargo.toml": `[package]
name = "api"
version = "0.1.0"
`,
		"apps/worker/Cargo.toml": `[package]
name = "worker"
version = "0.1.0"
`,
	})

	ws := DetectWorkspace(a, logger.NewLogger())
	require.NotNil(t, ws)
	require.Equal(t, []string{"apps/api", "apps/worker"}, ws.MemberPaths)
	require.Equal(t, "apps/api", ws.Members["api"])
	require.Equal(t, "apps/worker", ws.Members["worker"])
}

func TestDetectWorkspace_GlobMembers(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/*", "packages/*"]
`,
		"apps/api/Cargo.toml":         `[package]` + "\n" + `name = "api"` + "\n",
		"apps/web/Cargo.toml":         `[package]` + "\n" + `name = "web"` + "\n",
		"packages/shared/Cargo.toml":  `[package]` + "\n" + `name = "shared"` + "\n",
	})

	ws := DetectWorkspace(a, logger.NewLogger())
	require.NotNil(t, ws)
	require.Len(t, ws.MemberPaths, 3)
	require.Contains(t, ws.Members, "api")
	require.Contains(t, ws.Members, "web")
	require.Contains(t, ws.Members, "shared")
}

func TestDetectWorkspace_VirtualRoot(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api"]
`,
		"apps/api/Cargo.toml": `[package]` + "\n" + `name = "api"` + "\n",
	})
	ws := DetectWorkspace(a, logger.NewLogger())
	require.NotNil(t, ws)
	require.Len(t, ws.MemberPaths, 1)
}

func TestDetectWorkspace_SkipsMissingMembers(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api", "apps/missing"]
`,
		"apps/api/Cargo.toml": `[package]` + "\n" + `name = "api"` + "\n",
		// apps/missing/Cargo.toml is intentionally absent.
	})
	ws := DetectWorkspace(a, logger.NewLogger())
	require.NotNil(t, ws)
	require.Len(t, ws.MemberPaths, 1)
	require.Equal(t, "apps/api", ws.MemberPaths[0])
}

func TestDetectWorkspace_SkipsMembersWithoutPackageName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"Cargo.toml": `[workspace]
members = ["apps/api"]
`,
		"apps/api/Cargo.toml": `[lib]` + "\n",
	})
	ws := DetectWorkspace(a, logger.NewLogger())
	require.Nil(t, ws, "workspace with no nameable members should return nil")
}

func TestSelectMember_AppNameSelects(t *testing.T) {
	ws := &WorkspaceInfo{
		MemberPaths: []string{"apps/api", "apps/worker"},
		Members:     map[string]string{"api": "apps/api", "worker": "apps/worker"},
	}
	name, path, ok := ws.SelectMember("api")
	require.True(t, ok)
	require.Equal(t, "api", name)
	require.Equal(t, "apps/api", path)
}

func TestSelectMember_AppNameMissing(t *testing.T) {
	ws := &WorkspaceInfo{
		Members: map[string]string{"api": "apps/api"},
	}
	_, _, ok := ws.SelectMember("nope")
	require.False(t, ok)
}

func TestSelectMember_SingleMemberAutoSelect(t *testing.T) {
	ws := &WorkspaceInfo{
		Members: map[string]string{"only": "apps/only"},
	}
	name, path, ok := ws.SelectMember("")
	require.True(t, ok)
	require.Equal(t, "only", name)
	require.Equal(t, "apps/only", path)
}

func TestSelectMember_MultipleMembersNoAppName(t *testing.T) {
	ws := &WorkspaceInfo{
		Members: map[string]string{"api": "apps/api", "worker": "apps/worker"},
	}
	_, _, ok := ws.SelectMember("")
	require.False(t, ok, "ambiguous workspace must report no selection")
}

func TestMemberNames_Sorted(t *testing.T) {
	ws := &WorkspaceInfo{
		Members: map[string]string{"worker": "apps/worker", "api": "apps/api", "z": "apps/z"},
	}
	require.Equal(t, []string{"api", "worker", "z"}, ws.MemberNames())
}
