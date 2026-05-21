// Package theoyaml parses a `theo.yaml` descriptor — the per-template
// config that theo-stacks ships alongside every scaffold template. The
// runtime gate (T1.1, theo-stacks-build-and-run-plan) consumes this to
// know which port to probe and whether the app is a server / worker /
// frontend.
//
// Spec mirror: every template in https://github.com/usetheodev/theo-stacks
// follows the shape below. We intentionally parse only the fields the
// gate needs, ignoring extras so future schema additions don't break us.
package theoyaml

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// AppType enumerates the runtime shapes the gate cares about.
type AppType string

const (
	TypeServer   AppType = "server"
	TypeWorker   AppType = "worker"
	TypeFrontend AppType = "frontend"
)

// AppConfig is the subset of `apps.<name>` we consume.
type AppConfig struct {
	Path      string  `yaml:"path"`
	Framework string  `yaml:"framework"`
	Type      AppType `yaml:"type"`
	Port      int     `yaml:"port"`
}

// Config is the top-level theo.yaml document.
type Config struct {
	Version int                  `yaml:"version"`
	Project string               `yaml:"project"`
	Apps    map[string]AppConfig `yaml:"apps"`
}

// ParseFile reads + parses a theo.yaml at the given path. Returns an
// error when the file is missing, malformed YAML, or empty.
func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read theo.yaml: %w", err)
	}
	return Parse(data)
}

// Parse decodes the YAML bytes into Config. Defaults Type to
// TypeServer when omitted (the most common shape in theo-stacks).
func Parse(data []byte) (*Config, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("theo.yaml is empty")
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal theo.yaml: %w", err)
	}
	for name, app := range cfg.Apps {
		if app.Type == "" {
			app.Type = TypeServer
			cfg.Apps[name] = app
		}
	}
	return &cfg, nil
}

// App returns the AppConfig for the given app name. Empty name + single
// app present → returns that app (convenience for single-app templates).
func (c *Config) App(name string) (AppConfig, bool) {
	if name == "" {
		if len(c.Apps) == 1 {
			for _, a := range c.Apps {
				return a, true
			}
		}
		return AppConfig{}, false
	}
	a, ok := c.Apps[name]
	return a, ok
}
