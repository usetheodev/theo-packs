package providers

import (
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/core/providers/deno"
	"github.com/usetheo/theopacks/core/providers/dotnet"
	"github.com/usetheo/theopacks/core/providers/golang"
	"github.com/usetheo/theopacks/core/providers/java"
	"github.com/usetheo/theopacks/core/providers/node"
	"github.com/usetheo/theopacks/core/providers/php"
	"github.com/usetheo/theopacks/core/providers/python"
	"github.com/usetheo/theopacks/core/providers/ruby"
	"github.com/usetheo/theopacks/core/providers/rust"
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

// GetLanguageProviders returns the registered providers in DETECTION ORDER.
// First-match-wins via Provider.Detect(); the order matters where manifests
// can collide (Deno before Node so a Deno project shipping a npm-compat
// package.json routes to Deno, not Node).
func GetLanguageProviders() []Provider {
	return []Provider{
		&golang.GoProvider{},
		&rust.RustProvider{},
		&java.JavaProvider{},
		&dotnet.DotnetProvider{},
		&ruby.RubyProvider{},
		&php.PhpProvider{},
		&python.PythonProvider{},
		&deno.DenoProvider{},
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
