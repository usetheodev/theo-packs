// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package ruby

import (
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
)

// Framework distinguishes deployment shapes. Detection drives the start
// command and (in the future) any framework-specific build steps.
type Framework int

const (
	FrameworkUnknown Framework = iota
	FrameworkRails
	FrameworkSinatra
	FrameworkRack
)

// procfileWebRe matches the `web:` line in a Procfile. The captured group is
// the raw command (everything after the colon, leading whitespace trimmed).
var procfileWebRe = regexp.MustCompile(`(?m)^web:\s*(.+)$`)

// detectFramework picks the runtime framework based on Gemfile contents and
// auxiliary files. Rails wins over Sinatra wins over plain Rack.
func detectFramework(a *app.App) Framework {
	if !a.HasFile("Gemfile") {
		return FrameworkUnknown
	}
	gemfile, err := a.ReadFile("Gemfile")
	if err != nil {
		return FrameworkUnknown
	}

	if hasGem(gemfile, "rails") && a.HasFile("config/application.rb") {
		return FrameworkRails
	}
	if hasGem(gemfile, "sinatra") {
		return FrameworkSinatra
	}
	if a.HasFile("config.ru") {
		return FrameworkRack
	}
	return FrameworkUnknown
}

// procfileWebCommand returns the value of the `web:` line from a Procfile,
// or "" when there's no Procfile / no web entry.
func procfileWebCommand(a *app.App) string {
	if !a.HasFile("Procfile") {
		return ""
	}
	content, err := a.ReadFile("Procfile")
	if err != nil {
		return ""
	}
	m := procfileWebRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// frameworkStartCommand returns the canonical start command for each
// framework. The Procfile takes precedence when present.
func frameworkStartCommand(a *app.App, fw Framework) string {
	if cmd := procfileWebCommand(a); cmd != "" {
		return cmd
	}
	switch fw {
	case FrameworkRails:
		return "bundle exec rails server -b 0.0.0.0 -p ${PORT:-3000} -e production"
	case FrameworkSinatra, FrameworkRack:
		return "bundle exec rackup -p ${PORT:-4567} -o 0.0.0.0"
	}
	return ""
}
