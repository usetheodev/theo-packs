package node

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
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

// --- DetectPackageManager ---

func TestDetectPackageManager_Npm(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":      `{"name":"test"}`,
		"package-lock.json": "{}",
	})
	require.Equal(t, PackageManagerNpm, DetectPackageManager(a))
}

func TestDetectPackageManager_Pnpm(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":   `{"name":"test"}`,
		"pnpm-lock.yaml": "lockfileVersion: 6",
	})
	require.Equal(t, PackageManagerPnpm, DetectPackageManager(a))
}

func TestDetectPackageManager_Yarn(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json": `{"name":"test"}`,
		"yarn.lock":    "# yarn lock",
	})
	require.Equal(t, PackageManagerYarn, DetectPackageManager(a))
}

func TestDetectPackageManager_Bun(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json": `{"name":"test"}`,
		"bun.lockb":    "",
	})
	require.Equal(t, PackageManagerBun, DetectPackageManager(a))
}

func TestDetectPackageManager_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json": `{"name":"test"}`,
	})
	require.Equal(t, PackageManagerNpm, DetectPackageManager(a))
}

// --- DetectWorkspace ---

func TestDetectWorkspace_NpmWorkspaces(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":                 `{"name":"root","workspaces":["packages/*"]}`,
		"package-lock.json":            "{}",
		"packages/api/package.json":    `{"name":"api"}`,
		"packages/shared/package.json": `{"name":"shared"}`,
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Equal(t, WorkspaceNpm, ws.Type)
	require.Equal(t, PackageManagerNpm, ws.PackageManager)
	require.Equal(t, []string{"packages/api", "packages/shared"}, ws.MemberPaths)
	require.False(t, ws.HasTurbo)
}

func TestDetectWorkspace_PnpmWorkspaces(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":                `{"name":"root"}`,
		"pnpm-lock.yaml":              "lockfileVersion: 6",
		"pnpm-workspace.yaml":         "packages:\n  - \"packages/*\"",
		"packages/pkg-a/package.json": `{"name":"pkg-a"}`,
		"packages/pkg-b/package.json": `{"name":"pkg-b"}`,
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Equal(t, WorkspacePnpm, ws.Type)
	require.Equal(t, PackageManagerPnpm, ws.PackageManager)
	require.Equal(t, []string{"packages/pkg-a", "packages/pkg-b"}, ws.MemberPaths)
}

func TestDetectWorkspace_YarnWorkspaces(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":              `{"name":"root","workspaces":["packages/*"]}`,
		"yarn.lock":                 "# yarn",
		"packages/api/package.json": `{"name":"api"}`,
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Equal(t, WorkspaceYarn, ws.Type)
	require.Equal(t, PackageManagerYarn, ws.PackageManager)
	require.Equal(t, []string{"packages/api"}, ws.MemberPaths)
}

func TestDetectWorkspace_Turborepo(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":             `{"name":"root","workspaces":["apps/*","packages/*"]}`,
		"package-lock.json":        "{}",
		"turbo.json":               `{"tasks":{}}`,
		"apps/web/package.json":    `{"name":"web"}`,
		"apps/api/package.json":    `{"name":"api"}`,
		"packages/ui/package.json": `{"name":"ui"}`,
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Equal(t, WorkspaceNpm, ws.Type) // turbo uses npm workspaces under the hood
	require.True(t, ws.HasTurbo)
	require.Equal(t, []string{"apps/api", "apps/web", "packages/ui"}, ws.MemberPaths)
}

func TestDetectWorkspace_NoWorkspace(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json": `{"name":"simple"}`,
	})
	ws := DetectWorkspace(a)
	require.Nil(t, ws)
}

// --- InstallCommand ---

func TestInstallCommand_NpmWithLock(t *testing.T) {
	require.Equal(t, "npm ci", InstallCommand(PackageManagerNpm, true))
}

func TestInstallCommand_NpmNoLock(t *testing.T) {
	require.Equal(t, "npm install", InstallCommand(PackageManagerNpm, false))
}

func TestInstallCommand_PnpmWithLock(t *testing.T) {
	require.Equal(t, "pnpm install --frozen-lockfile", InstallCommand(PackageManagerPnpm, true))
}

func TestInstallCommand_YarnWithLock(t *testing.T) {
	require.Equal(t, "yarn install --frozen-lockfile", InstallCommand(PackageManagerYarn, true))
}

func TestInstallCommand_BunWithLock(t *testing.T) {
	require.Equal(t, "bun install --frozen-lockfile", InstallCommand(PackageManagerBun, true))
}

// --- ManifestFiles ---

func TestManifestFiles_Simple(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":      `{"name":"test"}`,
		"package-lock.json": "{}",
	})
	files := ManifestFiles(a, PackageManagerNpm, nil)
	require.Equal(t, []string{"package.json", "package-lock.json"}, files)
}

func TestManifestFiles_Workspace(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":                 `{"name":"root","workspaces":["packages/*"]}`,
		"package-lock.json":            "{}",
		"turbo.json":                   `{}`,
		"packages/api/package.json":    `{"name":"api"}`,
		"packages/shared/package.json": `{"name":"shared"}`,
	})
	ws := &WorkspaceInfo{
		Type:        WorkspaceNpm,
		MemberPaths: []string{"packages/api", "packages/shared"},
		HasTurbo:    true,
	}
	files := ManifestFiles(a, PackageManagerNpm, ws)
	require.Contains(t, files, "package.json")
	require.Contains(t, files, "package-lock.json")
	require.Contains(t, files, "turbo.json")
	require.Contains(t, files, "packages/api/package.json")
	require.Contains(t, files, "packages/shared/package.json")
}

func TestDetectWorkspace_UnresolvableGlob(t *testing.T) {
	// Workspace glob pattern that matches nothing should return empty members
	a := createTempApp(t, map[string]string{
		"package.json": `{"name":"root","workspaces":["nonexistent/*"]}`,
	})
	ws := DetectWorkspace(a)
	// Patterns are present so workspace is detected, but members are empty
	require.NotNil(t, ws)
	require.Empty(t, ws.MemberPaths)
}

func TestDetectWorkspace_NestedPatterns(t *testing.T) {
	// Nested workspace pattern packages/** should find deeply nested packages
	a := createTempApp(t, map[string]string{
		"package.json":                       `{"name":"root","workspaces":["packages/**"]}`,
		"packages/api/package.json":          `{"name":"api"}`,
		"packages/shared/utils/package.json": `{"name":"utils"}`,
	})
	ws := DetectWorkspace(a)
	require.NotNil(t, ws)
	require.Contains(t, ws.MemberPaths, "packages/api")
	require.Contains(t, ws.MemberPaths, "packages/shared/utils")
}

func TestManifestFiles_Pnpm(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"package.json":                `{"name":"root"}`,
		"pnpm-lock.yaml":              "lockfileVersion: 6",
		"pnpm-workspace.yaml":         "packages:\n  - packages/*",
		"packages/pkg-a/package.json": `{"name":"pkg-a"}`,
	})
	ws := &WorkspaceInfo{
		Type:        WorkspacePnpm,
		MemberPaths: []string{"packages/pkg-a"},
	}
	files := ManifestFiles(a, PackageManagerPnpm, ws)
	require.Contains(t, files, "package.json")
	require.Contains(t, files, "pnpm-lock.yaml")
	require.Contains(t, files, "pnpm-workspace.yaml")
	require.Contains(t, files, "packages/pkg-a/package.json")
}

// --- PruneCommand ---

func TestPruneCommand_Npm(t *testing.T) {
	require.Equal(t, "npm prune --omit=dev", PruneCommand(PackageManagerNpm))
}

func TestPruneCommand_Pnpm(t *testing.T) {
	require.Equal(t, "pnpm prune --prod", PruneCommand(PackageManagerPnpm))
}

func TestPruneCommand_Yarn(t *testing.T) {
	require.Equal(t, "yarn install --production --ignore-scripts --prefer-offline", PruneCommand(PackageManagerYarn))
}

func TestPruneCommand_Bun(t *testing.T) {
	require.Equal(t, "", PruneCommand(PackageManagerBun))
}
