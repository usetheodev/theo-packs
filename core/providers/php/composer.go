// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors
// Portions derived from github.com/railwayapp/railpack (Apache-2.0).

package php

import (
	"github.com/usetheo/theopacks/core/app"
)

// ComposerJson is the subset of composer.json fields theo-packs needs for
// language detection, version resolution, and framework detection.
type ComposerJson struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Require map[string]string `json:"require"`
	Scripts map[string]any    `json:"scripts"`
}

func parseComposer(a *app.App) (*ComposerJson, error) {
	var c ComposerJson
	if err := a.ReadJSON("composer.json", &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// HasPackage reports whether the named package is in `require`. Useful for
// framework detection (e.g., laravel/framework, slim/slim).
func (c *ComposerJson) HasPackage(name string) bool {
	if c == nil {
		return false
	}
	_, ok := c.Require[name]
	return ok
}
