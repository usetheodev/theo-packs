package generate

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	a "github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/core/resolver"
	"github.com/usetheo/theopacks/internal/utils"
)

type BuildStepOptions struct {
	ResolvedPackages map[string]*resolver.ResolvedPackage
	Caches           *CacheContext
}

type StepBuilder interface {
	Name() string
	Build(p *plan.BuildPlan, options *BuildStepOptions) error
}

type GenerateContext struct {
	App             *a.App
	Env             *a.Environment
	Config          *config.Config
	dockerignoreCtx *plan.DockerignoreContext

	Steps  []StepBuilder
	Deploy *DeployBuilder

	Caches  *CacheContext
	Secrets []string

	SubContexts []string

	Metadata *Metadata
	Resolver *resolver.Resolver
	Logger   *logger.Logger
}

func NewGenerateContext(app *a.App, env *a.Environment, cfg *config.Config, log *logger.Logger) (*GenerateContext, error) {
	dockerignoreCtx, err := plan.NewDockerignoreContext(app)
	if err != nil {
		return nil, fmt.Errorf("failed to parse .dockerignore: %w", err)
	}

	if dockerignoreCtx.HasFile {
		log.LogInfo("Found .dockerignore file, applying filters")
	}

	ctx := &GenerateContext{
		App:             app,
		Env:             env,
		Config:          cfg,
		Steps:           make([]StepBuilder, 0),
		Deploy:          NewDeployBuilder(),
		Caches:          NewCacheContext(),
		Secrets:         []string{},
		Metadata:        NewMetadata(),
		Resolver:        resolver.NewResolver(),
		Logger:          log,
		dockerignoreCtx: dockerignoreCtx,
	}

	ctx.applyPackagesFromConfig()

	if dockerignoreCtx.HasFile {
		ctx.Metadata.SetBool("dockerIgnore", true)
	}

	return ctx, nil
}

func (c *GenerateContext) EnterSubContext(subContext string) *GenerateContext {
	c.SubContexts = append(c.SubContexts, subContext)
	return c
}

func (c *GenerateContext) ExitSubContext() *GenerateContext {
	c.SubContexts = c.SubContexts[:len(c.SubContexts)-1]
	return c
}

func (c *GenerateContext) GetStepName(name string) string {
	subContextNames := strings.Join(c.SubContexts, ":")
	if subContextNames != "" {
		return name + ":" + subContextNames
	}
	return name
}

func (c *GenerateContext) GetStepByName(name string) *StepBuilder {
	for _, step := range c.Steps {
		if step.Name() == name {
			return &step
		}
	}
	return nil
}

func (c *GenerateContext) ResolvePackages() (map[string]*resolver.ResolvedPackage, error) {
	return c.Resolver.ResolvePackages()
}

// Generate creates a build plan from the context
func (c *GenerateContext) Generate() (*plan.BuildPlan, map[string]*resolver.ResolvedPackage, error) {
	c.applyConfig()

	resolvedPackages, err := c.ResolvePackages()
	if err != nil {
		return nil, nil, err
	}

	buildPlan := plan.NewBuildPlan()

	buildStepOptions := &BuildStepOptions{
		ResolvedPackages: resolvedPackages,
		Caches:           c.Caches,
	}

	for _, stepBuilder := range c.Steps {
		err := stepBuilder.Build(buildPlan, buildStepOptions)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build step: %w", err)
		}
	}

	buildPlan.Caches = c.Caches.Caches
	buildPlan.Secrets = utils.RemoveDuplicates(c.Secrets)
	c.Deploy.Build(buildPlan, buildStepOptions)

	buildPlan.Normalize()

	return buildPlan, resolvedPackages, nil
}

func (o *BuildStepOptions) NewAptInstallCommand(pkgs []string) plan.Command {
	pkgs = utils.RemoveDuplicates(pkgs)
	sort.Strings(pkgs)

	return plan.NewExecCommand("sh -c 'apt-get update && apt-get install -y "+strings.Join(pkgs, " ")+"'", plan.ExecOptions{
		CustomName: "install apt packages: " + strings.Join(pkgs, " "),
	})
}

func (c *GenerateContext) applyPackagesFromConfig() {
	for _, pkg := range slices.Sorted(maps.Keys(c.Config.Packages)) {
		version := c.Config.Packages[pkg]
		pkgRef := c.Resolver.Default(pkg, version)
		c.Resolver.Version(pkgRef, version, "custom config")
	}
}

func (c *GenerateContext) applyConfig() {
	c.applyPackagesFromConfig()

	maps.Copy(c.Caches.Caches, c.Config.Caches)
	c.Secrets = plan.SpreadStrings(c.Config.Secrets, c.Secrets)

	if c.Config.Deploy != nil {
		if c.Config.Deploy.StartCmd != "" {
			c.Deploy.StartCmd = c.Config.Deploy.StartCmd
		}

		c.Deploy.AptPackages = plan.SpreadStrings(c.Config.Deploy.AptPackages, c.Deploy.AptPackages)
		c.Deploy.DeployInputs = plan.Spread(c.Config.Deploy.Inputs, c.Deploy.DeployInputs)
		c.Deploy.Paths = plan.SpreadStrings(c.Config.Deploy.Paths, c.Deploy.Paths)
		maps.Copy(c.Deploy.Variables, c.Config.Deploy.Variables)
	}

	for _, name := range slices.Sorted(maps.Keys(c.Config.Steps)) {
		configStep := c.Config.Steps[name]

		var commandStepBuilder *CommandStepBuilder

		if existingStep := c.GetStepByName(name); existingStep != nil {
			if csb, ok := (*existingStep).(*CommandStepBuilder); ok {
				commandStepBuilder = csb
			} else {
				continue
			}
		} else {
			commandStepBuilder = c.NewCommandStep(name)
		}

		commandStepBuilder.Inputs = plan.Spread(configStep.Inputs, commandStepBuilder.Inputs)
		commandStepBuilder.Commands = plan.Spread(configStep.Commands, commandStepBuilder.Commands)
		commandStepBuilder.Secrets = plan.SpreadStrings(configStep.Secrets, commandStepBuilder.Secrets)
		commandStepBuilder.Caches = plan.SpreadStrings(configStep.Caches, commandStepBuilder.Caches)
		commandStepBuilder.AddEnvVars(configStep.Variables)
		maps.Copy(commandStepBuilder.Assets, configStep.Assets)

		outputFilters := []plan.Filter{plan.NewIncludeFilter([]string{"."})}
		if configStep.DeployOutputs != nil {
			outputFilters = configStep.DeployOutputs
		}
		for _, filter := range outputFilters {
			alreadyCovered := false
			for _, inc := range filter.Include {
				if c.Deploy.HasIncludeForStep(name, inc) {
					alreadyCovered = true
					break
				}
			}
			if !alreadyCovered {
				c.Deploy.AddInputs([]plan.Layer{plan.NewStepLayer(name, filter)})
			}
		}
	}
}

// NewLocalLayer creates a local layer with dockerignore patterns applied
func (c *GenerateContext) NewLocalLayer() plan.Layer {
	layer := plan.NewLocalLayer()

	if len(c.dockerignoreCtx.Includes) > 0 {
		layer.Include = append(layer.Include, c.dockerignoreCtx.Includes...)
	}
	if len(c.dockerignoreCtx.Excludes) > 0 {
		layer.Exclude = append(layer.Exclude, c.dockerignoreCtx.Excludes...)
	}

	return layer
}

func (c *GenerateContext) GetAppSource() string {
	return c.App.Source
}

func (c *GenerateContext) GetLogger() *logger.Logger {
	return c.Logger
}
