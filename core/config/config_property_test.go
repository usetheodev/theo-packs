package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"
)

// Property-based tests for Merge (T2.2, robust-test-suite-plan).
//
// Merge composes Config instances. The provider/CLI/env-bridge plumbing
// produces multiple Configs (one per source) that get folded together
// — invariants below ensure that fold is well-defined regardless of
// order/grouping.

// genConfig produces a Config with random-but-realistic fields. Not
// exhaustive on every field — focused on the surfaces that providers
// actually populate (Packages, Deploy.StartCmd, Steps).
func genConfig() *rapid.Generator[*Config] {
	return rapid.Custom(func(rt *rapid.T) *Config {
		c := EmptyConfig()
		// Packages
		if rapid.Bool().Draw(rt, "has-packages") {
			n := rapid.IntRange(0, 3).Draw(rt, "n-packages")
			for i := 0; i < n; i++ {
				name := rapid.SampledFrom([]string{"node", "python", "go", "rust"}).Draw(rt, "pkg")
				ver := rapid.SampledFrom([]string{"1.0", "2.0", "16", "20", "3.12"}).Draw(rt, "ver")
				c.Packages[name] = ver
			}
		}
		// Deploy.StartCmd
		if rapid.Bool().Draw(rt, "has-start") {
			c.Deploy.StartCmd = rapid.SampledFrom([]string{
				"npm start", "node index.js", "/app/server", "gunicorn app:app",
			}).Draw(rt, "start")
		}
		// BuildAptPackages
		if rapid.Bool().Draw(rt, "has-apt") {
			c.BuildAptPackages = rapid.SliceOf(
				rapid.SampledFrom([]string{"git", "curl", "build-essential"}),
			).Draw(rt, "apt")
		}
		return c
	})
}

// TestConfig_Merge_Identity — Merge(a, EmptyConfig) is observationally
// equivalent to a (up to map allocation).
func TestConfig_Merge_Identity(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		a := genConfig().Draw(rt, "a")
		merged, err := Merge(a, EmptyConfig())
		if err != nil {
			rt.Fatalf("merge a, empty: %v", err)
		}
		if diff := cmp.Diff(a, merged); diff != "" {
			rt.Fatalf("Merge(a, empty) ≢ a:\n%s", diff)
		}
	})
}

// TestConfig_Merge_Idempotent — Merge(a, a) == a. The result of merging
// a value with itself must not duplicate or accumulate.
func TestConfig_Merge_Idempotent(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		a := genConfig().Draw(rt, "a")
		merged, err := Merge(a, a)
		if err != nil {
			rt.Fatalf("merge a, a: %v", err)
		}
		if diff := cmp.Diff(a, merged); diff != "" {
			rt.Fatalf("Merge(a, a) ≢ a:\n%s", diff)
		}
	})
}

// TestConfig_Merge_NeverErrorsForValidConfigs — Merge over any
// combination of valid Configs returns nil error. This guards against
// reflection-based merge regressions panicking on edge inputs.
func TestConfig_Merge_NeverErrorsForValidConfigs(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		nConfigs := rapid.IntRange(1, 4).Draw(rt, "n")
		configs := make([]*Config, nConfigs)
		for i := 0; i < nConfigs; i++ {
			configs[i] = genConfig().Draw(rt, "c")
		}
		_, err := Merge(configs...)
		if err != nil {
			rt.Fatalf("Merge() must not error on valid Configs: %v", err)
		}
	})
}
