package generate

import (
	"fmt"
	"strings"
)

// Default versions matching the hardcoded constants in deploy_builder.go.
// Used as fallback when no version is detected from project files or config.
const (
	DefaultNodeVersion   = "20"
	DefaultPythonVersion = "3.12"
	DefaultGoVersion     = "1.23"
	DefaultRustVersion   = "1"
	DefaultJavaVersion   = "21"
	DefaultRubyVersion   = "3.3"
	DefaultPhpVersion    = "8.3"
	DefaultDotnetVersion = "8.0"
	DefaultDenoVersion   = "2"
)

// Static runtime images. Used by providers whose runtime stage does not embed
// the language toolchain (Rust static binaries, PHP-CLI fallback, etc.).
const (
	// RustRuntimeImage is the runtime base for Rust binaries (static).
	RustRuntimeImage = "debian:bookworm-slim"

	// ComposerImage is the multi-stage source for the composer CLI.
	ComposerImage = "composer:2"
)

// NodeBuildImageForVersion returns the build image tag for a given Node major version.
// Example: "22" → "node:22-bookworm"
func NodeBuildImageForVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultNodeVersion
	}
	return fmt.Sprintf("node:%s-bookworm", v)
}

// NodeRuntimeImageForVersion returns the runtime image tag for a given Node major version.
// Example: "22" → "node:22-bookworm-slim"
func NodeRuntimeImageForVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultNodeVersion
	}
	return fmt.Sprintf("node:%s-bookworm-slim", v)
}

// PythonBuildImageForVersion returns the build image tag for a given Python major.minor version.
// Example: "3.11" → "python:3.11-bookworm"
func PythonBuildImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultPythonVersion
	}
	return fmt.Sprintf("python:%s-bookworm", v)
}

// PythonRuntimeImageForVersion returns the runtime image tag for a given Python major.minor version.
// Example: "3.11" → "python:3.11-slim-bookworm"
func PythonRuntimeImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultPythonVersion
	}
	return fmt.Sprintf("python:%s-slim-bookworm", v)
}

// GoBuildImageForVersion returns the build image tag for a given Go major.minor version.
// Example: "1.22" → "golang:1.22-bookworm"
func GoBuildImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultGoVersion
	}
	return fmt.Sprintf("golang:%s-bookworm", v)
}

// RustBuildImageForVersion returns the build image tag for a given Rust version.
// Example: "1" → "rust:1-bookworm", "1.83" → "rust:1.83-bookworm".
// Rust producers a static binary, so the runtime image is RustRuntimeImage (debian-slim),
// independent of build version.
func RustBuildImageForVersion(version string) string {
	v := normalizeRustVersion(version)
	if v == "" {
		v = DefaultRustVersion
	}
	return fmt.Sprintf("rust:%s-bookworm", v)
}

// JavaJdkImageForVersion returns the JDK image (Eclipse Temurin) for the given major version.
// Example: "21" → "eclipse-temurin:21-jdk".
func JavaJdkImageForVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultJavaVersion
	}
	return fmt.Sprintf("eclipse-temurin:%s-jdk", v)
}

// JavaJreImageForVersion returns the JRE-only runtime image for the given major version.
// Example: "21" → "eclipse-temurin:21-jre".
func JavaJreImageForVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultJavaVersion
	}
	return fmt.Sprintf("eclipse-temurin:%s-jre", v)
}

// GradleImageForJavaVersion returns the Gradle build image for the given Java major version.
// We pin Gradle 8 (LTS-friendly) and combine it with the requested JDK.
// Example: "21" → "gradle:8-jdk21".
func GradleImageForJavaVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultJavaVersion
	}
	return fmt.Sprintf("gradle:8-jdk%s", v)
}

// MavenImageForJavaVersion returns the Maven build image bundled with Eclipse Temurin
// for the given Java major version.
// Example: "21" → "maven:3-eclipse-temurin-21".
func MavenImageForJavaVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultJavaVersion
	}
	return fmt.Sprintf("maven:3-eclipse-temurin-%s", v)
}

// RubyImageForVersion returns a single Ruby image used for both build and runtime.
// Example: "3.3" → "ruby:3.3-bookworm-slim".
func RubyImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultRubyVersion
	}
	return fmt.Sprintf("ruby:%s-bookworm-slim", v)
}

