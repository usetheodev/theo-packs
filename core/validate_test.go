package core

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/logger"
	"github.com/usetheo/theopacks/core/plan"
	"github.com/usetheo/theopacks/core/providers"
)

type mockProvider struct {
	providers.Provider
	startCommandHelp string
}

func (m *mockProvider) StartCommandHelp() string {
	return m.startCommandHelp
}

func TestValidatePlan(t *testing.T) {
	log := logger.NewLogger()
	testApp, _ := app.NewApp(".")
	mockProv := &mockProvider{startCommandHelp: "Add a start command"}

	t.Run("valid plan", func(t *testing.T) {
		buildPlan := plan.NewBuildPlan()
		buildStep := plan.NewStep("build")
		buildStep.Commands = []plan.Command{plan.NewExecShellCommand("npm build")}
		buildStep.Inputs = []plan.Layer{plan.NewImageLayer("node:18")}
		buildPlan.Steps = append(buildPlan.Steps, *buildStep)
		buildPlan.Deploy = plan.Deploy{
			Base:     plan.NewImageLayer("node:18"),
			StartCmd: "npm start",
			Inputs:   []plan.Layer{plan.NewStepLayer("build", plan.Filter{Include: []string{"."}})},
		}

		options := &ValidatePlanOptions{
			ErrorMissingStartCommand: true,
			ProviderToUse:            mockProv,
		}
		require.True(t, ValidatePlan(buildPlan, testApp, log, options))
	})
}

func TestValidateCommands(t *testing.T) {
	log := logger.NewLogger()
	testApp, _ := app.NewApp(".")

	t.Run("plan with commands", func(t *testing.T) {
		buildPlan := plan.NewBuildPlan()
		buildStep := plan.NewStep("build")
		buildStep.Commands = []plan.Command{plan.NewExecShellCommand("npm build")}
		buildPlan.Steps = append(buildPlan.Steps, *buildStep)
		require.True(t, validateCommands(buildPlan, testApp, log))
	})

	t.Run("plan without commands", func(t *testing.T) {
		buildPlan := plan.NewBuildPlan()
		buildStep := plan.NewStep("build")
		buildPlan.Steps = append(buildPlan.Steps, *buildStep)
		require.False(t, validateCommands(buildPlan, testApp, log))
	})
}

func TestValidateStartCommand(t *testing.T) {
	mockProv := &mockProvider{startCommandHelp: "Add a start command"}

	t.Run("with start command", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		buildPlan.Deploy = plan.Deploy{
			StartCmd: "npm start",
		}
		options := &ValidatePlanOptions{
			ErrorMissingStartCommand: true,
			ProviderToUse:            mockProv,
		}
		require.True(t, validateStartCommand(buildPlan, log, options))
		require.Equal(t, 0, len(log.Logs))
	})

	t.Run("without start command (error)", func(t *testing.T) {
		loggerInst := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		options := &ValidatePlanOptions{
			ErrorMissingStartCommand: true,
			ProviderToUse:            mockProv,
		}
		require.False(t, validateStartCommand(buildPlan, loggerInst, options))
		require.Equal(t, 1, len(loggerInst.Logs))
		require.Equal(t, logger.Error, loggerInst.Logs[0].Level)
		require.Contains(t, loggerInst.Logs[0].Msg, "No start command detected")
		require.Contains(t, loggerInst.Logs[0].Msg, "Add a start command")
	})

	t.Run("without start command (warning)", func(t *testing.T) {
		loggerInst := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		options := &ValidatePlanOptions{
			ErrorMissingStartCommand: false,
			ProviderToUse:            mockProv,
		}
		require.True(t, validateStartCommand(buildPlan, loggerInst, options))
		require.Equal(t, 1, len(loggerInst.Logs))
		require.Equal(t, logger.Warn, loggerInst.Logs[0].Level)
		require.Contains(t, loggerInst.Logs[0].Msg, "No start command detected")
		require.Contains(t, loggerInst.Logs[0].Msg, "Add a start command")
	})
}

