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
	"log"
	"os"
	"path/filepath"

	"github.com/usetheo/theopacks/core"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/dockerfile"
	"github.com/usetheo/theopacks/core/dockerignore"
	"github.com/usetheo/theopacks/core/providers/node"
)

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

	if *output == "" {
		log.Fatal("--output is required")
	}

	// Resolve full app directory
	appDir := filepath.Join(*source, *appPath)

	// Check if user already provided a Dockerfile — user-provided takes precedence.
	userDockerfile := filepath.Join(appDir, "Dockerfile")
	if _, err := os.Stat(userDockerfile); err == nil {
		fmt.Fprintf(os.Stderr, "[theopacks] User-provided Dockerfile found at %s — skipping generation\n", userDockerfile)
		content, readErr := os.ReadFile(userDockerfile)
		if readErr != nil {
			log.Fatalf("Failed to read user Dockerfile: %v", readErr)
		}
		if err := os.WriteFile(*output, content, 0644); err != nil {
			log.Fatalf("Failed to copy user Dockerfile to %s: %v", *output, err)
		}
		fmt.Println("--- User-provided Dockerfile ---")
		fmt.Print(string(content))
		return
	}

	// Bridge --app-name and --app-path to env vars so any provider can read
	// them via Environment.GetConfigVariable("APP_NAME"). Originally this
	// only fired for Node workspaces (CHG-002b); extended in this change to
	// fire unconditionally when the flag is non-empty so that Cargo/Ruby/
	// PHP/.NET/Deno workspaces also receive the target. Empty values are NOT
	// bridged because providers treat THEOPACKS_APP_NAME="" as "unspecified"
	// and the usual workspace error path is preferable to a silent miss.
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
		if ws := node.DetectWorkspace(rootApp); ws != nil {
			fmt.Fprintf(os.Stderr,
				"[theopacks] Node workspace detected at %s (type=%v, hasTurbo=%v, members=%d) — analyzing root for app %q at %q\n",
				*source, ws.Type, ws.HasTurbo, len(ws.MemberPaths), *appName, *appPath)
			analyzeDir = *source
		}
	}

	// Initialize the app abstraction from the chosen directory
	a, err := app.NewApp(analyzeDir)
	if err != nil {
		log.Fatalf("[theopacks] Failed to analyze source at %s: %v\n\nMake sure the app path is correct in your theo.yaml.", analyzeDir, err)
	}

	env := app.NewEnvironment(&envVars)
	result := core.GenerateBuildPlan(a, env, &core.GenerateBuildPlanOptions{})

	if !result.Success || result.Plan == nil {
		fmt.Fprintf(os.Stderr, "[theopacks] Could not detect how to build app '%s' at %s\n", *appName, appDir)
		for _, msg := range result.Logs {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", msg.Level, msg.Msg)
		}
		fmt.Fprintln(os.Stderr, "\nTo fix: add a start command to package.json, or use 'build: dockerfile' with your own Dockerfile.")
		os.Exit(1)
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
		log.Fatalf("[theopacks] Failed to generate Dockerfile: %v", err)
	}

	// Write to output path
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.WriteFile(*output, []byte(dockerfileContent), 0644); err != nil {
		log.Fatalf("Failed to write Dockerfile to %s: %v", *output, err)
	}

	// Log the generated Dockerfile to stdout (captured by Loki via Promtail)
	fmt.Printf("--- Generated Dockerfile for %s ---\n", *appName)
	fmt.Print(dockerfileContent)
	fmt.Println("--- End Dockerfile ---")
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

	if _, err := os.Stat(path); err == nil {
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
