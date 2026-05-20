package resolver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// Property-based tests for the Resolver (T2.1, robust-test-suite-plan).
//
// The Resolver is a small state machine with two primitives:
//   - Default(name, default): register a package with its default version
//   - Version(ref, version, source): override the version with a source label
// Properties verified here are invariants any provider relies on.

// genSemver generates plausible version strings (not exhaustive — domain
// generator focused on what providers actually emit).
func genSemver() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"1", "1.0", "1.0.0", "1.2.3", "2", "16", "16.18.0", "20", "22",
		"3.9", "3.11", "3.12", "0.1.0", "*", "latest",
	})
}

func genPackageName() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"node", "python", "go", "rust", "ruby", "java"})
}

func genSource() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"theopacks.json", "THEOPACKS_NODE_VERSION", ".nvmrc",
		"package.json engines.node", "custom config", "rust-toolchain.toml",
	})
}

// TestResolver_VersionLastWriteWins — applying Version() multiple times
// must leave Get(name).Version equal to the last value written.
func TestResolver_VersionLastWriteWins(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		name := genPackageName().Draw(rt, "name")
		defaultV := genSemver().Draw(rt, "default")
		writes := rapid.SliceOfN(genSemver(), 1, 5).Draw(rt, "writes")

		r := NewResolver()
		ref := r.Default(name, defaultV)
		for _, v := range writes {
			r.Version(ref, v, "test")
		}
		got := r.Get(name)
		require.NotNil(rt, got)
		require.Equal(rt, writes[len(writes)-1], got.Version,
			"last Version() write must win; defaultV=%q writes=%v", defaultV, writes)
	})
}

// TestResolver_DefaultPreviousVersionAppliesWhenDifferent — when
// SetPreviousVersion is set BEFORE Default and previous != default, the
// resolver promotes the previous version with source "previous installed
// version" (locking the upgrade path described in resolver.go::Default).
func TestResolver_DefaultPreviousVersionAppliesWhenDifferent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		name := genPackageName().Draw(rt, "name")
		defaultV := genSemver().Draw(rt, "default")
		prevV := genSemver().Draw(rt, "previous")

		r := NewResolver()
		r.SetPreviousVersion(name, prevV)
		r.Default(name, defaultV)

		got := r.Get(name)
		require.NotNil(rt, got)
		if prevV == defaultV || prevV == "" {
			// Previous == default OR empty → default keeps its place
			require.Equal(rt, defaultV, got.Version)
		} else {
			require.Equal(rt, prevV, got.Version,
				"previous version must override default when distinct")
			require.Equal(rt, "previous installed version", got.Source)
		}
	})
}

// TestResolver_ResolveIdempotent — calling ResolvePackages multiple
// times in a row produces equal output (map keys + each ResolvedPackage
// equal field-by-field).
func TestResolver_ResolveIdempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		nPkgs := rapid.IntRange(0, 4).Draw(rt, "n")
		r := NewResolver()
		for i := 0; i < nPkgs; i++ {
			name := genPackageName().Draw(rt, "name")
			defaultV := genSemver().Draw(rt, "default")
			ref := r.Default(name, defaultV)
			if rapid.Bool().Draw(rt, "override") {
				v := genSemver().Draw(rt, "override-v")
				s := genSource().Draw(rt, "override-s")
				r.Version(ref, v, s)
			}
		}

		got1, err1 := r.ResolvePackages()
		require.NoError(rt, err1)
		got2, err2 := r.ResolvePackages()
		require.NoError(rt, err2)

		require.Equal(rt, len(got1), len(got2),
			"resolve N times must produce same cardinality")
		for k, v1 := range got1 {
			v2, ok := got2[k]
			require.True(rt, ok)
			require.Equal(rt, v1.Source, v2.Source)
			require.Equal(rt, v1.Name, v2.Name)
			require.Equal(rt, *v1.RequestedVersion, *v2.RequestedVersion)
		}
	})
}