func TestValidateStepNames(t *testing.T) {
	t.Run("empty step name", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		step := plan.NewStep("")
		step.Commands = []plan.Command{plan.NewExecShellCommand("echo hi")}
		buildPlan.Steps = append(buildPlan.Steps, *step)

		require.False(t, validateStepNames(buildPlan, log))
		require.Equal(t, 1, len(log.Logs))
		require.Equal(t, logger.Error, log.Logs[0].Level)
		require.Contains(t, log.Logs[0].Msg, "empty name")
	})

	t.Run("whitespace-only step name", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		step := plan.NewStep("   ")
		step.Commands = []plan.Command{plan.NewExecShellCommand("echo hi")}
		buildPlan.Steps = append(buildPlan.Steps, *step)

		require.False(t, validateStepNames(buildPlan, log))
		require.Equal(t, 1, len(log.Logs))
		require.Contains(t, log.Logs[0].Msg, "empty name")
	})

	t.Run("duplicate step names", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		step1 := plan.NewStep("build")
		step1.Commands = []plan.Command{plan.NewExecShellCommand("echo hi")}
		step2 := plan.NewStep("build")
		step2.Commands = []plan.Command{plan.NewExecShellCommand("echo hi")}
		buildPlan.Steps = append(buildPlan.Steps, *step1, *step2)

		require.False(t, validateStepNames(buildPlan, log))
		require.Equal(t, 1, len(log.Logs))
		require.Equal(t, logger.Error, log.Logs[0].Level)
		require.Contains(t, log.Logs[0].Msg, "duplicate step name")
		require.Contains(t, log.Logs[0].Msg, "build")
	})

	t.Run("valid unique step names", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		step1 := plan.NewStep("install")
		step2 := plan.NewStep("build")
		buildPlan.Steps = append(buildPlan.Steps, *step1, *step2)

		require.True(t, validateStepNames(buildPlan, log))
		require.Equal(t, 0, len(log.Logs))
	})
}

func TestValidateCircularReferences(t *testing.T) {
	t.Run("no circular references", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		installStep := plan.NewStep("install")
		installStep.Inputs = []plan.Layer{plan.NewImageLayer("node:18")}
		buildStep := plan.NewStep("build")
		buildStep.Inputs = []plan.Layer{plan.NewStepLayer("install")}
		buildPlan.Steps = append(buildPlan.Steps, *installStep, *buildStep)

		require.True(t, validateCircularReferences(buildPlan, log))
		require.Equal(t, 0, len(log.Logs))
	})

	t.Run("direct self-reference", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		step := plan.NewStep("build")
		step.Inputs = []plan.Layer{plan.NewStepLayer("build")}
		buildPlan.Steps = append(buildPlan.Steps, *step)

		require.False(t, validateCircularReferences(buildPlan, log))
		require.Equal(t, 1, len(log.Logs))
		require.Equal(t, logger.Error, log.Logs[0].Level)
		require.Contains(t, log.Logs[0].Msg, "circular reference")
	})

	t.Run("indirect circular reference", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		stepA := plan.NewStep("a")
		stepA.Inputs = []plan.Layer{plan.NewStepLayer("b")}
		stepB := plan.NewStep("b")
		stepB.Inputs = []plan.Layer{plan.NewStepLayer("c")}
		stepC := plan.NewStep("c")
		stepC.Inputs = []plan.Layer{plan.NewStepLayer("a")}
		buildPlan.Steps = append(buildPlan.Steps, *stepA, *stepB, *stepC)

		require.False(t, validateCircularReferences(buildPlan, log))
		require.Equal(t, 1, len(log.Logs))
		require.Contains(t, log.Logs[0].Msg, "circular reference")
	})
}

func TestValidateDeployBase(t *testing.T) {
	t.Run("empty deploy base", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		buildPlan.Deploy = plan.Deploy{
			Base: plan.Layer{},
		}

		require.False(t, validateDeployLayers(buildPlan, log))
		require.Equal(t, 1, len(log.Logs))
		require.Equal(t, logger.Error, log.Logs[0].Level)
		require.Contains(t, log.Logs[0].Msg, "deploy.base is required")
	})

	t.Run("deploy base with image", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		buildPlan.Deploy = plan.Deploy{
			Base: plan.NewImageLayer("node:18"),
		}

		require.True(t, validateDeployLayers(buildPlan, log))
		require.Equal(t, 0, len(log.Logs))
	})

	t.Run("deploy base with step", func(t *testing.T) {
		log := logger.NewLogger()
		buildPlan := plan.NewBuildPlan()
		buildPlan.Deploy = plan.Deploy{
			Base: plan.NewStepLayer("build"),
		}

		require.True(t, validateDeployLayers(buildPlan, log))
		require.Equal(t, 0, len(log.Logs))
	})
}

func TestValidateInputs(t *testing.T) {
	log := logger.NewLogger()

	t.Run("valid inputs", func(t *testing.T) {
		inputs := []plan.Layer{
			plan.NewImageLayer("node:18"),
			plan.NewStepLayer("build", plan.Filter{Include: []string{"src"}}),
		}
		require.True(t, validateInputs(inputs, "test", log))
	})

	t.Run("no inputs", func(t *testing.T) {
		inputs := []plan.Layer{}
		require.False(t, validateInputs(inputs, "test", log))
	})

	t.Run("invalid first input", func(t *testing.T) {
		inputs := []plan.Layer{
			plan.NewLocalLayer(),
		}
		require.False(t, validateInputs(inputs, "test", log))
	})

	t.Run("first input with includes", func(t *testing.T) {
		inputs := []plan.Layer{
			plan.NewImageLayer("node:18", plan.Filter{Include: []string{"src"}}),
		}
		require.False(t, validateInputs(inputs, "test", log))
	})
}
