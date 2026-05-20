// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package generate

// WorkspaceTarget identifies which member of a monorepo / workspace the
// caller wants built. Set by the CLI from --app-name / --app-path; left
// nil for single-app projects.
//
// Typed alternative to the historical env-var bridge
// (THEOPACKS_APP_NAME / THEOPACKS_APP_PATH), introduced in T3.1 (see
// docs/plans/deep-review-hardening-plan.md) to reduce string coupling
// between CLI and providers and to make the `--mount=type=secret`
// filter (T1.4) unnecessary as a workaround.
//
// Backward compat: when (*GenerateContext).WorkspaceTarget is nil,
// providers fall back to the env vars. The CLI populates both during
// the deprecation window.
type WorkspaceTarget struct {
	// AppName is the workspace-member identifier, e.g. "api" or
	// "@scope/api" (npm scoped). Empty means "no specific member".
	AppName string
	// AppPath is the relative path from the workspace root to the
	// member's source dir, e.g. "apps/api". Empty or "." means "root".
	AppPath string
}

// ResolveAppName returns the workspace member name, preferring the
// typed WorkspaceTarget over the legacy env-var bridge. Empty string
// when neither is set.
func (c *GenerateContext) ResolveAppName() string {
	if c.WorkspaceTarget != nil && c.WorkspaceTarget.AppName != "" {
		return c.WorkspaceTarget.AppName
	}
	v, _ := c.Env.GetConfigVariable("APP_NAME")
	return v
}

// ResolveAppPath returns the workspace member path, preferring the
// typed WorkspaceTarget over the legacy env-var bridge. Empty string
// when neither is set.
func (c *GenerateContext) ResolveAppPath() string {
	if c.WorkspaceTarget != nil && c.WorkspaceTarget.AppPath != "" {
		return c.WorkspaceTarget.AppPath
	}
	v, _ := c.Env.GetConfigVariable("APP_PATH")
	return v
}
