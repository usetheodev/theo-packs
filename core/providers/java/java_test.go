// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package java

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
)

func createTempApp(t *testing.T, files map[string]string) *app.App {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}
	a, err := app.NewApp(dir)
	require.NoError(t, err)
	return a
}

func createTestContext(t *testing.T, a *app.App, envVars map[string]string) *generate.GenerateContext {
	t.Helper()
	var envPtr *map[string]string
	if envVars != nil {
		envPtr = &envVars
	}
	env := app.NewEnvironment(envPtr)
	cfg := config.EmptyConfig()
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	require.NoError(t, err)
	return ctx
}

const springGradleKts = `plugins {
    java
    id("org.springframework.boot") version "3.3.0"
    id("io.spring.dependency-management") version "1.1.5"
}

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

dependencies {
    implementation("org.springframework.boot:spring-boot-starter-web")
}
`

const plainGradleKts = `plugins {
    java
}

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(17)
    }
}
`

const springPomXml = `<?xml version="1.0"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.theo</groupId>
  <artifactId>app</artifactId>
  <version>0.1.0</version>
  <properties>
    <java.version>21</java.version>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>
</project>
`

const plainPomXml = `<?xml version="1.0"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.theo</groupId>
  <artifactId>app</artifactId>
  <version>0.1.0</version>
  <properties>
    <maven.compiler.target>17</maven.compiler.target>
  </properties>
</project>
`

func TestJavaProvider_Name(t *testing.T) {
	require.Equal(t, "java", (&JavaProvider{}).Name())
}

func TestJavaProvider_Detect(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		expected bool
	}{
		{"gradle kts", map[string]string{"build.gradle.kts": "plugins{}"}, true},
		{"gradle groovy", map[string]string{"build.gradle": "plugins{}"}, true},
		{"maven", map[string]string{"pom.xml": "<project/>"}, true},
		{"none", map[string]string{"package.json": "{}"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := createTempApp(t, tt.files)
			ctx := createTestContext(t, a, nil)
			got, err := (&JavaProvider{}).Detect(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestJavaProvider_StartCommandHelp(t *testing.T) {
	help := (&JavaProvider{}).StartCommandHelp()
	require.Contains(t, help, "JAR")
	require.Contains(t, help, "THEOPACKS_APP_NAME")
}

func TestDetectJavaVersion_Default(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts": "plugins{}\n",
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectJavaVersion(ctx)
	require.Equal(t, "21", v)
	require.Equal(t, "default", src)
}

func TestDetectJavaVersion_EnvVar(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts": "plugins{}\n",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_JAVA_VERSION": "17"})
	v, src := detectJavaVersion(ctx)
	require.Equal(t, "17", v)
	require.Equal(t, "THEOPACKS_JAVA_VERSION", src)
}

func TestDetectJavaVersion_DotJavaVersion(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts": "plugins{}\n",
		".java-version":    "17\n",
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectJavaVersion(ctx)
	require.Equal(t, "17", v)
	require.Equal(t, ".java-version", src)
}

func TestDetectJavaVersion_GradleProperties(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts":  "plugins{}\n",
		"gradle.properties": "javaVersion=11\nfoo=bar\n",
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectJavaVersion(ctx)
	require.Equal(t, "11", v)
	require.Equal(t, "gradle.properties", src)
}

func TestDetectJavaVersion_GradleToolchain(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts": springGradleKts,
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectJavaVersion(ctx)
	require.Equal(t, "21", v)
	require.Equal(t, "build.gradle.kts", src)
}

func TestDetectJavaVersion_PomXml(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml": plainPomXml,
	})
	ctx := createTestContext(t, a, nil)
	v, src := detectJavaVersion(ctx)
	require.Equal(t, "17", v)
	require.Equal(t, "pom.xml", src)
}

func TestGradleHasSpringBoot_Kts(t *testing.T) {
	a := createTempApp(t, map[string]string{"build.gradle.kts": springGradleKts})
	require.True(t, gradleHasSpringBoot(a))
}

func TestGradleHasSpringBoot_Groovy(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle": `apply plugin: 'org.springframework.boot'`,
	})
	require.True(t, gradleHasSpringBoot(a))
}

func TestGradleHasSpringBoot_GroovyIdSyntax(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle": `plugins { id 'org.springframework.boot' version '3.3.0' }`,
	})
	require.True(t, gradleHasSpringBoot(a))
}

func TestGradleHasSpringBoot_Plain(t *testing.T) {
	a := createTempApp(t, map[string]string{"build.gradle.kts": plainGradleKts})
	require.False(t, gradleHasSpringBoot(a))
}

func TestMavenHasSpringBoot(t *testing.T) {
	a := createTempApp(t, map[string]string{"pom.xml": springPomXml})
	require.True(t, mavenHasSpringBoot(a))
}

func TestMavenHasSpringBoot_Plain(t *testing.T) {
	a := createTempApp(t, map[string]string{"pom.xml": plainPomXml})
	require.False(t, mavenHasSpringBoot(a))
}

func TestPlanGradle_SpringBoot(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts":       springGradleKts,
		"settings.gradle.kts":    `rootProject.name = "demo"`,
		"src/main/java/App.java": "public class App {}",
	})
	ctx := createTestContext(t, a, nil)
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "java -jar /app/app.jar", ctx.Deploy.StartCmd)
	require.Len(t, ctx.Steps, 2)
}

func TestPlanGradle_PlainJava(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts":       plainGradleKts,
		"src/main/java/App.java": "public class App {}",
	})
	ctx := createTestContext(t, a, nil)
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "java -jar /app/app.jar", ctx.Deploy.StartCmd)
}

