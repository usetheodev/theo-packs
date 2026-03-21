package golang

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type GoProvider struct{}

func (p *GoProvider) Name() string {
	return "go"
}

func (p *GoProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("go.mod"), nil
}

func (p *GoProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *GoProvider) Plan(ctx *generate.GenerateContext) error {
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewImageLayer(generate.GoBuildImage))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCommand(plan.NewExecShellCommand("go build -o /app/server ."))

	ctx.Deploy.Base = plan.NewImageLayer(generate.GoRuntimeImage)
	ctx.Deploy.StartCmd = "/app/server"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"/app/server"}}),
	})

	return nil
}

func (p *GoProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *GoProvider) StartCommandHelp() string {
	return "Ensure your Go application has a main package."
}
