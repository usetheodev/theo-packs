package resolver

import "strings"

func resolveToFuzzyVersion(version string) string {
	version = strings.TrimSpace(version)

	if version == "" || version == "*" {
		return "latest"
	}

	// Handle range notation (e.g. ">=22 <23" or ">= 22" or ">=20.0.0")
	if strings.Contains(version, ">=") || strings.Contains(version, "<") {
		parts := strings.Fields(version)
		for i, part := range parts {
			if strings.HasPrefix(part, ">=") {
				v := strings.TrimPrefix(part, ">=")
				if v == "" && i+1 < len(parts) {
					v = parts[i+1]
				}
				return strings.Split(strings.TrimSpace(v), ".")[0]
			}
		}
	}

	// Handle caret notation by only keeping major version
	if strings.HasPrefix(version, "^") {
		version = strings.TrimPrefix(version, "^")
		parts := strings.Split(version, ".")
		return parts[0]
	}

	version = strings.TrimPrefix(version, "~")
	version = strings.TrimPrefix(version, "v")

	version = strings.ReplaceAll(version, ".x", "")

	version = strings.TrimRight(version, ".")

	return version
}
