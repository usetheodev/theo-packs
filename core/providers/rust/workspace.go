// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package rust

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/logger"
)

// WorkspaceInfo describes a Cargo workspace after glob expansion.
type WorkspaceInfo struct {
	// MemberPaths are the resolved relative directory paths (e.g., "apps/api").
	MemberPaths []string
	// Members maps the package name → directory path. Names come from each
	// member's Cargo.toml [package].name.
	Members map[string]string
}

// DetectWorkspace inspects the root Cargo.toml; returns nil when the project
// is not a Cargo workspace (i.e. has no [workspace] members). Glob members
// like "apps/*" are expanded against the filesystem.
func DetectWorkspace(a *app.App, log *logger.Logger) *WorkspaceInfo {
	if !a.HasFile("Cargo.toml") {
		return nil
	}

	cargo, err := parseCargoToml(a, "Cargo.toml")
	if err != nil {
		if log != nil {
			log.LogWarn("Failed to parse Cargo.toml: %v", err)
		}
		return nil
	}

	if !cargo.IsWorkspace() {
		return nil
	}

	info := &WorkspaceInfo{
		Members: make(map[string]string),
	}

	for _, raw := range cargo.Workspace.Members {
		raw = strings.TrimPrefix(raw, "./")
		raw = strings.TrimSuffix(raw, "/")

		paths := expandMember(a, raw)
		for _, p := range paths {
			memberManifest := filepath.Join(p, "Cargo.toml")
			if !a.HasFile(memberManifest) {
				continue
			}

			memberCargo, err := parseCargoToml(a, memberManifest)
			if err != nil {
				if log != nil {
					log.LogWarn("Failed to parse %s: %v", memberManifest, err)
				}
				continue
			}

			name := ""
			if memberCargo.Package != nil {
				name = memberCargo.Package.Name
			}
			if name == "" {
				// Skip workspace entries without a package name (probably
				// virtual subdirs or malformed members).
				continue
			}

			info.MemberPaths = append(info.MemberPaths, p)
			info.Members[name] = p
		}
	}

	if len(info.MemberPaths) == 0 {
		return nil
	}

	// Stable order helps with deterministic plan generation and golden tests.
	sort.Strings(info.MemberPaths)
	return info
}

// expandMember turns a workspace member entry into one or more concrete
// directory paths. Literal entries return themselves; glob entries (containing
// '*') are expanded against directories that contain a Cargo.toml.
func expandMember(a *app.App, member string) []string {
	if !strings.ContainsAny(member, "*?[") {
		return []string{member}
	}

	pattern := filepath.ToSlash(filepath.Join(member, "Cargo.toml"))
	matches, err := a.FindFiles(pattern)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, m := range matches {
		dirs = append(dirs, filepath.Dir(m))
	}
	return dirs
}

// SelectMember resolves the build target inside a workspace.
//   - When `appName` is non-empty, returns the matching member or empty if not found.
//   - When `appName` is empty and the workspace has exactly one member, returns it.
//   - Otherwise returns "" — the caller must report ambiguity to the user.
func (w *WorkspaceInfo) SelectMember(appName string) (name, path string, ok bool) {
	if appName != "" {
		if p, found := w.Members[appName]; found {
			return appName, p, true
		}
		return "", "", false
	}

	if len(w.Members) == 1 {
		for n, p := range w.Members {
			return n, p, true
		}
	}
	return "", "", false
}

// MemberNames returns workspace member names sorted alphabetically. Used to
// build helpful error messages when the user-selected app doesn't exist.
func (w *WorkspaceInfo) MemberNames() []string {
	names := make([]string, 0, len(w.Members))
	for n := range w.Members {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