// PhpImageForVersion returns a single PHP CLI image used for both build and runtime.
// Example: "8.3" → "php:8.3-cli-bookworm".
func PhpImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultPhpVersion
	}
	return fmt.Sprintf("php:%s-cli-bookworm", v)
}

// DotnetSdkImageForVersion returns the .NET SDK build image for the given version.
// .NET versions are always major.minor (e.g., "8.0", "6.0"); we normalize accordingly.
// Example: "8.0" → "mcr.microsoft.com/dotnet/sdk:8.0".
func DotnetSdkImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultDotnetVersion
	}
	return fmt.Sprintf("mcr.microsoft.com/dotnet/sdk:%s", v)
}

// DotnetAspnetImageForVersion returns the ASP.NET Core runtime image (includes Kestrel).
// Use this when the project targets Microsoft.NET.Sdk.Web or references Microsoft.AspNetCore.*.
// Example: "8.0" → "mcr.microsoft.com/dotnet/aspnet:8.0".
func DotnetAspnetImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultDotnetVersion
	}
	return fmt.Sprintf("mcr.microsoft.com/dotnet/aspnet:%s", v)
}

// DotnetRuntimeImageForVersion returns the slimmer .NET runtime image (no ASP.NET).
// Use this for console apps and workers (Microsoft.NET.Sdk with OutputType=Exe).
// Example: "8.0" → "mcr.microsoft.com/dotnet/runtime:8.0".
func DotnetRuntimeImageForVersion(version string) string {
	v := NormalizeToMajorMinor(version)
	if v == "" {
		v = DefaultDotnetVersion
	}
	return fmt.Sprintf("mcr.microsoft.com/dotnet/runtime:%s", v)
}

// DenoImageForVersion returns the Deno build image (full Debian-based) for the given major version.
// Example: "2" → "denoland/deno:bin-2".
func DenoImageForVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultDenoVersion
	}
	return fmt.Sprintf("denoland/deno:bin-%s", v)
}

// DenoRuntimeImageForVersion returns the distroless Deno runtime image for the given major version.
// Example: "2" → "denoland/deno:distroless-2".
func DenoRuntimeImageForVersion(version string) string {
	v := NormalizeToMajor(version)
	if v == "" {
		v = DefaultDenoVersion
	}
	return fmt.Sprintf("denoland/deno:distroless-%s", v)
}

// normalizeRustVersion preserves the level of precision the user gave (major, major.minor,
// major.minor.patch) so users targeting "1.75" get exactly that, while "1" produces a
// rolling-stable image. Rust's official tag scheme ships images for all three forms.
func normalizeRustVersion(version string) string {
	return cleanVersionPrefix(version)
}

// NormalizeToMajor extracts the major version component.
// Handles common formats: semver, ranges, caret/tilde notation, v-prefix.
// "20.1.0" → "20", "^22" → "22", ">=18 <20" → "18", "v16" → "16"
func NormalizeToMajor(version string) string {
	v := cleanVersionPrefix(version)
	if v == "" || v == "latest" {
		return ""
	}
	parts := strings.Split(v, ".")
	return parts[0]
}

// NormalizeToMajorMinor extracts major.minor version components.
// "3.12.1" → "3.12", "1.23" → "1.23", "3" → "3"
func NormalizeToMajorMinor(version string) string {
	v := cleanVersionPrefix(version)
	if v == "" || v == "latest" {
		return ""
	}
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return parts[0]
}

// cleanVersionPrefix strips common prefixes and resolves range/caret/tilde notation
// to a plain version string.
func cleanVersionPrefix(version string) string {
	version = strings.TrimSpace(version)

	if version == "" || version == "*" {
		return ""
	}

	// Handle range notation: ">=22 <23" → "22"
	if strings.Contains(version, ">=") {
		parts := strings.Fields(version)
		for i, part := range parts {
			if strings.HasPrefix(part, ">=") {
				v := strings.TrimPrefix(part, ">=")
				if v == "" && i+1 < len(parts) {
					v = parts[i+1]
				}
				return strings.TrimSpace(v)
			}
		}
	}

	// Caret: "^14.3.2" → "14.3.2" (caller decides precision)
	version = strings.TrimPrefix(version, "^")
	version = strings.TrimPrefix(version, "~")
	version = strings.TrimPrefix(version, "v")

	// Remove .x notation: "14.x" → "14"
	version = strings.ReplaceAll(version, ".x", "")
	version = strings.TrimRight(version, ".")

	return version
}
