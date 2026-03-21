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
	if isUvWorkspace(ctx) {
		return p.planUvWorkspace(ctx)
	}
	if ctx.App.HasFile("requirements.txt") {
		return p.planRequirements(ctx)
	}
	if isPoetryProject(ctx) {
		return p.planPoetry(ctx)
	}
	if ctx.App.HasFile("Pipfile") {
		return p.planPipfile(ctx)
	}
	if ctx.App.HasFile("pyproject.toml") {
		return p.planPyproject(ctx)
	}
	if ctx.App.HasFile("setup.py") {
		return p.planSetupPy(ctx)
	}
	return nil
}

// planRequirements optimizes for requirements.txt with manifest-first caching.
func (p *PythonProvider) planRequirements(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImage))
	installStep.AddCommand(plan.NewCopyCommand("requirements.txt", "./"))
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir -r requirements.txt"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImage)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// planPoetry handles Poetry projects with pyproject.toml containing [tool.poetry].
// Uses poetry install with lock file caching.
func (p *PythonProvider) planPoetry(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImage))

	// Copy manifests first for caching
	installStep.AddCommand(plan.NewCopyCommand("pyproject.toml", "./"))
	if ctx.App.HasFile("poetry.lock") {
		installStep.AddCommand(plan.NewCopyCommand("poetry.lock", "./"))
	}

	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir poetry && poetry config virtualenvs.create false && poetry install --no-interaction --no-ansi"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImage)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// planPipfile handles Pipfile-based projects.
func (p *PythonProvider) planPipfile(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImage))

	// Copy manifests first for caching
	installStep.AddCommand(plan.NewCopyCommand("Pipfile", "./"))
	if ctx.App.HasFile("Pipfile.lock") {
		installStep.AddCommand(plan.NewCopyCommand("Pipfile.lock", "./"))
	}

	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir pipenv && pipenv install --deploy --system"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImage)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// planPyproject handles generic pyproject.toml projects (hatchling, setuptools, etc.).
func (p *PythonProvider) planPyproject(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImage))
	installStep.AddInput(ctx.NewLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir ."))

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImage)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// planSetupPy handles legacy setup.py projects.
func (p *PythonProvider) planSetupPy(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImage))
	installStep.AddInput(ctx.NewLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir ."))

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImage)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// planUvWorkspace handles UV workspace projects.
// UV workspaces have local path deps that need all files at install time.
func (p *PythonProvider) planUvWorkspace(ctx *generate.GenerateContext) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImage))
	installStep.AddInput(ctx.NewLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir uv && uv sync --no-dev"))

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImage)
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// isPoetryProject checks if pyproject.toml contains [tool.poetry].
func isPoetryProject(ctx *generate.GenerateContext) bool {
	if !ctx.App.HasFile("pyproject.toml") {
		return false
	}

	var pyproject struct {
		Tool struct {
			Poetry struct {
				Name string `toml:"name"`
			} `toml:"poetry"`
		} `toml:"tool"`
	}

	if err := ctx.App.ReadTOML("pyproject.toml", &pyproject); err != nil {
		return false
	}

	return pyproject.Tool.Poetry.Name != ""
}

// isUvWorkspace checks if pyproject.toml contains [tool.uv.workspace].
func isUvWorkspace(ctx *generate.GenerateContext) bool {
	if !ctx.App.HasFile("pyproject.toml") {
		return false
	}

	var pyproject struct {
		Tool struct {
			UV struct {
				Workspace struct {
					Members []string `toml:"members"`
				} `toml:"workspace"`
			} `toml:"uv"`
		} `toml:"tool"`
	}

	if err := ctx.App.ReadTOML("pyproject.toml", &pyproject); err != nil {
		return false
	}

	return len(pyproject.Tool.UV.Workspace.Members) > 0
}

func (p *PythonProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *PythonProvider) StartCommandHelp() string {
	return "Specify a start command:\n  - Add a Procfile with: web: python app.py\n  - Or set THEOPACKS_START_CMD=python app.py"
}
