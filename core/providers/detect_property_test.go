package providers

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"github.com/usetheo/theopacks/core/app"
	c "github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
)

// Property: detection is stable against noise files (T2.3,
// robust-test-suite-plan).
//
// For any synthetic project where ONE provider's signal is present
// (e.g., package.json for Node), adding arbitrary noise files
// (README.md, .gitignore, LICENSE, src/foo.txt) must NOT change which
// provider Detect()s positively. This guards against regex/glob
// regressions in any provider's Detect() implementation that
// accidentally treats arbitrary files as signals.

// detectorSignals enumerates the canonical file that triggers each
// provider. One signal per language is enough to exercise the
// invariant; multi-file signals (Pipfile + setup.py for Python) are
// covered by other tests.
var detectorSignals = []struct {
	provider string
	files    map[string]string
}{
	{"go", map[string]string{"go.mod": "module x\ngo 1.22\n", "main.go": "package main\nfunc main(){}\n"}},
	{"node", map[string]string{"package.json": `{"name":"x","scripts":{"start":"node ."}}`}},
	{"python", map[string]string{"requirements.txt": "flask==2.0\n"}},
	{"rust", map[string]string{"Cargo.toml": `[package]
name = "x"
version = "0.1.0"
edition = "2021"
`}},
	{"ruby", map[string]string{"Gemfile": `source "https://rubygems.org"
gem "sinatra"
`}},
	{"php", map[string]string{"composer.json": `{"name":"x/y","require":{"php":"^8.1"}}`}},
	{"deno", map[string]string{"deno.json": `{"tasks":{"start":"deno run main.ts"}}`, "main.ts": ""}},
}

// genNoiseFilename generates filenames that are NOT detector signals
// for any registered provider.
func genNoiseFilename() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"README.md", "LICENSE", ".gitignore", "CHANGELOG.md",
		"docs/architecture.md", "scripts/deploy.sh", "config.yaml",
		"data.csv", ".editorconfig", ".github/workflows/ci.yml",
	})
}

// detectFor returns the provider that detects positively against the
// given directory, or "" when none matches.
func detectFor(t *rapid.T, dir string) string {
	t.Helper()
	a, err := app.NewApp(dir)
	if err != nil {
		t.Fatalf("NewApp(%q): %v", dir, err)
	}
	env := app.NewEnvironment(nil)
	cfg := c.EmptyConfig()
	log := logger.NewLogger()
	ctx, err := generate.NewGenerateContext(a, env, cfg, log)
	if err != nil {
		t.Fatalf("NewGenerateContext: %v", err)
	}
	for _, p := range GetLanguageProviders() {
		ok, err := p.Detect(ctx)
		if err != nil {
			t.Fatalf("Detect(%s): %v", p.Name(), err)
		}
		if ok {
			return p.Name()
		}
	}
	return ""
}

func TestProviders_DetectionStableAgainstNoise(t *testing.T) {
	t.Parallel()
	for _, sig := range detectorSignals {
		sig := sig
		t.Run(sig.provider, func(t *testing.T) {
			t.Parallel()
			rapid.Check(t, func(rt *rapid.T) {
				baseDir, err := os.MkdirTemp("", "detect-stable-")
				if err != nil {
					rt.Fatal(err)
				}
				defer func() { _ = os.RemoveAll(baseDir) }()

				// Plant the canonical signal files.
				for name, content := range sig.files {
					full := filepath.Join(baseDir, name)
					if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
						rt.Fatal(err)
					}
					if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
						rt.Fatal(err)
					}
				}

				baseline := detectFor(rt, baseDir)
				if baseline != sig.provider {
					rt.Fatalf("baseline detect for %s = %q; canonical signal must match",
						sig.provider, baseline)
				}

				// Add 1-5 noise files.
				n := rapid.IntRange(1, 5).Draw(rt, "n-noise")
				for i := 0; i < n; i++ {
					name := genNoiseFilename().Draw(rt, "noise")
					full := filepath.Join(baseDir, name)
					if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
						rt.Fatal(err)
					}
					content := rapid.SampledFrom([]string{
						"", "noise\n", "# comment\n", "lorem ipsum\n",
					}).Draw(rt, "content")
					if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
						rt.Fatal(err)
					}
				}

				after := detectFor(rt, baseDir)
				if after != baseline {
					rt.Fatalf("detection flipped from %q to %q after noise was added",
						baseline, after)
				}
			})
		})
	}
}
