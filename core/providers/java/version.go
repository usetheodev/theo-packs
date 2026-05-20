// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package java

import (
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
)

// detectJavaVersion picks the Java major version, in priority order:
//  1. Config packages (theopacks.json / THEOPACKS_PACKAGES) — highest.
//  2. THEOPACKS_JAVA_VERSION env var.
//  3. .java-version file (jenv-style, single line).
//  4. gradle.properties (javaVersion= or org.gradle.java.home hint).
//  5. build.gradle / build.gradle.kts toolchain languageVersion.
//  6. pom.xml <maven.compiler.target>, <maven.compiler.release>, or <java.version>.
//  7. DefaultJavaVersion (LTS 21).
//
// The returned source string is used for build logs.
func detectJavaVersion(ctx *generate.GenerateContext) (version string, source string) {
	if pkg := ctx.Resolver.Get("java"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajor(pkg.Version), pkg.Source
	}

	if envVersion, varName := ctx.Env.GetConfigVariable("JAVA_VERSION"); envVersion != "" {
		return generate.NormalizeToMajor(envVersion), varName
	}

	if ctx.App.HasFile(".java-version") {
		if content, err := ctx.App.ReadFile(".java-version"); err == nil {
			if v := strings.TrimSpace(content); v != "" {
				return generate.NormalizeToMajor(v), ".java-version"
			}
		}
	}

	if ctx.App.HasFile("gradle.properties") {
		if content, err := ctx.App.ReadFile("gradle.properties"); err == nil {
			if v := extractGradleProperty(content, "javaVersion"); v != "" {
				return generate.NormalizeToMajor(v), "gradle.properties"
			}
		}
	}

	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		if ctx.App.HasFile(name) {
			if content, err := ctx.App.ReadFile(name); err == nil {
				if v := extractGradleToolchain(content); v != "" {
					return v, name
				}
			}
		}
	}

	if ctx.App.HasFile("pom.xml") {
		if content, err := ctx.App.ReadFile("pom.xml"); err == nil {
			if v := extractMavenJavaVersion(content); v != "" {
				return generate.NormalizeToMajor(v), "pom.xml"
			}
		}
	}

	return generate.DefaultJavaVersion, "default"
}

var (
	gradleToolchainRe = regexp.MustCompile(`languageVersion\s*=?\s*JavaLanguageVersion\.of\(\s*(\d+)\s*\)`)
	mavenTargetRe     = regexp.MustCompile(`<(?:maven\.compiler\.target|maven\.compiler\.release|java\.version)>\s*([\w.-]+)\s*</`)
)

// extractGradleProperty pulls a `key=value` style property from gradle.properties.
func extractGradleProperty(content, key string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		if k == key {
			return strings.TrimSpace(line[eq+1:])
		}
	}
	return ""
}

// extractGradleToolchain finds `JavaLanguageVersion.of(NN)` in either Gradle
// Kotlin DSL or Groovy DSL toolchain blocks. Returns "" if not found.
func extractGradleToolchain(content string) string {
	m := gradleToolchainRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return m[1]
}

// extractMavenJavaVersion reads the first occurrence of <maven.compiler.target>,
// <maven.compiler.release>, or <java.version> from pom.xml. Returns "" if none.
func extractMavenJavaVersion(content string) string {
	m := mavenTargetRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return m[1]
}
