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
