package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// Shared test binary infrastructure (T2.1, test-suite-hardening-plan).
//
// Every subprocess-based test in this package used to invoke
// `buildBinary(t)`, which called `go build` per test — adding ~500ms
// each and ~10s to the full suite. TestMain compiles the binary once
// and exposes it via sharedBinary; buildBinary now returns that path.
//
// Synchronization: TestMain runs before any test, and the test runtime
// joins before TestMain returns, so plain assignment is safe. We still
// guard with sync.Once in case a test calls buildBinary outside a
// normal Test* function (e.g., from an init() in another _test.go).

var (
	sharedBinary string
	sharedOnce   sync.Once
	sharedDir    string
)

func TestMain(m *testing.M) {
	sharedOnce.Do(func() {
		dir, err := os.MkdirTemp("", "theopacks-binary-")
		if err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: mktemp failed: %v\n", err)
			os.Exit(1)
		}
		sharedDir = dir
		sharedBinary = filepath.Join(dir, "theopacks-generate")

		cmd := exec.Command("go", "build", "-o", sharedBinary, ".")
		cmd.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "TestMain: go build failed: %v\n%s\n", err, string(out))
			os.Exit(1)
		}
	})

	code := m.Run()

	if sharedDir != "" {
		_ = os.RemoveAll(sharedDir)
	}
	os.Exit(code)
}
