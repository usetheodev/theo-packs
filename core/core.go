package core

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/usetheo/theopacks/core/app"
	c "github.com/usetheo/theopacks/core/config"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/core/providers"
	"github.com/usetheo/theopacks/core/resolver"
	"github.com/usetheo/theopacks/internal/utils"
)

const (
	defaultConfigFileName = "theopacks.json"
)

type GenerateBuildPlanOptions struct {
	TheopacksVersion         string
	BuildCommand             string
	StartCommand             string
	PreviousVersions         map[string]string
	ConfigFilePath           string
	ErrorMissingStartCommand bool
}

type BuildResult struct {
	TheopacksVersion  string                               `json:"theopacksVersion,omitempty"`
	Plan              *plan.BuildPlan                      `json:"plan,omitempty"`
	ResolvedPackages  map[string]*resolver.ResolvedPackage `json:"resolvedPackages,omitempty"`
	Metadata          map[string]string                    `json:"metadata,omitempty"`
	DetectedProviders []string                             `json:"detectedProviders,omitempty"`
	Logs              []logger.Msg                         `json:"logs,omitempty"`
	Success           bool                                 `json:"success,omitempty"`
}

func readConfigJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	jsonBytes, err := utils.StandardizeJSON([]byte(data))
	if err != nil {
		return err
	}

	stringData := string(jsonBytes)

	if err := json.Unmarshal([]byte(stringData), v); err != nil {
		return fmt.Errorf("error reading %s as JSON: %w", path, err)
	}

	return nil
}

func GenerateBuildPlan(app *app.App, env *app.Environment, options *GenerateBuildPlanOptions) *BuildResult {
	log := logger.NewLogger()

	config, err := GetConfig(app, env, options, log)
	if err != nil {
		log.LogError("%s", err.Error())
		return &BuildResult{Success: false, Logs: log.Logs}
	}

	ctx, err := generate.NewGenerateContext(app, env, config, log)
	if err != nil {
		log.LogError("%s", err.Error())
		return &BuildResult{Success: false, Logs: log.Logs}
	}

	if options.PreviousVersions != nil {
		for name, version := range options.PreviousVersions {
			ctx.Resolver.SetPreviousVersion(name, version)
		}
	}

	providerToUse, detectedProviderName := getProviders(ctx, config)
	ctx.Metadata.Set("providers", detectedProviderName)

	if providerToUse != nil {
		err = providerToUse.Plan(ctx)
		if err != nil {
			log.LogError("%s", err.Error())
			return &BuildResult{Success: false, Logs: log.Logs}
		}
	}

	buildPlan, resolvedPackages, err := ctx.Generate()
	if err != nil {
		log.LogError("%s", err.Error())
		return &BuildResult{Success: false, Logs: log.Logs}
	}

	if providerToUse != nil {
		providerToUse.CleansePlan(buildPlan)
	}

	// Stamp the provider name into the plan so the renderer can emit a
	// defensive header comment naming the generator. Detected name is
	// already known from getProviders() above.
	buildPlan.ProviderName = detectedProviderName

	if !ValidatePlan(buildPlan, app, log, &ValidatePlanOptions{
		ErrorMissingStartCommand: options.ErrorMissingStartCommand,
		ProviderToUse:            providerToUse,
	}) {
		return &BuildResult{Success: false, Logs: log.Logs}
	}

	buildResult := &BuildResult{
		TheopacksVersion:  options.TheopacksVersion,
		Plan:              buildPlan,
		ResolvedPackages:  resolvedPackages,
		Metadata:          ctx.Metadata.Properties,
		DetectedProviders: []string{detectedProviderName},
		Logs:              log.Logs,
		Success:           true,
	}

	return buildResult
}

func GetConfig(app *app.App, env *app.Environment, options *GenerateBuildPlanOptions, logger *logger.Logger) (*c.Config, error) {
	optionsConfig := GenerateConfigFromOptions(options)
	envConfig := GenerateConfigFromEnvironment(env)
	fileConfig, err := GenerateConfigFromFile(app, env, options, logger)
	if err != nil {
		return nil, err
	}

	mergedConfig, err := c.Merge(fileConfig, envConfig, optionsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to merge config: %w", err)
	}
	return mergedConfig, nil
}

