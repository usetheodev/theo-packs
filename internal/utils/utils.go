package utils

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

func RemoveDuplicates[T comparable](sliceList []T) []T {
	allKeys := make(map[T]bool)
	list := []T{}
	for _, item := range sliceList {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

// MergeStringSlicePointers combines multiple string slice pointers, deduplicates values, and sorts them
func MergeStringSlicePointers(slices ...*[]string) *[]string {
	if len(slices) == 0 {
		return nil
	}

	var allStrings []string
	for _, slice := range slices {
		if slice != nil {
			allStrings = append(allStrings, *slice...)
		}
	}

	if len(allStrings) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var uniqueStrings []string
	for _, s := range allStrings {
		if !seen[s] {
			seen[s] = true
			uniqueStrings = append(uniqueStrings, s)
		}
	}
	sort.Strings(uniqueStrings)
	return &uniqueStrings
}

func CapitalizeFirst(s string) string {
	if s == "" {
		return ""
	}

	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func ParsePackageWithVersion(versions []string) map[string]string {
	parsedVersions := make(map[string]string)

	for _, version := range versions {
		parts := strings.Split(version, "@")
		if len(parts) == 1 {
			parsedVersions[parts[0]] = "latest"
		} else {
			parsedVersions[parts[0]] = parts[1]
		}
	}

	return parsedVersions
}

func ExtractSemverVersion(version string) string {
	semverRe := regexp.MustCompile(`(\d+(?:\.\d+)?(?:\.\d+)?)`)
	matches := semverRe.FindStringSubmatch(strings.TrimSpace(version))
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ParseSemver parses a semantic version string and returns a Semver struct.
func ParseSemver(version string) (*Semver, error) {
	// Handle corepack version format like "pnpm@8.15.4"
	if idx := strings.Index(version, "@"); idx != -1 {
		version = version[idx+1:]
	}

	version = strings.TrimPrefix(version, "v")

	parts := strings.Split(version, ".")

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor := 0
	patch := 0
	if len(parts) > 1 {
		minorVer, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid minor version: %s", parts[1])
		}
		minor = minorVer
	}

	if len(parts) > 2 {
		patchStr := parts[2]
		patchParts := strings.Split(patchStr, "-")
		patchVer, err := strconv.Atoi(patchParts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid patch version: %s", patchParts[0])
		}
		patch = patchVer
	}

	return &Semver{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

type Semver struct {
	Major int
	Minor int
	Patch int
}

// StandardizeJSON converts hujson/extended JSON into standardized json
func StandardizeJSON(b []byte) ([]byte, error) {
	ast, err := hujson.Parse(b)
	if err != nil {
		return b, err
	}
	ast.Standardize()
	return ast.Pack(), nil
}
