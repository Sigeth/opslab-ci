package main

import (
	"context"

	"dagger/opslab-ci/internal/dagger"
)

// ChallengeLint validates a challenge package with `opslab challenge lint`
// (the same validation the operator runs at install time).
func (m *OpslabCi) ChallengeLint(
	ctx context.Context,
	// Challenge repository root (with challenge.yaml).
	// +ignore=[".git", ".idea", ".vscode"]
	source *dagger.Directory,
	// Version of the opslab CLI to install.
	// +optional
	// +default="v0.1.0"
	cliVersion string,
	// Local checkout of opslab-cli built instead of the published version.
	// +optional
	// +ignore=["bin", "dist", ".idea", ".vscode"]
	cli *dagger.Directory,
	// Local checkout of opslab-core-go used by the local CLI.
	// +optional
	// +ignore=["bin", ".idea", ".vscode"]
	coreGo *dagger.Directory,
) (string, error) {
	var ctr *dagger.Container
	if cli != nil {
		ctr = m.Go(cli, "1.27", coreGo).Container().
			WithExec([]string{"go", "build", "-o", "/usr/local/bin/opslab", "./cmd/opslab"})
	} else {
		ctr = m.Go(dag.Directory(), "1.27", nil).Container().
			WithExec([]string{"go", "install", "github.com/Sigeth/opslab-cli/cmd/opslab@" + cliVersion}).
			WithExec([]string{"cp", "/go/bin/opslab", "/usr/local/bin/opslab"})
	}
	return ctr.
		WithMountedDirectory("/challenge", source).
		WithExec([]string{"opslab", "challenge", "lint", "/challenge"}).
		Stdout(ctx)
}
