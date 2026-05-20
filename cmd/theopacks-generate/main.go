// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

// theopacks-generate analyzes source code and generates an optimized Dockerfile.
// Runs inside an Argo Workflow step in the build cluster.
//
// Usage:
//
//	theopacks-generate \
//	  --source /workspace \
//	  --app-path apps/api \
//	  --app-name api \
//	  --output /workspace/Dockerfile.api
//
// CHG-002b 2026-04-28 — workspace-aware build:
//
// If the project root (source) is a Node workspace monorepo (turbo.json,
// pnpm-workspace.yaml, or package.json#workspaces), theopacks-generate
// analyzes the WORKSPACE ROOT instead of the per-app subdirectory and
// emits a Dockerfile that:
//   - Installs deps once at the workspace root (lockfile + manifests)
//   - Copies the full workspace and runs the build with proper filtering
//     (turbo run build --filter=<app>... / pnpm --filter <app>... build /
//     npm run build --workspaces --if-present)
//   - In the runtime stage, copies only apps/<app>/dist + node_modules
//
// This unblocks cross-package TypeScript imports (the original failure
// mode that caused L3-realistic to fail at `tsc` with "Cannot find
// module '@dogfood/shared-utils'").
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/dockerfile"
	"github.com/usetheo/theopacks/core/dockerignore"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/providers/node"
)

// Exit codes documented in docs/contracts/theo-packs-cli-contract.md.
// Callers (Theo product) branch on these to decide retry policy:
//   - exitSuccess (0): build plan + Dockerfile written.
//   - exitGenericFailure (1): provider detection failed, write failed,
//     or some other transient/operational error. Caller MAY retry.
//   - exitInputInvariant (2): bad flag, traversal, symlink, or user
//     Dockerfile present. Caller MUST NOT retry without changing input.
const (
	exitSuccess        = 0
	exitGenericFailure = 1
	exitInputInvariant = 2
)

// fatal prints msg to stderr with the [theopacks] prefix and exits with
// the given code. Replaces ad-hoc log.Fatal* / os.Exit pairs scattered
// through main(). Centralized so adding telemetry later (e.g. a metric
// per exit code) is a one-edit change.
func fatal(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[theopacks] "+format+"\n", args...)
	os.Exit(code)
}

