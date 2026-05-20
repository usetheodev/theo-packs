// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package php

import (
	"path/filepath"
	"sort"

	"github.com/usetheo/theopacks/core/app"
)

// WorkspaceInfo describes an apps/+packages/ PHP monorepo using a single root
// composer.json and per-app source layouts.
type WorkspaceInfo struct {
	AppPaths map[string]string // "api" → "apps/api"
}

func DetectWorkspace(a *app.App) *WorkspaceInfo {
	if !a.HasFile("composer.json") {
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

func (w *WorkspaceInfo) AppNames() []string {
	names := make([]string, 0, len(w.AppPaths))
	for n := range w.AppPaths {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
