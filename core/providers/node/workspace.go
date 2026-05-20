package node

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/logger"
)

type WorkspaceType int

const (
	WorkspaceNone WorkspaceType = iota
	WorkspaceNpm
	WorkspacePnpm
	WorkspaceYarn
)

type PackageManager int

const (
	PackageManagerNpm PackageManager = iota
	PackageManagerPnpm
	PackageManagerYarn
	PackageManagerBun
)

type WorkspaceInfo struct {
	Type           WorkspaceType
	PackageManager PackageManager
	MemberPaths    []string // relative dirs containing package.json, e.g. ["packages/api", "packages/shared"]
	HasTurbo       bool
}

// DetectWorkspace analyzes the app to determine if it's a Node.js workspace monorepo.
func DetectWorkspace(a *app.App, log ...*logger.Logger) *WorkspaceInfo {
	pm := DetectPackageManager(a)
	hasTurbo := a.HasFile("turbo.json")

	// Extract optional logger
	var l *logger.Logger
	if len(log) > 0 {
		l = log[0]
	}

	// pnpm-workspace.yaml is the definitive pnpm indicator
	if a.HasFile("pnpm-workspace.yaml") {
		members := resolvePnpmWorkspaceMembers(a, l)
		return &WorkspaceInfo{
			Type:           WorkspacePnpm,
			PackageManager: PackageManagerPnpm,
			MemberPaths:    members,
			HasTurbo:       hasTurbo,
		}
	}

	// Check package.json for workspaces field
	patterns := readWorkspacesField(a, l)
	if len(patterns) == 0 {
		return nil
	}

	members := resolveWorkspacePatterns(a, patterns, l)

	wsType := WorkspaceNpm
	if pm == PackageManagerYarn {
		wsType = WorkspaceYarn
	}

	return &WorkspaceInfo{
		Type:           wsType,
		PackageManager: pm,
		MemberPaths:    members,
		HasTurbo:       hasTurbo,
	}
}

// DetectPackageManager determines the package manager from lock files.
func DetectPackageManager(a *app.App) PackageManager {
	if a.HasFile("pnpm-lock.yaml") {
		return PackageManagerPnpm
	}
	if a.HasFile("yarn.lock") || a.HasFile(".yarnrc.yml") {
		return PackageManagerYarn
	}
	if a.HasFile("bun.lockb") || a.HasFile("bun.lock") {
		return PackageManagerBun
	}
	return PackageManagerNpm
}

// InstallCommand returns the appropriate install command for the package manager.
// Uses frozen lockfile when available for reproducible builds.
func InstallCommand(pm PackageManager, hasLockfile bool) string {
	switch pm {
	case PackageManagerPnpm:
		if hasLockfile {
			return "pnpm install --frozen-lockfile"
		}
		return "pnpm install"
	case PackageManagerYarn:
		if hasLockfile {
			return "yarn install --frozen-lockfile"
		}
		return "yarn install"
	case PackageManagerBun:
		if hasLockfile {
			return "bun install --frozen-lockfile"
		}
		return "bun install"
	default:
		if hasLockfile {
			return "npm ci"
		}
		return "npm install"
	}
}

// PruneCommand returns the command that drops devDependencies from an
// already-installed node_modules tree. Run AFTER `<pm> run build` in the
// build stage so the build can use dev tooling (typescript, vitest, etc.)
// but the deploy stage's COPY of /app carries only production deps.
//
// Bun has no built-in prune subcommand and its hardlinked store is already
// lean — we return "" and the caller emits no extra RUN line.
//
// For yarn classic, `yarn install --production` reinstalls without dev
// deps (yarn 1 lacks a true prune). `--ignore-scripts --prefer-offline`
// keeps the reinstall fast and safe given the install-step cache is warm.
func PruneCommand(pm PackageManager) string {
	switch pm {
	case PackageManagerPnpm:
		return "pnpm prune --prod"
	case PackageManagerYarn:
		return "yarn install --production --ignore-scripts --prefer-offline"
	case PackageManagerBun:
		return ""
	default:
		return "npm prune --omit=dev"
	}
}

