// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package java

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
