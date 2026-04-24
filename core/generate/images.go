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
