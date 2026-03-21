package node

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

type NodeProvider struct{}

func (p *NodeProvider) Name() string {
	return "node"
}

func (p *NodeProvider) Detect(ctx *generate.GenerateContext) (bool, error) {
	return ctx.App.HasFile("package.json"), nil
}

func (p *NodeProvider) Initialize(ctx *generate.GenerateContext) error {
	return nil
}

func (p *NodeProvider) Plan(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.NodeBuildImage))
	installStep.AddInput(ctx.NewLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("npm install"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddCommand(plan.NewCopyCommand("."))

	ctx.Deploy.Base = plan.NewImageLayer(generate.NodeRuntimeImage)
	ctx.Deploy.StartCmd = "npm start"
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

func (p *NodeProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *NodeProvider) StartCommandHelp() string {
	return "Add a \"start\" script to your package.json, e.g.:\n  \"scripts\": { \"start\": \"node server.js\" }"
}
