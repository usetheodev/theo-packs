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
)

func main() {
	source := flag.String("source", "/workspace", "Root directory of the cloned source code")
	appPath := flag.String("app-path", ".", "Relative path to the app within the source (e.g., apps/api)")
	appName := flag.String("app-name", "app", "Name of the app (used for log context)")
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

	// Initialize the app abstraction from the app directory
	a, err := app.NewApp(appDir)
	if err != nil {
		log.Fatalf("[theopacks] Failed to analyze source at %s: %v\n\nMake sure the app path is correct in your theo.yaml.", appDir, err)
	}

	// Generate build plan
	envVars := map[string]string{}
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
