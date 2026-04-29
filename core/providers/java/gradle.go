// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package java

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

var (
	// Spring Boot plugin signatures across both Gradle DSLs.
	gradleKtsSpringBootRe    = regexp.MustCompile(`id\(\s*"org\.springframework\.boot"\s*\)`)
	gradleGroovySpringBootRe = regexp.MustCompile(`(?:apply\s+plugin:|id)\s+['"]org\.springframework\.boot['"]`)

	// Matches every Gradle subproject coordinate (":apps:api", ":lib"). All
	// Gradle paths start with ':' which uniquely distinguishes them from any
	// other quoted strings in settings.gradle (rootProject.name, etc.).
	gradleIncludeRe = regexp.MustCompile(`['"]:([^'"]+)['"]`)
)

// gradleHasSpringBoot reports whether the project applies the Spring Boot
// plugin at the root build script level.
func gradleHasSpringBoot(a *app.App) bool {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		if !a.HasFile(name) {
			continue
		}
		content, err := a.ReadFile(name)
		if err != nil {
			continue
		}
		if strings.HasSuffix(name, ".kts") {
			if gradleKtsSpringBootRe.MatchString(content) {
				return true
			}
		} else {
			if gradleGroovySpringBootRe.MatchString(content) {
				return true
			}
		}
	}
	return false
}

// planGradle generates the build plan for a Gradle-based Java project (single
// project — workspace flow is in workspace.go).
func planGradle(ctx *generate.GenerateContext, version string) error {
	hasSpring := gradleHasSpringBoot(ctx.App)
	buildCmd := "gradle build --no-daemon -x test"
	if hasSpring {
		buildCmd = "gradle bootJar --no-daemon -x test"
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.GradleImageForJavaVersion(version)))
	installStep.AddCacheMount("/root/.gradle", "")
	for _, manifest := range []string{"build.gradle.kts", "build.gradle", "settings.gradle.kts", "settings.gradle", "gradle.properties"} {
		if ctx.App.HasFile(manifest) {
			installStep.AddCommand(plan.NewCopyCommand(manifest, "./"))
		}
	}
	if ctx.App.HasFile("gradle") {
		installStep.AddCommand(plan.NewCopyCommand("gradle", "./gradle"))
	}
	for _, wrapper := range []string{"gradlew", "gradlew.bat"} {
		if ctx.App.HasFile(wrapper) {
			installStep.AddCommand(plan.NewCopyCommand(wrapper, "./"))
		}
	}

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/root/.gradle", "")
	buildStep.AddCommand(plan.NewExecShellCommand(buildCmd))
	// Renderer adds `sh -c '...'` once based on CommandKindShell. Pre-wrapping
	// here used to produce `RUN sh -c 'sh -c '...''` which broke quoting. Pass
	// the bare body.
	buildStep.AddCommand(plan.NewExecShellCommand(
		"set -e; jar=$(ls build/libs/*.jar | grep -v -- \"-plain\\.jar$\" | head -n1); cp \"$jar\" /app/app.jar",
	))

	configureGradleDeploy(ctx, version)
	return nil
}

func configureGradleDeploy(ctx *generate.GenerateContext, version string) {
	ctx.Deploy.Base = plan.NewImageLayer(generate.JavaJreImageForVersion(version))
	ctx.Deploy.StartCmd = "java -jar /app/app.jar"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/app.jar"}}),
	})
}

// gradleSubprojects parses settings.gradle / settings.gradle.kts and returns
// the include() target paths in colon-separated form (":apps:api" → "apps/api").
// Filtered to entries that exist on disk and have a build.gradle*.
func gradleSubprojects(a *app.App) []string {
	settings := ""
	for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
		if a.HasFile(name) {
			content, err := a.ReadFile(name)
			if err == nil {
				settings = content
			}
			break
		}
	}
	if settings == "" {
		return nil
	}

	seen := make(map[string]bool)
	var out []string
	for _, m := range gradleIncludeRe.FindAllStringSubmatch(settings, -1) {
		// Each m[1] is the path *after* the leading colon (e.g. "apps:api").
		dir := strings.ReplaceAll(m[1], ":", "/")
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		if a.HasFile(dir+"/build.gradle.kts") || a.HasFile(dir+"/build.gradle") {
			out = append(out, dir)
		}
	}
	return out
}

// planGradleWorkspace builds a single subproject targeted by THEOPACKS_APP_NAME.
func planGradleWorkspace(ctx *generate.GenerateContext, version string, subprojects []string) error {
	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")

	target, ok := selectGradleSubproject(subprojects, appName)
	if !ok {
		if appName == "" {
			return fmt.Errorf("gradle workspace has multiple subprojects; set THEOPACKS_APP_NAME to one of: %s", strings.Join(subprojectNames(subprojects), ", "))
		}
		return fmt.Errorf("gradle workspace has no subproject named %q; available: %s", appName, strings.Join(subprojectNames(subprojects), ", "))
	}

	hasSpring := gradleHasSpringBoot(ctx.App)
	gradleTarget := ":" + strings.ReplaceAll(target, "/", ":")
	buildCmd := fmt.Sprintf("gradle %s:build --no-daemon -x test", gradleTarget)
	if hasSpring {
		buildCmd = fmt.Sprintf("gradle %s:bootJar --no-daemon -x test", gradleTarget)
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.GradleImageForJavaVersion(version)))
	installStep.AddCacheMount("/root/.gradle", "")
	for _, manifest := range []string{"build.gradle.kts", "build.gradle", "settings.gradle.kts", "settings.gradle", "gradle.properties"} {
		if ctx.App.HasFile(manifest) {
			installStep.AddCommand(plan.NewCopyCommand(manifest, "./"))
		}
	}

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/root/.gradle", "")
	buildStep.AddCommand(plan.NewExecShellCommand(buildCmd))
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("set -e; jar=$(ls %s/build/libs/*.jar | grep -v -- \"-plain\\.jar$\" | head -n1); cp \"$jar\" /app/app.jar", target),
	))

	configureGradleDeploy(ctx, version)
	return nil
}

func selectGradleSubproject(subprojects []string, appName string) (string, bool) {
	if appName != "" {
		for _, s := range subprojects {
			if subprojectShortName(s) == appName {
				return s, true
			}
		}
		return "", false
	}
	if len(subprojects) == 1 {
		return subprojects[0], true
	}
	return "", false
}

// subprojectShortName takes "apps/api" → "api", matching the convention where
// THEOPACKS_APP_NAME is the leaf directory name.
func subprojectShortName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func subprojectNames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = subprojectShortName(p)
	}
	return out
}
