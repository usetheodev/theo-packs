// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package deno

import (
	"github.com/usetheo/theopacks/core/app"
)

// DenoConfig is the subset of deno.json / deno.jsonc fields theo-packs needs.
// The existing app.ReadJSON helper handles JSONC (tailscale/hujson is in
// dependency graph), so .jsonc files parse fine.
type DenoConfig struct {
	Imports   map[string]string `json:"imports"`
	Tasks     map[string]string `json:"tasks"`
	Workspace []string          `json:"workspace"`
	// Deno 2 module name (when present in member configs).
	Name string `json:"name"`
	// Deno 2 lock setting.
	Lock any `json:"lock"`
}

// readDenoConfig loads deno.json or deno.jsonc from the given app, returning
// the first one found. JSONC tolerance is provided by ReadJSON via hujson.
func readDenoConfig(a *app.App) (*DenoConfig, string, error) {
	for _, name := range []string{"deno.json", "deno.jsonc"} {
		if a.HasFile(name) {
			var cfg DenoConfig
			if err := a.ReadJSON(name, &cfg); err != nil {
				return nil, name, err
			}
			return &cfg, name, nil
		}
	}
	return nil, "", nil
}

// hasDenoManifest reports whether the project has a deno.json or deno.jsonc.
// Used by Detect() before parsing.
func hasDenoManifest(a *app.App) bool {
	return a.HasFile("deno.json") || a.HasFile("deno.jsonc")
}

// importsContain checks whether any import key or value references the given
// substring. Useful for framework detection (Fresh, Hono, etc.).
func (c *DenoConfig) importsContain(needle string) bool {
	if c == nil {
		return false
	}
	for k, v := range c.Imports {
		if containsCI(k, needle) || containsCI(v, needle) {
			return true
		}
	}
	return false
}

// containsCI is a case-insensitive substring check that avoids importing
// strings.Contains+ToLower at every call site.
func containsCI(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	return indexCI(s, sub) >= 0
}

func indexCI(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a := s[i+j]
			b := sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
