// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

// Package java implements language detection and Dockerfile build planning
// for Java projects. Both Gradle (Kotlin DSL and Groovy) and Maven are
// supported, with single-module and multi-module/subproject layouts.
//
// Spring Boot is detected automatically (Gradle plugin or Maven starter
// dependency) and routes the build to bootJar / fat-JAR layout. Generic Java
// (no Spring) still produces a runnable JAR via plain `gradle build` /
// `mvn package` and uses the first non-plain jar from build/libs / target.
//
// Build image: gradle:8-jdk<v> or maven:3-eclipse-temurin-<v>.
// Runtime image: eclipse-temurin:<v>-jre (smaller than JDK).
package java

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type JavaProvider struct{}

func (p *JavaProvider) Name() string {
	return "java"
}

func (p *JavaProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("build.gradle.kts") ||
		ctx.App.HasFile("build.gradle") ||
		ctx.App.HasFile("pom.xml"), nil
}

func (p *JavaProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

// Plan dispatches to Gradle or Maven flows. When both manifests are present
// (rare but possible in migration projects), Gradle wins because new Java
// projects in 2026 default to Gradle and the user's intent is more likely
// reflected there.
func (p *JavaProvider) Plan(ctx *generate.GenerateContext) error {
	version, source := detectJavaVersion(ctx)
	ref := ctx.Resolver.Default("java", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	hasGradle := ctx.App.HasFile("build.gradle.kts") || ctx.App.HasFile("build.gradle")
	hasMaven := ctx.App.HasFile("pom.xml")

	if hasGradle {
		if hasMaven {
			ctx.Logger.LogWarn("Detected both Gradle and Maven manifests; building with Gradle (override via theopacks.json provider field)")
		}
		if subprojects := gradleSubprojects(ctx.App); len(subprojects) > 0 {
			return planGradleWorkspace(ctx, version, subprojects)
		}
		return planGradle(ctx, version)
	}

	if hasMaven {
		if modules := mavenModules(ctx.App); len(modules) > 0 {
			return planMavenWorkspace(ctx, version, modules)
		}
		return planMaven(ctx, version)
	}

	// Should never happen because Detect already screened for these manifests,
	// but Plan is the public contract — be explicit.
	return nil
}

func (p *JavaProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *JavaProvider) StartCommandHelp() string {
	return "Java apps must produce a runnable JAR (Spring Boot bootJar or a self-executing artifact). For multi-module/subproject builds, set THEOPACKS_APP_NAME to the module/subproject directory leaf name."
}
