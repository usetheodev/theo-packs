package node

import (
	"fmt"
	"path/filepath"

	"github.com/usetheo/theopacks/core/app"
	"github.com/usetheo/theopacks/core/generate"
	"github.com/usetheo/theopacks/core/logger"
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
	pm := DetectPackageManager(ctx.App)
	ws := DetectWorkspace(ctx.App, ctx.Logger)

	// Workspace detection may override package manager
	if ws != nil {
		pm = ws.PackageManager
	}

	lockfile := LockfileName(pm)
	hasLock := ctx.App.HasFile(lockfile)
	if pm == PackageManagerBun && !hasLock {
		hasLock = ctx.App.HasFile("bun.lock")
	}

	installCmd := InstallCommand(pm, hasLock)
	pkg := readPackageJSON(ctx.App, ctx.Logger)

	// Install step — copy manifests first for layer caching
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.NodeBuildImage))

	if setup := SetupCommand(pm); setup != "" {
		installStep.AddCommand(plan.NewExecShellCommand(setup))
	}

	if ws != nil {
		manifests := ManifestFiles(ctx.App, pm, ws)
		for _, f := range manifests {
			dest := filepath.Dir(f)
			if dest == "." {
				dest = "./"
			} else {
				dest += "/"
			}
			installStep.AddCommand(plan.NewCopyCommand(f, dest))
		}
	} else {
		installStep.AddCommand(plan.NewCopyCommand("package.json", "./"))
		if hasLock {
			installStep.AddCommand(plan.NewCopyCommand(lockfile, "./"))
		}
	}

	installStep.AddCommand(plan.NewExecShellCommand(installCmd))

	// Build step — copy full source and run build if available
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())

	if pkg.hasBuildScript() {
		buildCmd := runCommand(pm, "build")
		buildStep.AddCommand(plan.NewExecShellCommand(buildCmd))
	}

	// Deploy — use start script from package.json if available
	ctx.Deploy.Base = plan.NewImageLayer(generate.NodeRuntimeImage)
	if pkg.hasStartScript() {
		ctx.Deploy.StartCmd = startCommand()
	} else {
		ctx.Deploy.StartCmd = "npm start"
	}
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

func (p *NodeProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *NodeProvider) StartCommandHelp() string {
	return "Add a \"start\" script to your package.json, e.g.:\n  \"scripts\": { \"start\": \"node server.js\" }"
}

// packageJSON holds the fields we need from package.json.
type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

func readPackageJSON(a *app.App, log *logger.Logger) *packageJSON {
	var pkg packageJSON
	if err := a.ReadJSON("package.json", &pkg); err != nil {
		log.LogWarn("Failed to parse package.json, falling back to defaults: %s", err)
		return &packageJSON{}
	}
	return &pkg
}

func (p *packageJSON) hasBuildScript() bool {
	_, ok := p.Scripts["build"]
	return ok
}

func (p *packageJSON) hasStartScript() bool {
	_, ok := p.Scripts["start"]
	return ok
}

// runCommand returns the command to run a package.json script via the package manager.
func runCommand(pm PackageManager, script string) string {
	switch pm {
	case PackageManagerPnpm:
		return fmt.Sprintf("pnpm run %s", script)
	case PackageManagerYarn:
		return fmt.Sprintf("yarn run %s", script)
	case PackageManagerBun:
		return fmt.Sprintf("bun run %s", script)
	default:
		return fmt.Sprintf("npm run %s", script)
	}
}

// startCommand returns the start command for the deploy stage.
// Always uses npm start because npm is guaranteed to be available in the
// node runtime image. pnpm/yarn/bun are only installed in the build stage.
// npm start reads scripts.start from package.json, which is package-manager agnostic.
func startCommand() string {
	return "npm start"
}
