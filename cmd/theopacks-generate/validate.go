// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package main

import (
	"fmt"
	"regexp"
)

// pathPattern accepts segments allowed in a relative project path:
// alphanumerics, dot, slash, at-sign, hyphen, underscore. Notably does NOT
// accept whitespace, shell metacharacters (; & | $ < > ` * ? ! () {}), or
// arbitrary unicode — those would either be invalid POSIX paths or open
// shell-injection vectors when the path is interpolated into a Dockerfile
// CMD or RUN line. `..` is allowed *textually*; the actual escape-the-root
// check is handled by clampPath (T1.2), keeping concerns separated.
var pathPattern = regexp.MustCompile(`^[A-Za-z0-9._/@-]+$`)

// namePattern accepts identifiers used as workspace member names — npm
// scoped (`@scope/pkg`), Cargo crates, Gradle subprojects, etc. Same
// allowlist as pathPattern but must start with a letter, digit, underscore
// or `@` (no leading `.` or `/` or `-`).
var namePattern = regexp.MustCompile(`^[@A-Za-z0-9_][A-Za-z0-9._@/-]*$`)

// validateCLIInput enforces a defensive allowlist on every user-controlled
// flag before any of them reach the rest of the program. Caller upstream
// (the Theo product) may or may not sanitize; this binary refuses to trust
// it. Returns nil only when all fields are well-formed.
//
// Empty `appName` is allowed (the CLI default — single-app projects).
// Empty `appPath` becomes "." downstream and is rejected here because that
// transformation should be explicit in the caller, not silent.
func validateCLIInput(source, appPath, appName, output string) error {
	if source == "" {
		return fmt.Errorf("--source: cannot be empty")
	}
	if !pathPattern.MatchString(source) {
		return fmt.Errorf("--source: %q contains disallowed characters (allow: A-Za-z0-9._/@-)", source)
	}

	if appPath == "" {
		return fmt.Errorf("--app-path: cannot be empty (use \".\" for the source root)")
	}
	if !pathPattern.MatchString(appPath) {
		return fmt.Errorf("--app-path: %q contains disallowed characters (allow: A-Za-z0-9._/@-)", appPath)
	}

	if appName != "" && !namePattern.MatchString(appName) {
		return fmt.Errorf("--app-name: %q contains disallowed characters (allow: A-Za-z0-9._@/- starting with letter/digit/underscore/@)", appName)
	}

	if output == "" {
		return fmt.Errorf("--output: is required")
	}
	if !pathPattern.MatchString(output) {
		return fmt.Errorf("--output: %q contains disallowed characters (allow: A-Za-z0-9._/@-)", output)
	}

	return nil
}