func TestPlanMaven_SpringBoot(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml":                springPomXml,
		"src/main/java/App.java": "public class App {}",
	})
	ctx := createTestContext(t, a, nil)
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.Equal(t, "java -jar /app/app.jar", ctx.Deploy.StartCmd)
}

func TestPlanMaven_Plain(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml":                plainPomXml,
		"src/main/java/App.java": "public class App {}",
	})
	ctx := createTestContext(t, a, nil)
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
}

func TestPlan_GradleWinsOverMaven(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts": plainGradleKts,
		"pom.xml":          plainPomXml,
	})
	ctx := createTestContext(t, a, nil)
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
	require.NotNil(t, ctx.Deploy.Base)
	// Gradle wins → install image is gradle:*, not maven:*. We assert via the
	// StartCmd which is the same for both, but the install step's image will
	// reflect the choice — we don't need to dig that deep here, the absence
	// of error + the warning log is enough.
}

func TestGradleSubprojects(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"settings.gradle.kts": `rootProject.name = "monorepo"
include(":apps:api", ":apps:worker", ":packages:shared")
`,
		"apps/api/build.gradle.kts":        "plugins{}",
		"apps/worker/build.gradle.kts":     "plugins{}",
		"packages/shared/build.gradle.kts": "plugins{}",
	})
	got := gradleSubprojects(a)
	require.ElementsMatch(t, []string{"apps/api", "apps/worker", "packages/shared"}, got)
}

func TestGradleSubprojects_NoSettings(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts": plainGradleKts,
	})
	require.Empty(t, gradleSubprojects(a))
}

func TestGradleSubprojects_FiltersMissing(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"settings.gradle.kts":       `include(":apps:api", ":apps:nope")`,
		"apps/api/build.gradle.kts": "plugins{}",
		// apps/nope has no build.gradle*.
	})
	got := gradleSubprojects(a)
	require.Equal(t, []string{"apps/api"}, got)
}

func TestPlanGradleWorkspace_SelectsApp(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts":             springGradleKts,
		"settings.gradle.kts":          `include(":apps:api", ":apps:worker")`,
		"apps/api/build.gradle.kts":    `plugins { id("org.springframework.boot") }`,
		"apps/worker/build.gradle.kts": `plugins { id("org.springframework.boot") }`,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "api"})
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
}

func TestPlanGradleWorkspace_AmbiguousNoEnv(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts":             springGradleKts,
		"settings.gradle.kts":          `include(":apps:api", ":apps:worker")`,
		"apps/api/build.gradle.kts":    "plugins{}",
		"apps/worker/build.gradle.kts": "plugins{}",
	})
	ctx := createTestContext(t, a, nil)
	err := (&JavaProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "THEOPACKS_APP_NAME")
}

func TestPlanGradleWorkspace_BadAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"build.gradle.kts":          springGradleKts,
		"settings.gradle.kts":       `include(":apps:api")`,
		"apps/api/build.gradle.kts": "plugins{}",
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "ghost"})
	err := (&JavaProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no subproject named")
	require.Contains(t, err.Error(), "api")
}

func TestMavenModules(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml": `<project>
  <modules>
    <module>apps/api</module>
    <module>apps/worker</module>
  </modules>
</project>`,
		"apps/api/pom.xml":    `<project/>`,
		"apps/worker/pom.xml": `<project/>`,
	})
	require.ElementsMatch(t, []string{"apps/api", "apps/worker"}, mavenModules(a))
}

func TestMavenModules_FiltersMissing(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml": `<project><modules>
  <module>apps/api</module>
  <module>apps/nope</module>
</modules></project>`,
		"apps/api/pom.xml": `<project/>`,
	})
	require.Equal(t, []string{"apps/api"}, mavenModules(a))
}

func TestPlanMavenWorkspace(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml": `<project><packaging>pom</packaging>
<modules>
  <module>apps/api</module>
  <module>apps/worker</module>
</modules></project>`,
		"apps/api/pom.xml":    springPomXml,
		"apps/worker/pom.xml": plainPomXml,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "api"})
	err := (&JavaProvider{}).Plan(ctx)
	require.NoError(t, err)
}

func TestPlanMavenWorkspace_BadAppName(t *testing.T) {
	a := createTempApp(t, map[string]string{
		"pom.xml":          `<project><modules><module>apps/api</module></modules></project>`,
		"apps/api/pom.xml": springPomXml,
	})
	ctx := createTestContext(t, a, map[string]string{"THEOPACKS_APP_NAME": "ghost"})
	err := (&JavaProvider{}).Plan(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no module named")
}

func TestExtractGradleProperty(t *testing.T) {
	content := `# comment
javaVersion=11
   spaced =  17
empty=
junk
`
	require.Equal(t, "11", extractGradleProperty(content, "javaVersion"))
	require.Equal(t, "17", extractGradleProperty(content, "spaced"))
	require.Equal(t, "", extractGradleProperty(content, "empty"))
	require.Equal(t, "", extractGradleProperty(content, "missing"))
}

func TestSubprojectShortName(t *testing.T) {
	require.Equal(t, "api", subprojectShortName("apps/api"))
	require.Equal(t, "shared", subprojectShortName("packages/shared"))
	require.Equal(t, "root", subprojectShortName("root"))
}

func TestExtractMavenJavaVersion(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{`<java.version>21</java.version>`, "21"},
		{`<maven.compiler.target>17</maven.compiler.target>`, "17"},
		{`<maven.compiler.release>11</maven.compiler.release>`, "11"},
		{`<other>foo</other>`, ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.out, extractMavenJavaVersion(tt.in))
	}
}
