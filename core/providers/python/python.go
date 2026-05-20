package python

import (
	"fmt"
	"strings"

	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/plan"
)

// pythonDeployIncludes returns paths that must be copied from the build stage
// to the deploy stage for Python apps. The site-packages path is version-specific
// because the official Python Docker images install packages under /usr/local/lib/pythonX.Y/.
func pythonDeployIncludes(pythonVersion string) []string {
	return []string{
		".",
		fmt.Sprintf("/usr/local/lib/python%s/site-packages", pythonVersion),
		"/usr/local/bin",
	}
}

// pythonDefaultExcludes lists patterns that should never be carried from the
// build context into a Python build stage. Order is stable for golden-test
// determinism. Patterns use gitignore-style matching (recursive directory
// names without leading slash match anywhere).
//
// Categories:
//   - Bytecode: regenerated at runtime, dropping is free
//   - Tooling caches (pytest/mypy/ruff/tox): never used at runtime
//   - Coverage artifacts: tooling output, never runtime
//   - User virtualenvs (.venv/venv): runtime uses /usr/local/lib/...,
//     never a build-time venv
//   - .env: security-positive default — committed credentials should not
//     ship to runtime images. Runtime config goes through THEOPACKS_* env
//     vars or a secrets backend.
//   - .git: never relevant to runtime; can be hundreds of MB.
//   - tests/test: production code shouldn't import from a tests dir; if
//     it does, the user can override via .dockerignore (negation: !tests/data.json).
func pythonDefaultExcludes() []string {
	return []string{
		"__pycache__",
		"*.pyc",
		"*.pyo",
		".pytest_cache",
		".mypy_cache",
		".ruff_cache",
		"tests",
		"test",
		".venv",
		"venv",
		".tox",
		".coverage",
		".env",
		".git",
	}
}

// pythonLocalLayer returns plan.NewLocalLayer() with Python-specific default
// excludes applied. Used by every plan path so all Python flavors get the
// same hygiene baseline. User-supplied .dockerignore continues to take
// precedence (merged via plan.DockerignoreContext at higher level).
func pythonLocalLayer() plan.Layer {
	l := plan.NewLocalLayer()
	l.Exclude = pythonDefaultExcludes()
	return l
}

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
	version, source := detectPythonVersion(ctx)

	ref := ctx.Resolver.Default("python", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	var err error

	if isUvWorkspace(ctx) {
		err = p.planUvWorkspace(ctx, version)
	} else if ctx.App.HasFile("requirements.txt") {
		err = p.planRequirements(ctx, version)
	} else if isPoetryProject(ctx) {
		err = p.planPoetry(ctx, version)
	} else if ctx.App.HasFile("Pipfile") {
		err = p.planPipfile(ctx, version)
	} else if ctx.App.HasFile("pyproject.toml") {
		err = p.planPyproject(ctx, version)
	} else if ctx.App.HasFile("setup.py") {
		err = p.planSetupPy(ctx, version)
	}

	if err != nil {
		return err
	}

	// Auto-detect start command if not already set by config
	if ctx.Deploy.StartCmd == "" {
		if cmd := detectStartCommand(ctx); cmd != "" {
			ctx.Deploy.StartCmd = cmd
		}
	}

	return nil
}

// planRequirements optimizes for requirements.txt with manifest-first caching.
func (p *PythonProvider) planRequirements(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cache/pip", "")
	installStep.AddCommand(plan.NewCopyCommand("requirements.txt", "./"))
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir -r requirements.txt"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(pythonLocalLayer())

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImageForVersion(version))
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: pythonDeployIncludes(version)}),
	})

	return nil
}

// planPoetry handles Poetry projects with pyproject.toml containing [tool.poetry].
// Uses poetry install with lock file caching.
func (p *PythonProvider) planPoetry(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cache/pip", "")

	// Copy manifests first for caching
	installStep.AddCommand(plan.NewCopyCommand("pyproject.toml", "./"))
	if ctx.App.HasFile("poetry.lock") {
		installStep.AddCommand(plan.NewCopyCommand("poetry.lock", "./"))
	}

	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir poetry && poetry config virtualenvs.create false && poetry install --no-root --no-interaction --no-ansi"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(pythonLocalLayer())

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImageForVersion(version))
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: pythonDeployIncludes(version)}),
	})

	return nil
}

// planPipfile handles Pipfile-based projects.
func (p *PythonProvider) planPipfile(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cache/pip", "")

	// Copy manifests first for caching
	installStep.AddCommand(plan.NewCopyCommand("Pipfile", "./"))
	if ctx.App.HasFile("Pipfile.lock") {
		installStep.AddCommand(plan.NewCopyCommand("Pipfile.lock", "./"))
	}

	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir pipenv && pipenv requirements > requirements-pipfile.txt && pip install --no-cache-dir -r requirements-pipfile.txt"))

	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(pythonLocalLayer())

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImageForVersion(version))
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: pythonDeployIncludes(version)}),
	})

	return nil
}

// planPyproject handles generic pyproject.toml projects (hatchling, setuptools, etc.).
func (p *PythonProvider) planPyproject(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cache/pip", "")
	installStep.AddInput(pythonLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir ."))

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImageForVersion(version))
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: pythonDeployIncludes(version)}),
	})

	return nil
}

// planSetupPy handles legacy setup.py projects.
func (p *PythonProvider) planSetupPy(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cache/pip", "")
	installStep.AddInput(pythonLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir ."))

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImageForVersion(version))
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: pythonDeployIncludes(version)}),
	})

	return nil
}

