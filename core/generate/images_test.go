package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeToMajor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{"simple major", "20", "20"},
		{"semver", "20.1.0", "20"},
		{"major.minor", "18.2", "18"},
		{"caret", "^22", "22"},
		{"caret semver", "^14.3.2", "14"},
		{"tilde", "~18.1.0", "18"},
		{"range >=", ">=22 <23", "22"},
		{"range >= spaced", ">= 22", "22"},
		{"v prefix", "v16", "16"},
		{"x notation", "14.x", "14"},
		{"empty", "", ""},
		{"star", "*", ""},
		{"latest", "latest", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NormalizeToMajor(tt.input))
		})
	}
}

func TestNormalizeToMajorMinor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{"major.minor", "3.12", "3.12"},
		{"semver", "3.12.1", "3.12"},
		{"major only", "3", "3"},
		{"caret", "^3.11.2", "3.11"},
		{"tilde", "~1.23.1", "1.23"},
		{"v prefix", "v1.22", "1.22"},
		{"range", ">=3.10 <3.12", "3.10"},
		{"empty", "", ""},
		{"star", "*", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NormalizeToMajorMinor(tt.input))
		})
	}
}

func TestNodeBuildImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"specific major", "22", "node:22-bookworm"},
		{"semver", "18.3.2", "node:18-bookworm"},
		{"caret", "^20", "node:20-bookworm"},
		{"range", ">=22 <23", "node:22-bookworm"},
		{"empty uses default", "", "node:20-bookworm"},
		{"star uses default", "*", "node:20-bookworm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NodeBuildImageForVersion(tt.version))
		})
	}
}

func TestNodeRuntimeImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "node:22-bookworm-slim", NodeRuntimeImageForVersion("22"))
	assert.Equal(t, "node:20-bookworm-slim", NodeRuntimeImageForVersion(""))
}

func TestPythonBuildImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"3.11", "3.11", "python:3.11-bookworm"},
		{"3.12.1", "3.12.1", "python:3.12-bookworm"},
		{"3.9", "3.9", "python:3.9-bookworm"},
		{"empty uses default", "", "python:3.12-bookworm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, PythonBuildImageForVersion(tt.version))
		})
	}
}

func TestPythonRuntimeImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "python:3.11-slim-bookworm", PythonRuntimeImageForVersion("3.11"))
	assert.Equal(t, "python:3.12-slim-bookworm", PythonRuntimeImageForVersion(""))
}

func TestGoBuildImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"1.22", "1.22", "golang:1.22-bookworm"},
		{"1.23.4", "1.23.4", "golang:1.23-bookworm"},
		{"1.21", "1.21", "golang:1.21-bookworm"},
		{"empty uses default", "", "golang:1.23-bookworm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, GoBuildImageForVersion(tt.version))
		})
	}
}

func TestRustBuildImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"major only", "1", "rust:1-bookworm"},
		{"major minor", "1.83", "rust:1.83-bookworm"},
		{"semver", "1.83.0", "rust:1.83.0-bookworm"},
		{"v prefix", "v1.75", "rust:1.75-bookworm"},
		{"empty uses default", "", "rust:1-bookworm"},
		{"star uses default", "*", "rust:1-bookworm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RustBuildImageForVersion(tt.version))
		})
	}
}

func TestJavaJdkImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "eclipse-temurin:21-jdk", JavaJdkImageForVersion("21"))
	assert.Equal(t, "eclipse-temurin:17-jdk", JavaJdkImageForVersion("17"))
	assert.Equal(t, "eclipse-temurin:21-jdk", JavaJdkImageForVersion(""))
	assert.Equal(t, "eclipse-temurin:21-jdk", JavaJdkImageForVersion("^21"))
	assert.Equal(t, "eclipse-temurin:11-jdk", JavaJdkImageForVersion("11.0.20"))
}

func TestJavaJreImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "eclipse-temurin:21-jre", JavaJreImageForVersion("21"))
	assert.Equal(t, "eclipse-temurin:17-jre", JavaJreImageForVersion("17"))
	assert.Equal(t, "eclipse-temurin:21-jre", JavaJreImageForVersion(""))
}

func TestGradleImageForJavaVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "gradle:8-jdk21", GradleImageForJavaVersion("21"))
	assert.Equal(t, "gradle:8-jdk17", GradleImageForJavaVersion("17"))
	assert.Equal(t, "gradle:8-jdk21", GradleImageForJavaVersion(""))
}

func TestMavenImageForJavaVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "maven:3-eclipse-temurin-21", MavenImageForJavaVersion("21"))
	assert.Equal(t, "maven:3-eclipse-temurin-17", MavenImageForJavaVersion("17"))
	assert.Equal(t, "maven:3-eclipse-temurin-21", MavenImageForJavaVersion(""))
}

func TestRubyImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"3.3", "3.3", "ruby:3.3-bookworm-slim"},
		{"3.2.5", "3.2.5", "ruby:3.2-bookworm-slim"},
		{"3.4", "3.4", "ruby:3.4-bookworm-slim"},
		{"empty uses default", "", "ruby:3.3-bookworm-slim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RubyImageForVersion(tt.version))
		})
	}
}

func TestPhpImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"8.3", "8.3", "php:8.3-cli-bookworm"},
		{"8.2.10", "8.2.10", "php:8.2-cli-bookworm"},
		{"8.1", "8.1", "php:8.1-cli-bookworm"},
		{"caret", "^8.2", "php:8.2-cli-bookworm"},
		{"empty uses default", "", "php:8.3-cli-bookworm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, PhpImageForVersion(tt.version))
		})
	}
}

func TestDotnetSdkImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"8.0", "8.0", "mcr.microsoft.com/dotnet/sdk:8.0"},
		{"6.0", "6.0", "mcr.microsoft.com/dotnet/sdk:6.0"},
		{"8.0.100", "8.0.100", "mcr.microsoft.com/dotnet/sdk:8.0"},
		{"empty uses default", "", "mcr.microsoft.com/dotnet/sdk:8.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, DotnetSdkImageForVersion(tt.version))
		})
	}
}

func TestDotnetAspnetImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mcr.microsoft.com/dotnet/aspnet:8.0", DotnetAspnetImageForVersion("8.0"))
	assert.Equal(t, "mcr.microsoft.com/dotnet/aspnet:6.0", DotnetAspnetImageForVersion("6.0"))
	assert.Equal(t, "mcr.microsoft.com/dotnet/aspnet:8.0", DotnetAspnetImageForVersion(""))
}

func TestDotnetRuntimeImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mcr.microsoft.com/dotnet/runtime:8.0", DotnetRuntimeImageForVersion("8.0"))
	assert.Equal(t, "mcr.microsoft.com/dotnet/runtime:6.0", DotnetRuntimeImageForVersion("6.0"))
	assert.Equal(t, "mcr.microsoft.com/dotnet/runtime:8.0", DotnetRuntimeImageForVersion(""))
}

func TestDenoImageForVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"major", "2", "denoland/deno:bin-2"},
		{"semver", "2.1.5", "denoland/deno:bin-2"},
		{"v prefix", "v1", "denoland/deno:bin-1"},
		{"empty uses default", "", "denoland/deno:bin-2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, DenoImageForVersion(tt.version))
		})
	}
}

func TestDenoRuntimeImageForVersion(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "denoland/deno:distroless-2", DenoRuntimeImageForVersion("2"))
	assert.Equal(t, "denoland/deno:distroless-1", DenoRuntimeImageForVersion("1"))
	assert.Equal(t, "denoland/deno:distroless-2", DenoRuntimeImageForVersion(""))
}

// Sanity check: the new default constants must not be empty (otherwise image strings
// would degrade silently to ":bookworm" tags that don't exist).
func TestDefaultVersionsNonEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, DefaultRustVersion)
	assert.NotEmpty(t, DefaultJavaVersion)
	assert.NotEmpty(t, DefaultRubyVersion)
	assert.NotEmpty(t, DefaultPhpVersion)
	assert.NotEmpty(t, DefaultDotnetVersion)
	assert.NotEmpty(t, DefaultDenoVersion)
	assert.Equal(t, "debian:bookworm-slim", RustRuntimeImage)
	assert.Equal(t, "composer:2", ComposerImage)
}
