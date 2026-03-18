package python

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type PythonProvider struct{}

func (p *PythonProvider) Name() string {
	return "python"
}

func (p *PythonProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("requirements.txt") ||
		ctx.App.HasFile("pyproject.toml") ||
		ctx.App.HasFile("Pipfile") ||
		ctx.App.HasFile("setup.py"), nil
}

func (p *PythonProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *PythonProvider) Plan(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.DefaultRuntimeImage))
	installStep.AddInput(ctx.NewLocalLayer())

	if ctx.App.HasFile("requirements.txt") {
		installStep.AddCommand(plan.NewExecShellCommand("pip install -r requirements.txt"))
	} else if ctx.App.HasFile("pyproject.toml") {
		installStep.AddCommand(plan.NewExecShellCommand("pip install ."))
	}

	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

func (p *PythonProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *PythonProvider) StartCommandHelp() string {
	return "Specify a start command:\n  - Add a Procfile with: web: python app.py\n  - Or set THEOPACKS_START_CMD=python app.py"
}
