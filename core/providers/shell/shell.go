package shell

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type ShellProvider struct{}

func (p *ShellProvider) Name() string {
	return "shell"
}

func (p *ShellProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	files, err := ctx.App.FindFiles("*.sh")
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

func (p *ShellProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *ShellProvider) Plan(ctx *generate.GenerateContext) error {
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewImageLayer(generate.DefaultRuntimeImage))
	buildStep.AddCommand(plan.NewCopyCommand(".", "./"))

	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

func (p *ShellProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *ShellProvider) StartCommandHelp() string {
	return "Specify a start command with THEOPACKS_START_CMD environment variable."
}
