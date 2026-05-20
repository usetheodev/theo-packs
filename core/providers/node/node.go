package node

import (
	"fmt"
	"path/filepath"
	"strings"

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

	version, source := detectNodeVersion(ctx, pkg)
	ref := ctx.Resolver.Default("node", version)
	if source != "default" {
		ctx.Resolver.Version(ref, version, source)
	}

	// Install step — copy manifests first for layer caching
	installStep := ctx.NewCommandStep("install")
	installStep.AddInput(plan.NewImageLayer(generate.NodeBuildImageForVersion(version)))
	addNodeCacheMounts(installStep, pm)

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

	// Build step — copy full source and run build if available.
	//
	// CHG-002b 2026-04-28 — workspace-aware build command. When Theo runs
	// theopacks-generate against a workspace root, it sets
	// THEOPACKS_APP_NAME so we can scope the build to a single app while
	// still building all transitive deps in the workspace.
	buildStep := ctx.NewCommandStep("build")
	buildStep.AddInput(plan.NewStepLayer("install"))
	buildStep.AddInput(ctx.NewLocalLayer())
	addNodeCacheMounts(buildStep, pm)

	appName, _ := ctx.Env.GetConfigVariable("APP_NAME")
	appPath, _ := ctx.Env.GetConfigVariable("APP_PATH")

	if pkg.hasBuildScript() || (ws != nil && appName != "") {
		buildCmd := workspaceBuildCommand(pm, ws, appName, pkg.hasBuildScript())
		if buildCmd != "" {
			buildStep.AddCommand(plan.NewExecShellCommand(buildCmd))
		}
	}

	// Drop devDependencies from node_modules so the deploy stage's COPY of
	// /app carries only production deps. The prune RUN reuses the same cache
	// mounts attached above (addNodeCacheMounts), so no network round-trip.
	// Bun has no prune subcommand → PruneCommand returns "" and we skip.
	if pruneCmd := PruneCommand(pm); pruneCmd != "" {
		buildStep.AddCommand(plan.NewExecShellCommand(pruneCmd))
	}

	// Deploy — use start script from package.json if available
	ctx.Deploy.Base = plan.NewImageLayer(generate.NodeRuntimeImageForVersion(version))

	// In workspace mode, the app's start script lives in apps/<name>/package.json,
	// not the root. Inject `cd <appPath>` before the npm start.
	if ws != nil && appPath != "" {
		ctx.Deploy.StartCmd = fmt.Sprintf("cd %s && %s", appPath, startCommand())
	} else if pkg.hasStartScript() {
		ctx.Deploy.StartCmd = startCommand()
	} else {
		ctx.Deploy.StartCmd = "npm start"
	}
	ctx.Deploy.AddInputs([]plan.Layer{
		plan.NewStepLayer("build", plan.Filter{Include: []string{"."}}),
	})

	return nil
}

// workspaceBuildCommand returns the shell command that builds the target
// app, respecting any workspace dependency graph.
//
// Order of preference:
//  1. turbo (when turbo.json present)               — `npx turbo run build --filter=<app>...`
//     The trailing `...` includes the app's transitive deps.
//  2. pnpm workspaces                               — `pnpm --filter <app>... run build`
//     Same semantics: includes transitive deps.
//  3. npm/yarn workspaces with appName              — build all packages first
//     (no topological order in npm), then the app:
//     `npm run build --workspaces --if-present && npm run build -w <app> --if-present`
//  4. Standalone (no workspace)                     — `<pm> run build`.
//
// If the package.json has no build script and we're not in workspace mode,
// returns "" (caller skips emitting RUN).
func workspaceBuildCommand(pm PackageManager, ws *WorkspaceInfo, appName string, hasBuild bool) string {
	if ws == nil {
		if !hasBuild {
			return ""
		}
		return runCommand(pm, "build")
	}

	if ws.HasTurbo {
		if appName == "" {
			return "npx turbo run build"
		}
		return fmt.Sprintf("npx turbo run build --filter=%s...", appName)
	}

	switch ws.Type {
	case WorkspacePnpm:
		if appName == "" {
			return "pnpm -r run build"
		}
		// `<app>...` selects app + its workspace deps in topological order.
		return fmt.Sprintf("pnpm --filter %s... run build", appName)
	case WorkspaceNpm, WorkspaceYarn:
		// npm and yarn classic don't have built-in topological build; build
		// all workspaces (no-op for those without a build script via
		// --if-present) so packages are ready before the app's compiler runs.
		if appName == "" {
			return runCommand(pm, "build") + " --workspaces --if-present"
		}
		all := runCommand(pm, "build") + " --workspaces --if-present"
		one := fmt.Sprintf("%s --workspace=%s --if-present", runCommand(pm, "build"), appName)
		return all + " && " + one
	}

	// Defensive fallback.
	if hasBuild {
		return runCommand(pm, "build")
	}
	return ""
}

