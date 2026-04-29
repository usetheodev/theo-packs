// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package php

import (
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
)

// Framework distinguishes the deployable PHP shape and drives the start command.
type Framework int

const (
	FrameworkUnknown Framework = iota
	FrameworkLaravel
	FrameworkSymfony
	FrameworkSlim
	FrameworkGeneric
)

var phpProcfileWebRe = regexp.MustCompile(`(?m)^web:\s*(.+)$`)

// detectFramework picks the PHP framework based on composer require + presence
// of conventional entry-point files. Order matters because Laravel includes
// many sub-packages — we anchor on the framework metapackage first.
func detectFramework(a *app.App, c *ComposerJson) Framework {
	if c == nil {
		return FrameworkUnknown
	}
	if c.HasPackage("laravel/framework") && a.HasFile("artisan") {
		return FrameworkLaravel
	}
	if c.HasPackage("symfony/framework-bundle") {
		return FrameworkSymfony
	}
	if c.HasPackage("slim/slim") {
		return FrameworkSlim
	}
	return FrameworkGeneric
}

// procfileWebCommand returns the value of the Procfile `web:` line or "" if
// no such file/line exists.
func procfileWebCommand(a *app.App) string {
	if !a.HasFile("Procfile") {
		return ""
	}
	content, err := a.ReadFile("Procfile")
	if err != nil {
		return ""
	}
	m := phpProcfileWebRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// frameworkStartCommand returns the canonical start command per framework,
// with Procfile `web:` taking precedence.
func frameworkStartCommand(a *app.App, fw Framework) string {
	if cmd := procfileWebCommand(a); cmd != "" {
		return cmd
	}
	switch fw {
	case FrameworkLaravel:
		return "php artisan serve --host=0.0.0.0 --port=${PORT:-8000}"
	case FrameworkSymfony:
		// Symfony uses its console; "server:run" was deprecated. Most users
		// proxy via Apache/Nginx in production — the CLI fallback below is
		// fine for default zero-config scaffolds.
		return "php -S 0.0.0.0:${PORT:-8000} -t public"
	case FrameworkSlim:
		return "php -S 0.0.0.0:${PORT:-8000} -t public"
	case FrameworkGeneric:
		// public/ is the de facto convention; falling back to the project
		// root keeps zero-config working for tiny apps.
		if a.HasFile("public/index.php") {
			return "php -S 0.0.0.0:${PORT:-8000} -t public"
		}
		return "php -S 0.0.0.0:${PORT:-8000}"
	}
	return ""
}
