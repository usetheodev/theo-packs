package core

import (
	"fmt"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/core/providers"
	"github.com/usetheo/theopacks/internal/utils"
)

type ValidatePlanOptions struct {
	ErrorMissingStartCommand bool
	ProviderToUse            providers.Provider
}

func ValidatePlan(p *plan.BuildPlan, app *app.App, logger *logger.Logger, options *ValidatePlanOptions) bool {
	if !validateCommands(p, app, logger) {
		return false
	}

	if !validateStartCommand(p, logger, options) {
		return false
	}

	for _, step := range p.Steps {
		if !validateInputs(step.Inputs, step.Name, logger) {
			return false
		}
	}

	return validateDeployLayers(p, logger)
}

func validateCommands(p *plan.BuildPlan, app *app.App, logger *logger.Logger) bool {
	var atLeastOneCommand = false
	for _, step := range p.Steps {
		if len(step.Commands) > 0 {
			atLeastOneCommand = true
		}
	}

	if !atLeastOneCommand {
		logger.LogError("%s", getNoProviderError(app))
		return false
	}

	return true
}

func validateStartCommand(p *plan.BuildPlan, logger *logger.Logger, options *ValidatePlanOptions) bool {
	if p.Deploy.StartCmd != "" {
		return true
	}

	msg := "No start command detected. Specify a start command."
	if options.ProviderToUse != nil {
		if providerHelp := options.ProviderToUse.StartCommandHelp(); providerHelp != "" {
			msg += "\n\n" + providerHelp
		}
	}

	if options.ErrorMissingStartCommand {
		logger.LogError("%s", msg)
		return false
	}

	logger.LogWarn("%s", msg)
	return true
}

func validateInputs(inputs []plan.Layer, stepName string, logger *logger.Logger) bool {
	if len(inputs) == 0 {
		logger.LogError("step `%s` has no inputs", stepName)
		return false
	}

	firstInput := inputs[0]
	if firstInput.Image == "" && firstInput.Step == "" {
		logger.LogError("`%s` inputs must be an image or step input\n\n%s", stepName, firstInput.String())
		return false
	}

	if len(firstInput.Include) > 0 || len(firstInput.Exclude) > 0 {
		logger.LogError("the first input of `%s` cannot have any includes or excludes.\n\n%s", stepName, firstInput.String())
		return false
	}

	return true
}

func validateDeployLayers(p *plan.BuildPlan, logger *logger.Logger) bool {
	if p.Deploy.Base.Image == "" && p.Deploy.Base.Step == "" {
		logger.LogError("deploy.base is required")
		return false
	}

	return true
}

func getNoProviderError(app *app.App) string {
	providerNames := []string{}
	for _, provider := range providers.GetLanguageProviders() {
		providerNames = append(providerNames, utils.CapitalizeFirst(provider.Name()))
	}

	files, _ := app.FindFiles("*")
	dirs, _ := app.FindDirectories("*")

	fileTree := "./\n"

	for i, dir := range dirs {
		prefix := "├── "
		if i == len(dirs)-1 && len(files) == 0 {
			prefix = "└── "
		}
		fileTree += fmt.Sprintf("%s%s/\n", prefix, dir)
	}

	for i, file := range files {
		prefix := "├── "
		if i == len(files)-1 {
			prefix = "└── "
		}
		fileTree += fmt.Sprintf("%s%s\n", prefix, file)
	}

	errorMsg := "Theopacks could not determine how to build the app.\n\n"
	errorMsg += "The following languages are supported:\n"
	for _, provider := range providerNames {
		errorMsg += fmt.Sprintf("- %s\n", provider)
	}

	errorMsg += "\nThe app contents that Theopacks analyzed:\n\n"
	errorMsg += fileTree

	return errorMsg
}
