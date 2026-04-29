package golang

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
)

type GoProvider struct{}

func (p *GoProvider) Name() string {
	return "go"
}

func (p *GoProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("go.mod") || ctx.App.HasFile("go.work"), nil
}

func (p *GoProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *GoProvider) Plan(ctx *generate.GenerateContext) error {
	version, source := detectGoVersion(ctx)

	ref := ctx.Resolver.Default("go", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	if ctx.App.HasFile("go.work") {
		return p.planWorkspace(ctx, version)
	}
	return p.planSimple(ctx, version)
}

func (p *GoProvider) planSimple(ctx *generate.GenerateContext, version string) error {
	target := findSimpleBuildTarget(ctx)

	// Install step: copy manifests and download deps (cacheable layer)
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.GoBuildImageForVersion(version)))
	installStep.AddCacheMount("/go/pkg/mod", "")
	installStep.AddCacheMount("/root/.cache/go-build", "")
	installStep.AddCommand(plan.NewCopyCommand("go.mod", "./"))
	if ctx.App.HasFile("go.sum") {
		installStep.AddCommand(plan.NewCopyCommand("go.sum", "./"))
	}
	installStep.AddCommand(plan.NewExecShellCommand("go mod download"))

	// Build step: copy source and compile
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/go/pkg/mod", "")
	buildStep.AddCacheMount("/root/.cache/go-build", "")
	buildStep.AddCommand(plan.NewExecShellCommand(fmt.Sprintf("go build -o /app/server %s", target)))

	// Go compiles to a static binary, so the runtime image is always debian slim
	ctx.Deploy.Base = plan.NewImageLayer(generate.GoRuntimeImage)
	ctx.Deploy.StartCmd = "/app/server"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
	})

	return nil
}

func (p *GoProvider) planWorkspace(ctx *generate.GenerateContext, version string) error {
	modules, err := parseGoWork(ctx.App, ctx.Logger)
	if err != nil {
		return fmt.Errorf("failed to parse go.work: %w", err)
	}

	target := findBuildTarget(ctx, modules)
	if target == "" {
		return fmt.Errorf("no build target found in go.work — set THEOPACKS_GO_MODULE or add main.go to a module")
	}

	// Install step: copy workspace manifests and download deps
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.GoBuildImageForVersion(version)))
	installStep.AddCacheMount("/go/pkg/mod", "")
	installStep.AddCacheMount("/root/.cache/go-build", "")

	installStep.AddCommand(plan.NewCopyCommand("go.work", "./"))
	if ctx.App.HasFile("go.work.sum") {
		installStep.AddCommand(plan.NewCopyCommand("go.work.sum", "./"))
	}

	hasExternalDeps := false
	for _, mod := range modules {
		goMod := filepath.Join(mod, "go.mod")
		if ctx.App.HasFile(goMod) {
			installStep.AddCommand(plan.NewCopyCommand(goMod, mod+"/"))
		}
		goSum := filepath.Join(mod, "go.sum")
		if ctx.App.HasFile(goSum) {
			installStep.AddCommand(plan.NewCopyCommand(goSum, mod+"/"))
			hasExternalDeps = true
		}
	}

	// Only run go mod download when modules have external dependencies.
	// In a Go workspace, modules that only depend on each other (resolved
	// via go.work) and stdlib have no go.sum files and nothing to download.
	// Running go mod download in that case fails because workspace-local
	// module paths (e.g. example.com/project/pkg/shared) are not resolvable
	// as remote URLs.
	if hasExternalDeps {
		installStep.AddCommand(plan.NewExecShellCommand("go mod download"))
	}

	// Build step: copy source and compile target
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCacheMount("/go/pkg/mod", "")
	buildStep.AddCacheMount("/root/.cache/go-build", "")
	buildStep.AddCommand(plan.NewExecShellCommand(fmt.Sprintf("go build -o /app/server ./%s", target)))

	ctx.Deploy.Base = plan.NewImageLayer(generate.GoRuntimeImage)
	ctx.Deploy.StartCmd = "/app/server"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
	})

	return nil
}

