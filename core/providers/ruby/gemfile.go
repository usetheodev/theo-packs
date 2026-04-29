// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package ruby

import (
	"regexp"
	"strings"
)

// gemNameRe matches a `gem "name"` or `gem 'name'` directive (with optional
// version constraints/options after the name). Comments and group blocks are
// not stripped here — accuracy on real Gemfiles is good enough because the
// directive must start the line (modulo whitespace) and the regex anchors
// on the gem keyword.
var gemNameRe = regexp.MustCompile(`(?m)^\s*gem\s+['"]([^'"]+)['"]`)

// rubyDirectiveRe matches `ruby '3.3'` or `ruby "3.3.0"` at the top of Gemfile.
// Bundler also accepts `ruby file: ".ruby-version"` — that form is ignored
// here on purpose; the version-detection layer falls back to .ruby-version
// when this directive isn't found in literal form.
var rubyDirectiveRe = regexp.MustCompile(`(?m)^\s*ruby\s+['"]([^'"]+)['"]`)

// gemNames extracts the literal gem names declared in a Gemfile. Order is
// preserved; duplicates are removed (rare in practice but harmless).
func gemNames(content string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, m := range gemNameRe.FindAllStringSubmatch(content, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// hasGem reports whether the Gemfile declares the named gem.
func hasGem(content, name string) bool {
	for _, g := range gemNames(content) {
		if g == name {
			return true
		}
	}
	return false
}

// rubyVersionFromGemfile returns the literal Ruby version from a `ruby "X.Y"`
// directive. Empty string means "not declared in Gemfile".
func rubyVersionFromGemfile(content string) string {
	m := rubyDirectiveRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}
