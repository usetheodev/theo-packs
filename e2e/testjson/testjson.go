// Package testjson defines and parses the `test.json` config that
// lives alongside each example under examples/<name>/. The format is
// inspired by Railpack's per-example test descriptor — see
// docs/plans/robust-test-suite-plan.md T1.4 / T1.5 for context.
//
// Schema (JSONC — comments allowed):
//
//	{
//	  // "mode": one of "httpCheck" | "justBuild" | "expectedOutput"
//	  // Defaults to "justBuild" when omitted.
//	  "mode": "httpCheck",
//
//	  // When mode == "httpCheck", validate the running container via
//	  // an HTTP probe. Port may be a literal or the special token
//	  // "$PORT" — the runner picks a host port and substitutes.
//	  "httpCheck": { "path": "/health", "port": 8080, "expectedStatus": 200 },
//
//	  // When mode == "expectedOutput", run the image and grep stdout.
//	  "expectedOutput": { "args": ["--version"], "mustContain": "v1." },
//
//	  // Path (relative to the example dir) to a structure-tests.yaml.
//	  // Empty = no declarative structure test (T1.1 wrapper is no-op).
//	  "structureTest": "structure-tests.yaml",
//
//	  // Env vars passed to theopacks-generate (provider-scoped).
//	  "env": { "THEOPACKS_APP_NAME": "api" },
//
//	  // Mark the case skipped without removing it from the corpus.
//	  "skip": false,
//	  "skipReason": ""
//	}
package testjson

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/usetheo/theopacks/internal/utils"
)

// Mode names a validation strategy for the post-build phase.
type Mode string

const (
	ModeJustBuild      Mode = "justBuild"      // build succeeded → pass
	ModeHTTPCheck      Mode = "httpCheck"      // run + HTTP probe
	ModeExpectedOutput Mode = "expectedOutput" // run + stdout grep
)

// HTTPCheck describes a single endpoint probe.
type HTTPCheck struct {
	Path           string `json:"path"`
	Port           int    `json:"port"`
	ExpectedStatus int    `json:"expectedStatus"`
}

// ExpectedOutput describes a run-and-grep validation.
type ExpectedOutput struct {
	Args        []string `json:"args"`
	MustContain string   `json:"mustContain"`
}

// Config is the parsed test.json.
type Config struct {
	Mode           Mode              `json:"mode,omitempty"`
	HTTPCheck      *HTTPCheck        `json:"httpCheck,omitempty"`
	ExpectedOutput *ExpectedOutput   `json:"expectedOutput,omitempty"`
	StructureTest  string            `json:"structureTest,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Skip           bool              `json:"skip,omitempty"`
	SkipReason     string            `json:"skipReason,omitempty"`
}

// Parse reads bytes, strips JSONC comments via the existing hujson
// pipeline, and returns the structured Config. Defaults Mode to
// "justBuild" when unset; otherwise validates the value.
func Parse(data []byte) (*Config, error) {
	jsonBytes, err := utils.StandardizeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("standardize jsonc: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal test.json: %w", err)
	}
	if cfg.Mode == "" {
		cfg.Mode = ModeJustBuild
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Load reads a test.json file from path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data)
}

// Validate checks intra-config invariants. Caller responsibility:
// example files actually exist at declared paths.
func (c *Config) Validate() error {
	switch c.Mode {
	case ModeJustBuild:
		// No additional fields required.
	case ModeHTTPCheck:
		if c.HTTPCheck == nil {
			return fmt.Errorf("mode httpCheck requires httpCheck block")
		}
		if c.HTTPCheck.Path == "" {
			return fmt.Errorf("httpCheck.path is required")
		}
		if c.HTTPCheck.Port <= 0 {
			return fmt.Errorf("httpCheck.port must be > 0, got %d", c.HTTPCheck.Port)
		}
		if c.HTTPCheck.ExpectedStatus < 100 || c.HTTPCheck.ExpectedStatus > 599 {
			return fmt.Errorf("httpCheck.expectedStatus must be valid HTTP code, got %d",
				c.HTTPCheck.ExpectedStatus)
		}
	case ModeExpectedOutput:
		if c.ExpectedOutput == nil {
			return fmt.Errorf("mode expectedOutput requires expectedOutput block")
		}
		if len(c.ExpectedOutput.Args) == 0 {
			return fmt.Errorf("expectedOutput.args must not be empty")
		}
		if c.ExpectedOutput.MustContain == "" {
			return fmt.Errorf("expectedOutput.mustContain must not be empty")
		}
	default:
		return fmt.Errorf("unknown mode %q (allowed: justBuild, httpCheck, expectedOutput)", c.Mode)
	}
	return nil
}
