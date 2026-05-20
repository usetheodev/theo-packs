// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 The Theo Authors

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// clampPath resolves `sub` relative to `root` and refuses to return a path
// that escapes `root`. Defense against `--app-path=../../etc` and similar
// path-traversal payloads.
//
// `filepath.Join(root, sub)` on its own does NOT prevent escape — Go's
// docs are explicit about this. The canonical fix is filepath.Abs +
// prefix check, which is what we do here.
//
// Invariant: for any (p, nil) returned, either p == rootAbs OR
// strings.HasPrefix(p+sep, rootAbs+sep) holds.
func clampPath(root, sub string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", root, err)
	}

	// `filepath.IsAbs(sub)` is a separate failure mode: an absolute sub
	// path silently overrides the root, which is exactly the traversal
	// vector we're closing.
	if filepath.IsAbs(sub) {
		return "", fmt.Errorf("path %q is absolute; must be relative to source root", sub)
	}

	joined := filepath.Join(rootAbs, sub)
	sep := string(filepath.Separator)

	if joined != rootAbs && !strings.HasPrefix(joined+sep, rootAbs+sep) {
		return "", fmt.Errorf("path %q escapes source root %q", sub, rootAbs)
	}
	return joined, nil
}
