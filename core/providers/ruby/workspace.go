// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package ruby

import (
	"path/filepath"
	"sort"

	"github.com/usetheo/theopacks/core/app"
)

// WorkspaceInfo describes an apps/+packages/ Ruby monorepo. We do not parse
// per-app Gemfiles individually here; the assumption is that the root Gemfile
// provides the unified bundle for every app, and the start command is
// per-app via Procfile (or auto-detected).
type WorkspaceInfo struct {
	AppPaths map[string]string // "api" → "apps/api"
}

// DetectWorkspace returns workspace info if the project has an apps/ tree and
// at least one app under it (apps/<name>) plus a root Gemfile. Returns nil
// for non-monorepo projects.
func DetectWorkspace(a *app.App) *WorkspaceInfo {
	if !a.HasFile("Gemfile") {
		return nil
	}
	dirs, err := a.FindDirectories("apps/*")
	if err != nil || len(dirs) == 0 {
		return nil
	}

	info := &WorkspaceInfo{AppPaths: make(map[string]string)}
	for _, d := range dirs {
		name := filepath.Base(d)
		if name == "" {
			continue
		}
		info.AppPaths[name] = d
	}
	if len(info.AppPaths) == 0 {
		return nil
	}
	return info
}

// SelectApp resolves which app to deploy. With THEOPACKS_APP_NAME set, returns
// the matching app. With exactly one app present, returns it. Otherwise nil/false.
func (w *WorkspaceInfo) SelectApp(appName string) (name, path string, ok bool) {
	if appName != "" {
		if p, found := w.AppPaths[appName]; found {
			return appName, p, true
		}
		return "", "", false
	}
	if len(w.AppPaths) == 1 {
		for n, p := range w.AppPaths {
			return n, p, true
		}
	}
	return "", "", false
}

// AppNames returns the workspace app names sorted alphabetically.
func (w *WorkspaceInfo) AppNames() []string {
	names := make([]string, 0, len(w.AppPaths))
	for n := range w.AppPaths {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
