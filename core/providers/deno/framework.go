// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package deno

import (
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
)

type Framework int

const (
	FrameworkUnknown Framework = iota
	FrameworkFresh
	FrameworkHono
	FrameworkGeneric
)

var denoProcfileWebRe = regexp.MustCompile(`(?m)^web:\s*(.+)$`)

// detectFramework picks the Deno framework based on import map entries.
// Deno doesn't have a single canonical "framework field" — projects declare
// dependencies via import specifiers, so substring matching against well-
// known package names is the practical signal.
func detectFramework(c *DenoConfig) Framework {
	if c == nil {
		return FrameworkUnknown
	}
	if c.importsContain("fresh") {
		return FrameworkFresh
	}
	if c.importsContain("hono") {
		return FrameworkHono
	}
	if len(c.Imports) > 0 || len(c.Tasks) > 0 {
		return FrameworkGeneric
	}
	return FrameworkUnknown
}

func procfileWebCommand(a *app.App) string {
	if !a.HasFile("Procfile") {
		return ""
	}
	content, err := a.ReadFile("Procfile")
	if err != nil {
		return ""
	}
	m := denoProcfileWebRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// frameworkStartCommand resolves the start command using this priority:
//  1. Procfile `web:` line.
//  2. deno.json `tasks.start` (preferred — user-defined intent).
//  3. Framework defaults.
//  4. Fall back to deno run -A main.ts when main.ts exists.
func frameworkStartCommand(a *app.App, c *DenoConfig, fw Framework) string {
	if cmd := procfileWebCommand(a); cmd != "" {
		return cmd
	}
	if c != nil {
		if _, ok := c.Tasks["start"]; ok {
			return "deno task start"
		}
	}
	switch fw {
	case FrameworkFresh:
		// Fresh's scaffolded projects always ship a "start" task; if missing
		// we fall through to the explicit invocation here.
		return "deno run -A --watch=static/,routes/ main.ts"
	case FrameworkHono:
		return "deno run -A --no-check main.ts"
	}
	if a.HasFile("main.ts") {
		return "deno run -A main.ts"
	}
	if a.HasFile("main.js") {
		return "deno run -A main.js"
	}
	return ""
}

// hasMainEntry reports whether a recognized entry file (main.ts/main.js)
// exists. Used by the install step to scope `deno cache` warmup.
func hasMainEntry(a *app.App) (string, bool) {
	for _, name := range []string{"main.ts", "main.js", "mod.ts", "mod.js"} {
		if a.HasFile(name) {
			return name, true
		}
	}
	return "", false
}