func main() {
	source := flag.String("source", "/workspace", "Root directory of the cloned source code")
	appPath := flag.String("app-path", ".", "Relative path to the app within the source (e.g., apps/api)")
	// app-name has no default. When unset, providers that need a workspace
	// target (Cargo workspaces, Ruby/PHP apps/+packages, Gradle subprojects,
	// .NET solutions, Deno workspaces) will surface their usual "set
	// THEOPACKS_APP_NAME to one of: ..." error. The previous default of
	// "app" caused those providers to look for a literal app named "app".
	appName := flag.String("app-name", "", "Name of the app (selects target in monorepos; required for multi-app workspaces)")
	output := flag.String("output", "", "Output path for the generated Dockerfile (required)")
	flag.Parse()

	// Defense-in-depth sanitization (T1.1 — deep-review-hardening-plan).
	// Refuse to trust the caller. Every user-facing flag must match a
	// restrictive allowlist before it reaches path resolution, env-var
	// bridging, or shell-interpolated provider commands. Exit code 2
	// signals "input invariant violated" — distinguishable by callers
	// from a generic failure (exit 1).
	if err := validateCLIInput(*source, *appPath, *appName, *output); err != nil {
		fatal(exitInputInvariant, "%s", err)
	}

	// Resolve full app directory with traversal guard (T1.2). filepath.Join
	// alone does NOT prevent ../ escape; clampPath enforces the source-root
	// invariant explicitly.
	appDir, err := clampPath(*source, *appPath)
	if err != nil {
		fatal(exitInputInvariant, "--app-path: %s", err)
	}

	// Single source of truth: theo-packs generates the Dockerfile. A user-
	// supplied Dockerfile in the analyzed app directory is a contract
	// violation — there would be two sources of truth for the build
	// artifact, defeating the determinism theo-packs guarantees. Hard fail
	// with exit code 2 (input invariant violated, distinguishable from
	// generic failure code 1).
	//
	// A Dockerfile at the workspace ROOT (outside the analyzed app path)
	// is NOT checked — it may legitimately exist for local development
	// outside Theo (e.g., `docker compose up`). We only reject within the
	// app path being analyzed.
	//
	// See docs/contracts/theo-packs-cli-contract.md, "Single source of
	// truth" preamble, for the full rationale.
	userDockerfile := filepath.Join(appDir, "Dockerfile")
	// T1.3 — Lstat (does NOT follow symlinks) is required because the
	// binary runs as a privileged step in a multi-tenant Argo cluster. A
	// symlink Dockerfile → /etc/* would leak existence (via the
	// "found at <path>" branch) or worse if combined with future
	// behavior changes. Refuse symlinks outright.
	if info, err := os.Lstat(userDockerfile); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			fatal(exitInputInvariant,
				"ERROR: %s is a symbolic link — refusing to follow.",
				userDockerfile)
		}
		if info.Mode().IsRegular() {
			fatal(exitInputInvariant,
				"ERROR: user-supplied Dockerfile found at %s.\n\n"+
					"theo-packs is the single source of truth for Dockerfile generation.\n"+
					"Remove the file and rerun. To opt out of generation entirely, do not\n"+
					"invoke theo-packs — declare your build via a different mechanism in\n"+
					"your deployment pipeline.",
				userDockerfile)
		}
	}

	// T3.1 — pass --app-name and --app-path to providers via the typed
	// WorkspaceTarget on GenerateBuildPlanOptions. The legacy env-var
	// bridge (THEOPACKS_APP_NAME / THEOPACKS_APP_PATH) is still emitted in
	// parallel so:
	//   - Direct library consumers that haven't migrated keep working.
	//   - Provider code reading via ResolveAppName/Path picks the typed
	//     value first (it's authoritative).
	// Empty values are NOT bridged: providers treat unspecified target as
	// "no specific workspace member", and an explicit empty would override
	// THEOPACKS_APP_NAME if the user set it elsewhere.
	envVars := map[string]string{}
	if *appName != "" {
		envVars["THEOPACKS_APP_NAME"] = *appName
	}
	if *appPath != "" && *appPath != "." {
		envVars["THEOPACKS_APP_PATH"] = *appPath
	}

	// CHG-002b: if the source root is a Node workspace monorepo, analyze the
	// ROOT (not the per-app subdir) so the Node provider can detect
	// cross-package dependencies and emit a workspace-aware build command
	// (e.g. turbo --filter). For non-Node workspaces, providers handle the
	// workspace shape themselves from the per-app subdir, so we only redirect
	// analyzeDir for the Node case.
	rootApp, rootErr := app.NewApp(*source)
	analyzeDir := appDir
	if rootErr == nil {
		switch {
		case node.DetectWorkspace(rootApp, logger.Nop()) != nil:
			ws := node.DetectWorkspace(rootApp, logger.Nop())
			fmt.Fprintf(os.Stderr,
				"[theopacks] Node workspace detected at %s (type=%v, hasTurbo=%v, members=%d) — analyzing root for app %q at %q\n",
				*source, ws.Type, ws.HasTurbo, len(ws.MemberPaths), *appName, *appPath)
			analyzeDir = *source
		case isGenericMonorepoRoot(*source, *appPath):
			// theo-stacks parity: Ruby/PHP/Python/Java/Rust monorepos
			// ship the language manifest at the ROOT (Gemfile,
			// composer.json, pyproject.toml, build.gradle, Cargo.toml)
			// with apps/<name>/ subdirs that DON'T have their own
			// manifests. Analyze the root so the provider can detect
			// the language; pass appPath via WorkspaceTarget so the
			// provider scopes the build to the chosen app.
			fmt.Fprintf(os.Stderr,
				"[theopacks] Generic monorepo root detected at %s — analyzing root for app %q at %q\n",
				*source, *appName, *appPath)
			analyzeDir = *source
		}
	}

	// Initialize the app abstraction from the chosen directory
	a, err := app.NewApp(analyzeDir)
	if err != nil {
		fatal(exitGenericFailure,
			"Failed to analyze source at %s: %v\n\nMake sure the app path is correct in your theo.yaml.",
			analyzeDir, err)
	}

	env := app.NewEnvironment(&envVars)
	opts := &core.GenerateBuildPlanOptions{}
	if *appName != "" || (*appPath != "" && *appPath != ".") {
		opts.WorkspaceTarget = &core.WorkspaceTarget{
			AppName: *appName,
			AppPath: normalizeAppPath(*appPath),
		}
	}
	result := core.GenerateBuildPlan(a, env, opts)

	if !result.Success || result.Plan == nil {
		fmt.Fprintf(os.Stderr, "[theopacks] Could not detect how to build app '%s' at %s\n", *appName, appDir)
		for _, msg := range result.Logs {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", msg.Level, msg.Msg)
		}
		fmt.Fprintln(os.Stderr, "\nTo fix: add a start command to package.json, or use 'build: dockerfile' with your own Dockerfile.")
		os.Exit(exitGenericFailure)
	}

	// Log detected providers
	fmt.Fprintf(os.Stderr, "[theopacks] Detected: %v\n", result.DetectedProviders)
	if meta, ok := result.Metadata["startCommand"]; ok {
		fmt.Fprintf(os.Stderr, "[theopacks] Start command: %s\n", meta)
	}

	// Write a default .dockerignore tailored to the detected provider IF the
	// user has not supplied one. User-provided files are NEVER overwritten
	// (D3 in build-correctness-and-speed-v2 plan). Failures are logged but
	// don't abort — Dockerfile writing still happens on read-only sources.
	if len(result.DetectedProviders) > 0 {
		writeDefaultDockerignore(analyzeDir, result.DetectedProviders[0])
	}

	// Generate Dockerfile from build plan
	dockerfileContent, err := dockerfile.Generate(result.Plan)
	if err != nil {
		fatal(exitGenericFailure, "Failed to generate Dockerfile: %v", err)
	}

	// Write to output path
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		fatal(exitGenericFailure, "Failed to create output directory: %v", err)
	}
	if err := os.WriteFile(*output, []byte(dockerfileContent), 0644); err != nil {
		fatal(exitGenericFailure, "Failed to write Dockerfile to %s: %v", *output, err)
	}

	// Log the generated Dockerfile to stdout (captured by Loki via Promtail)
	fmt.Printf("--- Generated Dockerfile for %s ---\n", *appName)
	fmt.Print(dockerfileContent)
	fmt.Println("--- End Dockerfile ---")
}

