// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package deno

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/logger"
)

// WorkspaceInfo describes a Deno 2 workspace.
type WorkspaceInfo struct {
	Members  map[string]string // member name (or path leaf) → path
}

// DetectWorkspace returns workspace info when the root deno.json has a non-
// empty `workspace` array. Each member entry can be a literal directory path
// (relative to the project root); we don't expand globs because Deno's spec
// uses literal paths.
func DetectWorkspace(a *app.App, log *logger.Logger) *WorkspaceInfo {
	cfg, _, err := readDenoConfig(a)
	if err != nil || cfg == nil || len(cfg.Workspace) == 0 {
		return nil
	}

	info := &WorkspaceInfo{Members: make(map[string]string)}
	for _, raw := range cfg.Workspace {
		raw = strings.TrimPrefix(raw, "./")
		raw = strings.TrimSuffix(raw, "/")
		if raw == "" {
			continue
		}

		// Each member should have its own deno.json (Deno 2 convention).
		var memberCfg *DenoConfig
		for _, manifestName := range []string{raw + "/deno.json", raw + "/deno.jsonc"} {
			if a.HasFile(manifestName) {
				var c DenoConfig
				if err := a.ReadJSON(manifestName, &c); err == nil {
					memberCfg = &c
				}
				break
			}
		}

		// Determine the workspace-facing name. Prefer the explicit Deno 2
		// "name" (which is namespaced like "@scope/api"); strip the scope so
		// THEOPACKS_APP_NAME=api matches "@scope/api".
		name := filepath.Base(raw)
		if memberCfg != nil && memberCfg.Name != "" {
			n := memberCfg.Name
			if i := strings.LastIndex(n, "/"); i >= 0 {
				n = n[i+1:]
			}
			if n != "" {
				name = n
			}
		}

		info.Members[name] = raw
	}
	if len(info.Members) == 0 {
		return nil
	}
	_ = log
	return info
}

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

func (w *WorkspaceInfo) MemberNames() []string {
	names := make([]string, 0, len(w.Members))
	for n := range w.Members {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
