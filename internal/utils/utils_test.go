package utils

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveDuplicates(t *testing.T) {
	require.Equal(t, []string{"a", "b", "c"}, RemoveDuplicates([]string{"a", "b", "a", "c", "b"}))
	require.Equal(t, []int{1, 2, 3}, RemoveDuplicates([]int{1, 2, 1, 3, 2}))
	require.Empty(t, RemoveDuplicates([]string{}))
	require.Equal(t, []string{"a"}, RemoveDuplicates([]string{"a", "a", "a"}))
}

func TestCapitalizeFirst(t *testing.T) {
	require.Equal(t, "Hello", CapitalizeFirst("hello"))
	require.Equal(t, "Go", CapitalizeFirst("go"))
	require.Equal(t, "", CapitalizeFirst(""))
	require.Equal(t, "A", CapitalizeFirst("a"))
	require.Equal(t, "Already", CapitalizeFirst("Already"))
}

func TestParsePackageWithVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:     "versioned packages",
			input:    []string{"node@18", "python@3.9"},
			expected: map[string]string{"node": "18", "python": "3.9"},
		},
		{
			name:     "unversioned packages",
			input:    []string{"jq", "curl"},
			expected: map[string]string{"jq": "latest", "curl": "latest"},
		},
		{
			name:     "mixed",
			input:    []string{"node@20", "jq"},
			expected: map[string]string{"node": "20", "jq": "latest"},
		},
		{
			name:     "namespaced package",
			input:    []string{"pipx:httpie@3.2.4"},
			expected: map[string]string{"pipx:httpie": "3.2.4"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParsePackageWithVersion(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractSemverVersion(t *testing.T) {
	require.Equal(t, "1.2.3", ExtractSemverVersion("v1.2.3"))
	require.Equal(t, "22", ExtractSemverVersion("node 22"))
	require.Equal(t, "3.9", ExtractSemverVersion("python 3.9"))
	require.Equal(t, "", ExtractSemverVersion("no version here"))
	require.Equal(t, "1.2.3", ExtractSemverVersion("  1.2.3  "))
}

func TestStandardizeJSON(t *testing.T) {
	t.Run("standard json", func(t *testing.T) {
		input := []byte(`{"key": "value"}`)
		result, err := StandardizeJSON(input)
		require.NoError(t, err)
		require.JSONEq(t, `{"key": "value"}`, string(result))
	})

	t.Run("json with comments", func(t *testing.T) {
		input := []byte(`{
			// comment
			"key": "value"
		}`)
		result, err := StandardizeJSON(input)
		require.NoError(t, err)
		require.Contains(t, string(result), `"key"`)
	})

	t.Run("json with trailing comma", func(t *testing.T) {
		input := []byte(`{"key": "value",}`)
		result, err := StandardizeJSON(input)
		require.NoError(t, err)
		require.JSONEq(t, `{"key": "value"}`, string(result))
	})

	t.Run("invalid json", func(t *testing.T) {
		input := []byte(`{invalid`)
		_, err := StandardizeJSON(input)
		require.Error(t, err)
	})
}

func TestMergeStructs(t *testing.T) {
	type Inner struct {
		Value string
	}
	type TestStruct struct {
		Name  string
		Count int
		Tags  []string
		Data  map[string]string
		Inner *Inner
	}

	t.Run("merge basic fields", func(t *testing.T) {
		dst := &TestStruct{Name: "original", Count: 1}
		src := &TestStruct{Name: "updated", Count: 0}
		require.NoError(t, MergeStructs(dst, src))
		require.Equal(t, "updated", dst.Name)
		require.Equal(t, 1, dst.Count, "zero values should not override")
	})

	t.Run("merge slices", func(t *testing.T) {
		dst := &TestStruct{Tags: []string{"a"}}
		src := &TestStruct{Tags: []string{"b", "c"}}
		require.NoError(t, MergeStructs(dst, src))
		require.Equal(t, []string{"b", "c"}, dst.Tags, "slices should be replaced entirely")
	})

	t.Run("merge maps", func(t *testing.T) {
		dst := &TestStruct{Data: map[string]string{"a": "1", "b": "2"}}
		src := &TestStruct{Data: map[string]string{"b": "3", "c": "4"}}
		require.NoError(t, MergeStructs(dst, src))
		require.Equal(t, "1", dst.Data["a"])
		require.Equal(t, "3", dst.Data["b"], "later values win")
		require.Equal(t, "4", dst.Data["c"])
	})

	t.Run("merge pointer fields", func(t *testing.T) {
		dst := &TestStruct{Inner: &Inner{Value: "old"}}
		src := &TestStruct{Inner: &Inner{Value: "new"}}
		require.NoError(t, MergeStructs(dst, src))
		require.Equal(t, "new", dst.Inner.Value)
	})

	t.Run("nil src pointer does not override", func(t *testing.T) {
		dst := &TestStruct{Inner: &Inner{Value: "keep"}}
		src := &TestStruct{}
		require.NoError(t, MergeStructs(dst, src))
		require.Equal(t, "keep", dst.Inner.Value)
	})

	t.Run("nil src skipped", func(t *testing.T) {
		dst := &TestStruct{Name: "keep"}
		require.NoError(t, MergeStructs(dst, nil))
		require.Equal(t, "keep", dst.Name)
	})
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		want        *Semver
		wantErr     bool
		errorPrefix string
	}{
		{
			name:    "basic semver",
			version: "1.2.3",
			want:    &Semver{Major: 1, Minor: 2, Patch: 3},
			wantErr: false,
		},
		{
			name:    "with v prefix",
			version: "v1.2.3",
			want:    &Semver{Major: 1, Minor: 2, Patch: 3},
			wantErr: false,
		},
		{
			name:        "with other prefix",
			version:     "ruby-2.3.4",
			want:        nil,
			wantErr:     true,
			errorPrefix: "invalid major version",
		},
		{
			name:    "with alpha suffix",
			version: "1.2.3-alpha",
			want:    &Semver{Major: 1, Minor: 2, Patch: 3},
			wantErr: false,
		},
		{
			name:    "with beta suffix",
			version: "1.2.3-beta",
			want:    &Semver{Major: 1, Minor: 2, Patch: 3},
			wantErr: false,
		},
		{
			name:    "two parts",
			version: "1.2",
			want:    &Semver{Major: 1, Minor: 2, Patch: 0},
		},
		{
			name:        "invalid major",
			version:     "a.2.3",
			want:        nil,
			wantErr:     true,
			errorPrefix: "invalid major version",
		},
		{
			name:        "invalid minor",
			version:     "1.b.3",
			want:        nil,
			wantErr:     true,
			errorPrefix: "invalid minor version",
		},
		{
			name:        "invalid patch",
			version:     "1.2.c",
			want:        nil,
			wantErr:     true,
			errorPrefix: "invalid patch version",
		},
		{
			name:        "invalid",
			version:     "1-23",
			want:        nil,
			wantErr:     true,
			errorPrefix: "invalid major version",
		},
		{
			name:    "corepack version",
			version: "pnpm@8.15.4",
			want: &Semver{
				Major: 8,
				Minor: 15,
				Patch: 4,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSemver(tt.version)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSemver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.HasPrefix(err.Error(), tt.errorPrefix) {
					t.Errorf("ParseSemver() error = %v, wantErrPrefix %v", err, tt.errorPrefix)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSemver() = %v, want %v", got, tt.want)
			}
		})
	}
}