// LockfileName returns the lock file name for the package manager.
func LockfileName(pm PackageManager) string {
	switch pm {
	case PackageManagerPnpm:
		return "pnpm-lock.yaml"
	case PackageManagerYarn:
		return "yarn.lock"
	case PackageManagerBun:
		return "bun.lockb"
	default:
		return "package-lock.json"
	}
}

// SetupCommand returns a command to install the package manager globally if needed.
// npm and bun are typically available in their base images, pnpm and yarn need setup.
func SetupCommand(pm PackageManager) string {
	switch pm {
	case PackageManagerPnpm:
		return "npm install -g pnpm"
	case PackageManagerYarn:
		return "corepack enable"
	default:
		return ""
	}
}

// ManifestFiles returns the list of files to copy for dependency caching.
// Always includes package.json and the lock file. For workspaces, includes
// all member package.json files and workspace config files.
func ManifestFiles(a *app.App, pm PackageManager, ws *WorkspaceInfo) []string {
	var files []string

	// Root package.json always
	files = append(files, "package.json")

	// Lock file
	lockfile := LockfileName(pm)
	if a.HasFile(lockfile) {
		files = append(files, lockfile)
	}
	// Check alternative bun lock file
	if pm == PackageManagerBun && !a.HasFile(lockfile) && a.HasFile("bun.lock") {
		files = append(files, "bun.lock")
	}

	if ws == nil {
		return files
	}

	// Workspace-specific config files
	if ws.Type == WorkspacePnpm && a.HasFile("pnpm-workspace.yaml") {
		files = append(files, "pnpm-workspace.yaml")
	}
	if ws.HasTurbo && a.HasFile("turbo.json") {
		files = append(files, "turbo.json")
	}
	if a.HasFile(".npmrc") {
		files = append(files, ".npmrc")
	}

	// Member package.json files
	for _, member := range ws.MemberPaths {
		memberPkg := filepath.Join(member, "package.json")
		if a.HasFile(memberPkg) {
			files = append(files, memberPkg)
		}
	}

	return files
}

// --- internal helpers ---

func readWorkspacesField(a *app.App, log *logger.Logger) []string {
	var pkg struct {
		Workspaces []string `json:"workspaces"`
	}
	if err := a.ReadJSON("package.json", &pkg); err != nil {
		if log != nil {
			log.LogWarn("Failed to parse package.json for workspace detection: %s", err)
		}
		return nil
	}
	return pkg.Workspaces
}

func resolvePnpmWorkspaceMembers(a *app.App, log *logger.Logger) []string {
	var config struct {
		Packages []string `yaml:"packages"`
	}
	if err := a.ReadYAML("pnpm-workspace.yaml", &config); err != nil {
		if log != nil {
			log.LogWarn("Failed to parse pnpm-workspace.yaml: %s", err)
		}
		return nil
	}
	return resolveWorkspacePatterns(a, config.Packages, log)
}

// resolveWorkspacePatterns converts workspace glob patterns (e.g., "packages/*")
// into actual directory paths that contain a package.json.
func resolveWorkspacePatterns(a *app.App, patterns []string, log *logger.Logger) []string {
	seen := make(map[string]bool)
	var members []string

	for _, pattern := range patterns {
		// Convert workspace pattern to package.json glob
		pkgPattern := fmt.Sprintf("%s/package.json", pattern)
		files, err := a.FindFiles(pkgPattern)
		if err != nil {
			if log != nil {
				log.LogWarn("Failed to resolve workspace pattern %q: %s", pattern, err)
			}
			continue
		}
		for _, f := range files {
			dir := filepath.Dir(f)
			if !seen[dir] {
				seen[dir] = true
				members = append(members, dir)
			}
		}
	}

	sort.Strings(members)
	return members
}
