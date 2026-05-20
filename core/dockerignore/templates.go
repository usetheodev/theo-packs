// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

// Package dockerignore provides per-language default .dockerignore templates
// written by the CLI when the user has not supplied one. Excludes are tuned
// to drop language-specific build outputs (node_modules/, target/, etc.),
// tooling caches, OS noise, and version control state from the Docker build
// context — the most common cause of multi-GB context transfers in PaaS
// pipelines.
//
// The package is read-only: it returns templates as strings. Writing the file
// to disk is a CLI concern (cmd/theopacks-generate/main.go).
package dockerignore

// baseCommon is appended to every per-language template. Covers VCS state,
// editor cruft, OS noise, and the theo-packs-managed files that should never
// land in the build context.
const baseCommon = `# Common — version control, OS noise, editor cruft.
.git/
.gitignore
.svn/
.hg/
.DS_Store
Thumbs.db
*.swp
*~
.idea/
.vscode/

# theo-packs internals
theopacks.json
.dockerignore
`

const goAdditions = `
# Go
*.test
*.out
vendor/
`

const nodeAdditions = `
# Node.js
node_modules/
npm-debug.log*
yarn-debug.log*
yarn-error.log*
.next/cache/
.turbo/
dist/
build/
coverage/
.nyc_output/
`

const pythonAdditions = `
# Python
__pycache__/
*.pyc
*.pyo
*.pyd
*.egg-info/
.pytest_cache/
.mypy_cache/
.ruff_cache/
.tox/
.coverage
htmlcov/
.venv/
venv/
ENV/
env/
.env
.env.*
!.env.example
build/
dist/
`

const rustAdditions = `
# Rust
target/
Cargo.lock.bak
`

const javaAdditions = `
# Java
target/
build/
.gradle/
.classpath
.project
.settings/
*.class
*.jar
*.war
hs_err_pid*
`

const dotnetAdditions = `
# .NET
bin/
obj/
*.user
*.suo
.vs/
publish/
`

const rubyAdditions = `
# Ruby
.bundle/
vendor/bundle/
log/
tmp/
*.gem
.byebug_history
`

const phpAdditions = `
# PHP
vendor/
.phpunit.result.cache
.phpcs-cache
`

const denoAdditions = `
# Deno
.deno/
deno.lock.bak
`

// templates maps a Provider.Name() to the language-specific block appended to
// baseCommon. Unknown / empty names get baseCommon alone — still useful for
// excluding .git and OS noise.
var templates = map[string]string{
	"go":         goAdditions,
	"node":       nodeAdditions,
	"python":     pythonAdditions,
	"rust":       rustAdditions,
	"java":       javaAdditions,
	"dotnet":     dotnetAdditions,
	"ruby":       rubyAdditions,
	"php":        phpAdditions,
	"deno":       denoAdditions,
	"staticfile": "",
	"shell":      "",
}

// DefaultFor returns the recommended .dockerignore content for the given
// provider name. An empty or unknown name returns the language-agnostic
// baseline (still drops .git and OS noise). The output ends with a single
// trailing newline.
func DefaultFor(providerName string) string {
	addition := templates[providerName]
	return baseCommon + addition
}
