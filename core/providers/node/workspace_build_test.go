package node

import "testing"

func TestWorkspaceBuildCommand_Standalone(t *testing.T) {
	got := workspaceBuildCommand(PackageManagerNpm, nil, "", true)
	if got != "npm run build" {
		t.Errorf("standalone+hasBuild: got %q, want %q", got, "npm run build")
	}
	if got := workspaceBuildCommand(PackageManagerNpm, nil, "", false); got != "" {
		t.Errorf("standalone+noBuild: got %q, want \"\"", got)
	}
}

func TestWorkspaceBuildCommand_Turbo(t *testing.T) {
	ws := &WorkspaceInfo{Type: WorkspaceNpm, PackageManager: PackageManagerNpm, HasTurbo: true}
	got := workspaceBuildCommand(PackageManagerNpm, ws, "api", true)
	want := "npx turbo run build --filter=api..."
	if got != want {
		t.Errorf("turbo+app: got %q, want %q", got, want)
	}
	got = workspaceBuildCommand(PackageManagerNpm, ws, "", true)
	if got != "npx turbo run build" {
		t.Errorf("turbo+noApp: got %q, want %q", got, "npx turbo run build")
	}
}

func TestWorkspaceBuildCommand_Pnpm(t *testing.T) {
	ws := &WorkspaceInfo{Type: WorkspacePnpm, PackageManager: PackageManagerPnpm}
	got := workspaceBuildCommand(PackageManagerPnpm, ws, "api", true)
	want := "pnpm --filter api... run build"
	if got != want {
		t.Errorf("pnpm+app: got %q, want %q", got, want)
	}
}

func TestWorkspaceBuildCommand_NpmWorkspaces(t *testing.T) {
	ws := &WorkspaceInfo{Type: WorkspaceNpm, PackageManager: PackageManagerNpm}
	got := workspaceBuildCommand(PackageManagerNpm, ws, "api", true)
	want := "npm run build --workspaces --if-present && npm run build --workspace=api --if-present"
	if got != want {
		t.Errorf("npm-ws+app: got %q, want %q", got, want)
	}
}

func TestWorkspaceBuildCommand_Yarn(t *testing.T) {
	ws := &WorkspaceInfo{Type: WorkspaceYarn, PackageManager: PackageManagerYarn}
	got := workspaceBuildCommand(PackageManagerYarn, ws, "gateway", true)
	want := "yarn run build --workspaces --if-present && yarn run build --workspace=gateway --if-present"
	if got != want {
		t.Errorf("yarn-ws+app: got %q, want %q", got, want)
	}
}