func GenerateConfigFromFile(app *app.App, env *app.Environment, options *GenerateBuildPlanOptions, logger *logger.Logger) (*c.Config, error) {
	config := c.EmptyConfig()

	configFileName := defaultConfigFileName
	if options.ConfigFilePath != "" {
		configFileName = options.ConfigFilePath
	}

	if envConfigFileName, _ := env.GetConfigVariable("CONFIG_FILE"); envConfigFileName != "" {
		configFileName = envConfigFileName
	}

	absConfigFileName := filepath.Join(app.Source, configFileName)

	if _, err := os.Stat(absConfigFileName); err != nil && os.IsNotExist(err) {
		if configFileName != defaultConfigFileName {
			return nil, fmt.Errorf("config file %q not found", absConfigFileName)
		}
		return config, nil
	}

	if err := readConfigJSON(absConfigFileName, config); err != nil {
		logger.LogWarn("Failed to read config file `%s`", configFileName)
		return nil, err
	}

	logger.LogInfo("Using config file `%s`", configFileName)
	return config, nil
}

func GenerateConfigFromEnvironment(env *app.Environment) *c.Config {
	config := c.EmptyConfig()

	if env == nil {
		return config
	}

	if installCmdVar, _ := env.GetConfigVariable("INSTALL_CMD"); installCmdVar != "" {
		installStep := config.GetOrCreateStep("install")
		installStep.Commands = []plan.Command{
			plan.NewCopyCommand("."),
			plan.NewExecShellCommand(installCmdVar, plan.ExecOptions{CustomName: installCmdVar}),
		}
	}

	if buildCmdVar, _ := env.GetConfigVariable("BUILD_CMD"); buildCmdVar != "" {
		buildStep := config.GetOrCreateStep("build")
		buildStep.Commands = []plan.Command{
			plan.NewCopyCommand("."),
			plan.NewExecShellCommand(buildCmdVar, plan.ExecOptions{CustomName: buildCmdVar}),
		}
	}

	if startCmdVar, _ := env.GetConfigVariable("START_CMD"); startCmdVar != "" {
		config.Deploy.StartCmd = startCmdVar
	}

	if packages, _ := env.GetConfigVariableList("PACKAGES"); len(packages) > 0 {
		config.Packages = utils.ParsePackageWithVersion(packages)
	}

	if aptPackages, _ := env.GetConfigVariableList("BUILD_APT_PACKAGES"); len(aptPackages) > 0 {
		config.BuildAptPackages = aptPackages
	}

	if aptPackages, _ := env.GetConfigVariableList("DEPLOY_APT_PACKAGES"); len(aptPackages) > 0 {
		config.Deploy.AptPackages = aptPackages
	}

	config.Secrets = append(config.Secrets, slices.Sorted(maps.Keys(env.Variables))...)

	return config
}

func GenerateConfigFromOptions(options *GenerateBuildPlanOptions) *c.Config {
	config := c.EmptyConfig()

	if options == nil {
		return config
	}

	if options.BuildCommand != "" {
		buildStep := config.GetOrCreateStep("build")
		buildStep.Commands = []plan.Command{
			plan.NewCopyCommand("."),
			plan.NewExecShellCommand(options.BuildCommand, plan.ExecOptions{CustomName: options.BuildCommand}),
		}
	}

	if options.StartCommand != "" {
		config.Deploy.StartCmd = options.StartCommand
	}

	return config
}

func getProviders(ctx *generate.GenerateContext, config *c.Config) (providers.Provider, string) {
	allProviders := providers.GetLanguageProviders()

	var providerToUse providers.Provider
	var detectedProvider string

	for _, provider := range allProviders {
		matched, err := provider.Detect(ctx)
		if err != nil {
			ctx.Logger.LogWarn("Failed to detect provider `%s`: %s", provider.Name(), err.Error())
			continue
		}

		if matched {
			detectedProvider = provider.Name()

			if config.Provider == nil {
				if err := provider.Initialize(ctx); err != nil {
					ctx.Logger.LogWarn("Failed to initialize provider `%s`: %s", provider.Name(), err.Error())
					continue
				}

				ctx.Logger.LogInfo("Detected %s", utils.CapitalizeFirst(provider.Name()))
				providerToUse = provider
			}

			break
		}
	}

	if config.Provider != nil {
		provider := providers.GetProvider(*config.Provider)

		if provider == nil {
			ctx.Logger.LogWarn("Provider `%s` not found", *config.Provider)
			return providerToUse, detectedProvider
		}

		if err := provider.Initialize(ctx); err != nil {
			ctx.Logger.LogWarn("Failed to initialize provider `%s`: %s", *config.Provider, err.Error())
			return providerToUse, detectedProvider
		}

		ctx.Logger.LogInfo("Using provider %s from config", utils.CapitalizeFirst(*config.Provider))
		providerToUse = provider
	}

	return providerToUse, detectedProvider
}
