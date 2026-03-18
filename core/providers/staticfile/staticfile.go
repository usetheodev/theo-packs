package staticfile

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type StaticfileProvider struct{}

func (p *StaticfileProvider) Name() string {
	return "staticfile"
}

func (p *StaticfileProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("index.html"), nil
}

func (p *StaticfileProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *StaticfileProvider) Plan(ctx *generate.GenerateContext) error {
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewImageLayer(generate.DefaultRuntimeImage))
	buildStep.AddInput(ctx.NewLocalLayer())
	buildStep.AddCommand(plan.NewCopyCommand("."))

	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

func (p *StaticfileProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *StaticfileProvider) StartCommandHelp() string {
	return "Static files detected. Configure a web server to serve them."
}