// findSimpleBuildTarget determines the build target for a simple Go project.
// Checks: root main.go → cmd/*/main.go → "."
func findSimpleBuildTarget(ctx *generate.GenerateContext) string {
	if ctx.App.HasFile("main.go") {
		return "."
	}

	// Look for cmd/*/main.go pattern
	cmdMains, err := ctx.App.FindFiles("cmd/*/main.go")
	if err == nil && len(cmdMains) > 0 {
		// Use the first cmd directory found
		dir := filepath.Dir(cmdMains[0])
		return "./" + dir
	}

	return "."
}

// parseGoWork reads go.work and extracts module paths from the use (...) block.
func parseGoWork(a *app.App, log ...*logger.Logger) ([]string, error) {
	// Extract optional logger
	var l *logger.Logger
	if len(log) > 0 {
		l = log[0]
	}

	content, err := a.ReadFile("go.work")
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`use\s*\(\s*\n([\s\S]*?)\)`)
	match := re.FindStringSubmatch(content)
	if match == nil {
		singleRe := regexp.MustCompile(`use\s+(\S+)`)
		singleMatch := singleRe.FindStringSubmatch(content)
		if singleMatch != nil {
			return []string{strings.TrimPrefix(singleMatch[1], "./")}, nil
		}
		if l != nil {
			l.LogWarn("go.work file has no valid use directive — file may be malformed")
		}
		return nil, fmt.Errorf("no use directive found in go.work")
	}

	var modules []string
	for _, line := range strings.Split(match[1], "\n") {
		mod := strings.TrimSpace(line)
		mod = strings.TrimPrefix(mod, "./")
		if mod != "" {
			modules = append(modules, mod)
		}
	}

	if len(modules) == 0 {
		if l != nil {
			l.LogWarn("go.work has use () block but no modules listed inside it")
		}
		return nil, fmt.Errorf("go.work has empty use block — no modules declared")
	}

	return modules, nil
}

// findBuildTarget determines which module to build.
// Priority: THEOPACKS_GO_MODULE env var > first module with main.go
func findBuildTarget(ctx *generate.GenerateContext, modules []string) string {
	if target := ctx.Env.GetVariable("THEOPACKS_GO_MODULE"); target != "" {
		return target
	}

	for _, mod := range modules {
		mainFile := filepath.Join(mod, "main.go")
		if ctx.App.HasFile(mainFile) {
			return mod
		}
	}

	return ""
}

// detectGoVersion determines the Go version to use for the build image.
// Priority: config packages > THEOPACKS_GO_VERSION env var > go.mod directive > default.
func detectGoVersion(ctx *generate.GenerateContext) (version string, source string) {
	// Config packages have highest priority (set via theopacks.json or THEOPACKS_PACKAGES)
	if pkg := ctx.Resolver.Get("go"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajorMinor(pkg.Version), pkg.Source
	}

	// Environment variable
	if envVersion, varName := ctx.Env.GetConfigVariable("GO_VERSION"); envVersion != "" {
		return generate.NormalizeToMajorMinor(envVersion), varName
	}

	// go.mod "go" directive
	if ctx.App.HasFile("go.mod") {
		if v := extractGoVersionFromMod(ctx); v != "" {
			return v, "go.mod"
		}
	}

	return generate.DefaultGoVersion, "default"
}

// extractGoVersionFromMod reads the "go X.XX" directive from go.mod.
func extractGoVersionFromMod(ctx *generate.GenerateContext) string {
	content, err := ctx.App.ReadFile("go.mod")
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)
	match := re.FindStringSubmatch(content)
	if match == nil {
		return ""
	}

	return match[1]
}

func (p *GoProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *GoProvider) StartCommandHelp() string {
	return "Ensure your Go application has a main package."
}