// planUvWorkspace handles UV workspace projects.
// UV workspaces have local path deps that need all files at install time.
// uv sync installs into .venv/ so we add that to PATH in the deploy stage.
func (p *PythonProvider) planUvWorkspace(ctx *generate.GenerateContext, version string) error {
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.PythonBuildImageForVersion(version)))
	installStep.AddCacheMount("/root/.cache/pip", "")
	installStep.AddInput(pythonLocalLayer())
	installStep.AddCommand(plan.NewExecShellCommand("pip install --no-cache-dir uv && uv sync --all-packages --no-dev"))

	ctx.Deploy.Base = plan.NewImageLayer(generate.PythonRuntimeImageForVersion(version))
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("install", plan.Filter{Include: pythonDeployIncludes(version)}),
	})
	// uv sync installs packages into .venv/bin, not /usr/local/bin
	ctx.Deploy.Paths = append(ctx.Deploy.Paths, "/app/.venv/bin")

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
		ctx.Logger.LogWarn("Failed to parse pyproject.toml for Poetry detection: %s", err)
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
		ctx.Logger.LogWarn("Failed to parse pyproject.toml for UV workspace detection: %s", err)
		return false
	}

	return len(pyproject.Tool.UV.Workspace.Members) > 0
}

// detectStartCommand tries to determine the start command for a Python app.
// It checks, in order: Procfile, FastAPI with uvicorn, Flask, Django.
func detectStartCommand(ctx *generate.GenerateContext) string {
	// Check Procfile first (industry standard: Heroku, Railway, etc.)
	if ctx.App.HasFile("Procfile") {
		if cmd := parseProcfileWeb(ctx); cmd != "" {
			return cmd
		}
	}

	// Auto-detect FastAPI + uvicorn from requirements or pyproject.toml
	if hasPackage(ctx, "uvicorn") && hasPackage(ctx, "fastapi") {
		// Look for the main module that defines the FastAPI app
		if ctx.App.HasFile("main.py") {
			return "uvicorn main:app --host 0.0.0.0 --port ${PORT:-8000}"
		}
		if ctx.App.HasFile("app.py") {
			return "uvicorn app:app --host 0.0.0.0 --port ${PORT:-8000}"
		}
	}

	// Auto-detect gunicorn
	if hasPackage(ctx, "gunicorn") {
		if ctx.App.HasFile("app.py") {
			return "gunicorn app:app --bind 0.0.0.0:${PORT:-8000}"
		}
	}

	return ""
}

// parseProcfileWeb reads a Procfile and returns the web process command.
func parseProcfileWeb(ctx *generate.GenerateContext) string {
	content, err := ctx.App.ReadFile("Procfile")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "web:") {
			cmd := strings.TrimSpace(strings.TrimPrefix(line, "web:"))
			if cmd != "" {
				return cmd
			}
		}
	}

	return ""
}

// hasPackage checks whether a Python package is listed in requirements.txt
// or pyproject.toml dependencies.
func hasPackage(ctx *generate.GenerateContext, pkg string) bool {
	if ctx.App.HasFile("requirements.txt") {
		content, err := ctx.App.ReadFile("requirements.txt")
		if err == nil {
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				// Match package name at start of line (before version specifier)
				name := strings.Split(line, ">=")[0]
				name = strings.Split(name, "==")[0]
				name = strings.Split(name, "~=")[0]
				name = strings.Split(name, "<")[0]
				name = strings.Split(name, ">")[0]
				name = strings.Split(name, "[")[0]
				name = strings.TrimSpace(name)
				if strings.EqualFold(name, pkg) {
					return true
				}
			}
		}
	}

	if ctx.App.HasFile("pyproject.toml") {
		content, err := ctx.App.ReadFile("pyproject.toml")
		if err == nil {
			// Simple check: look for the package name in dependencies
			if strings.Contains(content, "\""+pkg) || strings.Contains(content, "'"+pkg) {
				return true
			}
		}
	}

	return false
}

// detectPythonVersion determines the Python version to use for build/runtime images.
// Priority: config packages > THEOPACKS_PYTHON_VERSION env var > .python-version > runtime.txt > default.
func detectPythonVersion(ctx *generate.GenerateContext) (version string, source string) {
	// Config packages have highest priority (set via theopacks.json or THEOPACKS_PACKAGES)
	if pkg := ctx.Resolver.Get("python"); pkg != nil && pkg.Source != "theopacks default" {
		return generate.NormalizeToMajorMinor(pkg.Version), pkg.Source
	}

	// Environment variable
	if envVersion, varName := ctx.Env.GetConfigVariable("PYTHON_VERSION"); envVersion != "" {
		return generate.NormalizeToMajorMinor(envVersion), varName
	}

	// .python-version file
	if ctx.App.HasFile(".python-version") {
		if content, err := ctx.App.ReadFile(".python-version"); err == nil {
			v := generate.NormalizeToMajorMinor(strings.TrimSpace(content))
			if v != "" {
				return v, ".python-version"
			}
		}
	}

	// runtime.txt (Heroku convention: "python-3.11.6" → "3.11")
	if ctx.App.HasFile("runtime.txt") {
		if content, err := ctx.App.ReadFile("runtime.txt"); err == nil {
			line := strings.TrimSpace(content)
			line = strings.TrimPrefix(line, "python-")
			v := generate.NormalizeToMajorMinor(line)
			if v != "" {
				return v, "runtime.txt"
			}
		}
	}

	return generate.DefaultPythonVersion, "default"
}

func (p *PythonProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *PythonProvider) StartCommandHelp() string {
	return "Specify a start command:\n  - Add a Procfile with: web: uvicorn main:app --host 0.0.0.0 --port 8000\n  - Or set THEOPACKS_START_CMD=python app.py"
}
