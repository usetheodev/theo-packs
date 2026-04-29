// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package dotnet

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
)

// SolutionEntry describes one project listed inside a .sln solution file.
type SolutionEntry struct {
	Name string // project name (e.g., "MyApi")
	Path string // relative path normalized to forward slashes (e.g., "src/MyApi/MyApi.csproj")
}

// Visual Studio solution file format. Sample:
//   Project("{FAE04EC0-301F-11D3-BF4B-00C04F79EFBC}") = "MyApi", "src\MyApi\MyApi.csproj", "{B2A7...}"
var solutionLineRe = regexp.MustCompile(`(?m)^Project\("\{[^}]+\}"\)\s*=\s*"([^"]+)"\s*,\s*"([^"]+)"\s*,\s*"\{[^}]+\}"`)

// parseSolution parses a .sln file and returns its project entries. Only
// project paths ending in a recognized extension (.csproj, .fsproj, .vbproj)
// are returned — solution folders use a different GUID and we filter them out
// implicitly by extension.
func parseSolution(a *app.App, path string) ([]SolutionEntry, error) {
	content, err := a.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var entries []SolutionEntry
	for _, m := range solutionLineRe.FindAllStringSubmatch(content, -1) {
		name := strings.TrimSpace(m[1])
		// Solution files always use Windows-style backslashes for project
		// paths, regardless of the host OS. filepath.ToSlash is a no-op on
		// Linux (where '/' is already the separator), so we replace
		// explicitly.
		projPath := strings.ReplaceAll(strings.TrimSpace(m[2]), `\`, "/")
		ext := strings.ToLower(filepath.Ext(projPath))
		switch ext {
		case ".csproj", ".fsproj", ".vbproj":
			entries = append(entries, SolutionEntry{Name: name, Path: projPath})
		}
	}
	return entries, nil
}

// findProjectFiles returns every project file in the given directory tree,
// sorted alphabetically for stable behavior.
func findProjectFiles(a *app.App) []string {
	var out []string
	for _, ext := range []string{"csproj", "fsproj", "vbproj"} {
		matches, err := a.FindFiles("**/*." + ext)
		if err == nil {
			out = append(out, matches...)
		}
	}
	return out
}

// findSolutionFile returns the first .sln in the project root, or "" if none.
func findSolutionFile(a *app.App) string {
	matches, err := a.FindFiles("*.sln")
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[0]
}