// normalizeAppPath collapses "." (the CLI default for single-app
// projects) into "" so that downstream code can branch on an empty
// AppPath without special-casing the literal dot.
func normalizeAppPath(p string) string {
	if p == "." {
		return ""
	}
	return p
}

// isGenericMonorepoRoot returns true when (a) appPath is non-empty
// and non-".", (b) source root has a language-manifest file that
// signals a single-language monorepo (Ruby/PHP/Python/Java/Rust),
// and (c) the apps/<name>/ subdir doesn't carry its own copy of the
// manifest. theo-stacks shapes its non-Node monorepos this way — the
// root Gemfile/composer.json/pyproject.toml/etc. holds shared deps and
// apps/<name>/ is just the app's source tree.
//
// This mirrors the Node CHG-002b redirect for languages that ship the
// equivalent layout via theo-stacks templates.
func isGenericMonorepoRoot(source, appPath string) bool {
	if appPath == "" || appPath == "." {
		return false
	}
	rootManifests := []string{
		"Gemfile",         // Ruby
		"composer.json",   // PHP
		"pyproject.toml",  // Python (monorepo-python uses root pyproject)
		"Cargo.toml",      // Rust (workspace root)
		"build.gradle",    // Java Gradle
		"build.gradle.kts",
		"settings.gradle",
		"settings.gradle.kts",
		"pom.xml",         // Java Maven (multi-module root)
	}
	rootHasManifest := false
	for _, m := range rootManifests {
		if fileExists(filepath.Join(source, m)) {
			rootHasManifest = true
			break
		}
	}
	if !rootHasManifest {
		return false
	}
	// If the app subdir has its OWN root-equivalent manifest, the
	// provider can detect from there directly — don't redirect.
	for _, m := range rootManifests {
		if fileExists(filepath.Join(source, appPath, m)) {
			return false
		}
	}
	return true
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// writeDefaultDockerignore writes a per-language .dockerignore template to
// dir/.dockerignore when the file does not already exist. User-supplied
// files are NEVER overwritten or merged with the default.
//
// Failures are logged to stderr and the function returns without aborting:
// (a) a read-only source mount in CI is a legitimate scenario where we
// should still produce the Dockerfile, and (b) a missing .dockerignore is a
// performance optimization, not a correctness requirement.
func writeDefaultDockerignore(dir, providerName string) {
	path := filepath.Join(dir, ".dockerignore")

	// T1.3 — Lstat refuses to follow symlinks. A symlink at .dockerignore is
	// treated as "user-provided file present" and skipped, matching the
	// general policy: never read OR overwrite paths the caller may have
	// crafted as symlinks to sensitive files.
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(os.Stderr,
				"[theopacks] .dockerignore at %s is a symlink — skipping default generation\n",
				path)
			return
		}
		fmt.Fprintf(os.Stderr,
			"[theopacks] User-provided .dockerignore found at %s — skipping default generation\n",
			path)
		return
	} else if !os.IsNotExist(err) {
		// Stat failed for a reason other than "file does not exist" (permission
		// denied, IO error). Don't try to write — we may be on a read-only
		// mount. Log and continue.
		fmt.Fprintf(os.Stderr,
			"[theopacks] Could not stat %s (%v) — skipping default .dockerignore generation\n",
			path, err)
		return
	}

	content := dockerignore.DefaultFor(providerName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr,
			"[theopacks] Failed to write default .dockerignore to %s: %v (continuing)\n",
			path, err)
		return
	}
	fmt.Fprintf(os.Stderr,
		"[theopacks] Wrote default .dockerignore for provider %q to %s\n",
		providerName, path)
}
