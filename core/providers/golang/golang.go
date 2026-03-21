package golang

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/generate"
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
	if ctx.App.HasFile("go.work") {
		return p.planWorkspace(ctx)
	}
	return p.planSimple(ctx)
}

func (p *GoProvider) planSimple(ctx *generate.GenerateContext) error {
	target := findSimpleBuildTarget(ctx)

	// Install step: copy manifests and download deps (cacheable layer)
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.GoBuildImage))
	installStep.AddCommand(plan.NewCopyCommand("go.mod", "./"))
	if ctx.App.HasFile("go.sum") {
		installStep.AddCommand(plan.NewCopyCommand("go.sum", "./"))
	}
	installStep.AddCommand(plan.NewExecShellCommand("go mod download"))

	// Build step: copy source and compile
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCommand(plan.NewExecShellCommand(fmt.Sprintf("go build -o /app/server %s", target)))

	ctx.Deploy.Base = plan.NewImageLayer(generate.GoRuntimeImage)
	ctx.Deploy.StartCmd = "/app/server"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
	})

	return nil
}

func (p *GoProvider) planWorkspace(ctx *generate.GenerateContext) error {
	modules, err := parseGoWork(ctx.App)
	if err != nil {
		return fmt.Errorf("failed to parse go.work: %w", err)
	}

	target := findBuildTarget(ctx, modules)
	if target == "" {
		return fmt.Errorf("no build target found in go.work — set THEOPACKS_GO_MODULE or add main.go to a module")
	}

	// Install step: copy workspace manifests and download deps
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.GoBuildImage))

	installStep.AddCommand(plan.NewCopyCommand("go.work", "./"))
	if ctx.App.HasFile("go.work.sum") {
		installStep.AddCommand(plan.NewCopyCommand("go.work.sum", "./"))
	}

	for _, mod := range modules {
		goMod := filepath.Join(mod, "go.mod")
		if ctx.App.HasFile(goMod) {
			installStep.AddCommand(plan.NewCopyCommand(goMod, mod+"/"))
		}
		goSum := filepath.Join(mod, "go.sum")
		if ctx.App.HasFile(goSum) {
			installStep.AddCommand(plan.NewCopyCommand(goSum, mod+"/"))
		}
	}

	installStep.AddCommand(plan.NewExecShellCommand("go mod download"))

	// Build step: copy source and compile target
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
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
func parseGoWork(a *app.App) ([]string, error) {
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

func (p *GoProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *GoProvider) StartCommandHelp() string {
	return "Ensure your Go application has a main package."
}
