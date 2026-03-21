package generate

import (
	"github.com/usetheo/theopacks/core/plan"
)

const (
	DefaultRuntimeImage = "debian:bookworm-slim"

	// Build images — official images with full toolchains for compilation
	GoBuildImage     = "golang:1.23-bookworm"
	NodeBuildImage   = "node:20-bookworm"
	PythonBuildImage = "python:3.12-bookworm"

	// Runtime images — slim images for the final deploy stage
	GoRuntimeImage     = DefaultRuntimeImage // Go compiles to static binary
	NodeRuntimeImage   = "node:20-bookworm-slim"
	PythonRuntimeImage = "python:3.12-slim-bookworm"
)

type DeployBuilder struct {
	Base         plan.Layer
	DeployInputs []plan.Layer
	StartCmd     string
	Variables    map[string]string
	Paths        []string
	AptPackages  []string
}

func NewDeployBuilder() *DeployBuilder {
	return &DeployBuilder{
		Base:         plan.NewImageLayer(DefaultRuntimeImage),
		DeployInputs: []plan.Layer{},
		StartCmd:     "",
		Variables:    map[string]string{},
		Paths:        []string{},
		AptPackages:  []string{},
	}
}

func (b *DeployBuilder) SetInputs(layers []plan.Layer) {
	b.DeployInputs = layers
}

func (b *DeployBuilder) AddInputs(layers []plan.Layer) {
	b.DeployInputs = append(b.DeployInputs, layers...)
}

func (b *DeployBuilder) HasIncludeForStep(stepName string, path string) bool {
	for _, layer := range b.DeployInputs {
		if layer.Step != stepName {
			continue
		}
		for _, inc := range layer.Include {
			if inc == path || inc == "." {
				return true
			}
		}
	}
	return false
}

func (b *DeployBuilder) AddAptPackages(packages []string) {
	b.AptPackages = append(b.AptPackages, packages...)
}

func (b *DeployBuilder) Build(p *plan.BuildPlan, options *BuildStepOptions) {
	baseLayer := b.Base

	if len(b.AptPackages) > 0 {
		runtimeAptStep := plan.NewStep("packages:apt:runtime")
		runtimeAptStep.Inputs = []plan.Layer{baseLayer}
		runtimeAptStep.AddCommands([]plan.Command{
			options.NewAptInstallCommand(b.AptPackages),
		})
		runtimeAptStep.Caches = options.Caches.GetAptCaches()
		runtimeAptStep.Secrets = []string{}
		p.Steps = append(p.Steps, *runtimeAptStep)
		baseLayer = plan.NewStepLayer(runtimeAptStep.Name)
	}

	p.Deploy.Base = baseLayer
	p.Deploy.Inputs = append(p.Deploy.Inputs, b.DeployInputs...)
	p.Deploy.StartCmd = b.StartCmd
	p.Deploy.Variables = b.Variables
	p.Deploy.Paths = b.Paths
}
