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
	// Spring Boot detection in pom.xml — any spring-boot-starter-* artifact.
	mavenSpringBootRe = regexp.MustCompile(`<artifactId>\s*spring-boot-starter`)

	// <module>apps/api</module> — multi-module entries.
	mavenModulesRe = regexp.MustCompile(`<module>\s*([^<\s]+)\s*</module>`)
)

// mavenHasSpringBoot reports whether pom.xml references a spring-boot-starter.
func mavenHasSpringBoot(a *app.App) bool {
	if !a.HasFile("pom.xml") {
		return false
	}
	content, err := a.ReadFile("pom.xml")
	if err != nil {
		return false
	}
	return mavenSpringBootRe.MatchString(content)
}

// planMaven generates the build plan for a Maven-based Java project.
// Spring Boot detection drives whether we ship a fat-jar start command;
// the build command is the same `mvn package` either way (the spring-boot
// plugin handles repackaging via its lifecycle binding).
func planMaven(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.MavenImageForJavaVersion(version)))
	installStep.AddCommand(plan.NewCopyCommand("pom.xml", "./"))
	for _, wrapper := range []string{"mvnw", "mvnw.cmd", ".mvn"} {
		if ctx.App.HasFile(wrapper) {
			installStep.AddCommand(plan.NewCopyCommand(wrapper, "./"))
		}
	}
	installStep.AddCommand(plan.NewExecShellCommand("mvn -B -DskipTests dependency:go-offline"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCommand(plan.NewExecShellCommand("mvn -B -DskipTests package"))
	buildStep.AddCommand(plan.NewExecShellCommand(
		"sh -c 'set -e; jar=$(ls target/*.jar | grep -v -- \"-sources\\.jar$\\|-javadoc\\.jar$\\|original-\" | head -n1); cp \"$jar\" /app/app.jar'",
	))

	configureMavenDeploy(ctx, version)
	return nil
}

func configureMavenDeploy(ctx *generate.GenerateContext, version string) {
	ctx.Deploy.Base = plan.NewImageLayer(generate.JavaJreImageForVersion(version))
	ctx.Deploy.StartCmd = "java -jar /app/app.jar"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/app.jar"}}),
	})
}

// mavenModules parses pom.xml for <module>...</module> entries (multi-module
// builds). Returns directories that exist and contain pom.xml.
func mavenModules(a *app.App) []string {
	if !a.HasFile("pom.xml") {
		return nil
	}
	content, err := a.ReadFile("pom.xml")
	if err != nil {
		return nil
	}

	var out []string
	for _, m := range mavenModulesRe.FindAllStringSubmatch(content, -1) {
		dir := strings.TrimSpace(m[1])
		if dir == "" {
			continue
		}
		if a.HasFile(dir + "/pom.xml") {
			out = append(out, dir)
		}
	}
	return out
}

// planMavenWorkspace builds a single submodule via `mvn -pl <path> -am package`.
func planMavenWorkspace(ctx *generate.GenerateContext, version string, modules []string) error {
	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	target, ok := selectMavenModule(modules, appName)
	if !ok {
		if appName == "" {
			return fmt.Errorf("maven workspace has multiple modules; set THEOPACKS_APP_NAME to one of: %s", strings.Join(subprojectNames(modules), ", "))
		}
		return fmt.Errorf("maven workspace has no module named %q; available: %s", appName, strings.Join(subprojectNames(modules), ", "))
	}

	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.MavenImageForJavaVersion(version)))
	installStep.AddCommand(plan.NewCopyCommand("pom.xml", "./"))
	for _, mod := range modules {
		installStep.AddCommand(plan.NewCopyCommand(mod+"/pom.xml", mod+"/"))
	}

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("mvn -B -DskipTests -pl %s -am package", target),
	))
	buildStep.AddCommand(plan.NewExecShellCommand(
		fmt.Sprintf("sh -c 'set -e; jar=$(ls %s/target/*.jar | grep -v -- \"-sources\\.jar$\\|-javadoc\\.jar$\\|original-\" | head -n1); cp \"$jar\" /app/app.jar'", target),
	))

	configureMavenDeploy(ctx, version)
	return nil
}

func selectMavenModule(modules []string, appName string) (string, bool) {
	if appName != "" {
		for _, m := range modules {
			if subprojectShortName(m) == appName {
				return m, true
			}
		}
		return "", false
	}
	if len(modules) == 1 {
		return modules[0], true
	}
	return "", false
}
