package providers

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/core/providers/golang"
	"github.com/usetheo/theopacks/core/providers/node"
	"github.com/usetheo/theopacks/core/providers/python"
	"github.com/usetheo/theopacks/core/providers/shell"
	"github.com/usetheo/theopacks/core/providers/staticfile"
)

type Provider interface {
	Name() string
	Detect(ctx *generate.GenerateContext) (bool, error)
	Initialize(ctx *generate.GenerateContext) error
	Plan(ctx *generate.GenerateContext) error
	CleansePlan(buildPlan *plan.BuildPlan)
	StartCommandHelp() string
}

func GetLanguageProviders() []Provider {
	return []Provider{
		&golang.GoProvider{},
		&python.PythonProvider{},
		&node.NodeProvider{},
		&staticfile.StaticfileProvider{},
		&shell.ShellProvider{},
	}
}

func GetProvider(name string) Provider {
	for _, provider := range GetLanguageProviders() {
		if provider.Name() == name {
			return provider
		}
	}
	return nil
}