// detectNodeVersion determines the Node.js version to use for build/runtime images.
// Priority: config packages > THEOPACKS_NODE_VERSION env var > package.json engines > .nvmrc > .node-version > default.
func detectNodeVersion(ctx *generate.GenerateContext, pkg *packageJSON) (version string, source string) {
	// Config packages have highest priority (set via theopacks.json or THEOPACKS_PACKAGES)
	if p := ctx.Resolver.Get("node"); p != nil && p.Source != "theopacks default" {
		return generate.NormalizeToMajor(p.Version), p.Source
	}

	// Environment variable
	if envVersion, varName := ctx.Env.GetConfigVariable("NODE_VERSION"); envVersion != "" {
		return generate.NormalizeToMajor(envVersion), varName
	}

	// package.json engines.node
	if nodeEngine, ok := pkg.Engines["node"]; ok && nodeEngine != "" {
		v := generate.NormalizeToMajor(nodeEngine)
		if v != "" {
			return v, "package.json engines.node"
		}
	}

	// .nvmrc file
	if ctx.App.HasFile(".nvmrc") {
		if content, err := ctx.App.ReadFile(".nvmrc"); err == nil {
			v := generate.NormalizeToMajor(strings.TrimSpace(content))
			if v != "" {
				return v, ".nvmrc"
			}
		}
	}

	// .node-version file
	if ctx.App.HasFile(".node-version") {
		if content, err := ctx.App.ReadFile(".node-version"); err == nil {
			v := generate.NormalizeToMajor(strings.TrimSpace(content))
			if v != "" {
				return v, ".node-version"
			}
		}
	}

	return generate.DefaultNodeVersion, "default"
}

func (p *NodeProvider) CleansePlan(buildPlan *plan.BuildPlan) {}

func (p *NodeProvider) StartCommandHelp() string {
	return "Add a \"start\" script to your package.json, e.g.:\n  \"scripts\": { \"start\": \"node server.js\" }"
}

// packageJSON holds the fields we need from package.json.
type packageJSON struct {
	Scripts map[string]string `json:"scripts"`
	Engines map[string]string `json:"engines"`
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

// addNodeCacheMounts attaches per-PM BuildKit cache mounts to a step. We
// mount each PM's local cache as well as ~/.npm so a hybrid project (e.g.
// pnpm at the workspace root, npm in a subpackage) still gets full reuse.
func addNodeCacheMounts(step *generate.CommandStepBuilder, pm PackageManager) {
	step.AddCacheMount("/root/.npm", "")
	switch pm {
	case PackageManagerPnpm:
		step.AddCacheMount("/root/.local/share/pnpm/store", "")
	case PackageManagerYarn:
		step.AddCacheMount("/usr/local/share/.cache/yarn", "")
	case PackageManagerBun:
		step.AddCacheMount("/root/.bun/install/cache", "")
	}
}
