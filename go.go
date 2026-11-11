package main

import (
	"context"

	"dagger/opslab-ci/internal/dagger"
)

const coreGoModule = "github.com/Sigeth/opslab-core-go"

// goVersion is the default golang image tag; keep it in sync with the
// +default of Go().
const goVersion = "1.27.4"

// licensesAllowed are the dependency licenses compatible with MIT (D14).
const licensesAllowed = "MIT,Apache-2.0,BSD-2-Clause,BSD-3-Clause,ISC"

// Go is the toolchain of a Go repository.
type Go struct {
	// +private
	Source *dagger.Directory
	// +private
	Version string
	// +private
	CoreGo *dagger.Directory
}

// Go returns the Go toolchain for a repository.
func (m *OpslabCi) Go(
	// Repository root (with its Makefile).
	// +ignore=["bin", "target", "node_modules", "dist", ".idea", ".vscode"]
	source *dagger.Directory,
	// Go version (golang image tag).
	// +optional
	// +default="1.27.4"
	version string,
	// Local checkout of opslab-core-go used instead of the published module.
	// +optional
	// +ignore=["bin", "target", "node_modules", "dist", ".idea", ".vscode"]
	coreGo *dagger.Directory,
) *Go {
	return &Go{Source: source, Version: version, CoreGo: coreGo}
}

// Container with the toolchain, caches and the source at /src.
func (g *Go) Container() *dagger.Container {
	ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
		From("docker.io/library/golang:"+g.Version).
		WithExec([]string{"sh", "-c", "apt-get update -qq && apt-get install -y -qq --no-install-recommends make >/dev/null"}).
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "*"}).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("opslab-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("opslab-go-build-"+g.Version)).
		WithEnvVariable("GOTOOLCHAIN", "local").
		WithMountedDirectory("/src", g.Source).
		WithWorkdir("/src")
	if g.CoreGo != nil {
		// A go.work outside the repository makes the local checkout replace
		// the required version of opslab-core-go.
		ctr = ctr.
			WithMountedDirectory("/deps/core-go", g.CoreGo).
			WithEnvVariable("GOWORK", "/deps/go.work").
			WithExec([]string{"sh", "-euc", `
cd /deps
go work init /src
if [ "$(cd /src && go list -m)" != "` + coreGoModule + `" ]; then
  go work use /deps/core-go
  version=$(cd /src && go mod edit -json | grep -A1 '"Path": "` + coreGoModule + `"' | sed -n 's/.*"Version": "\(.*\)".*/\1/p')
  [ -z "$version" ] || go work edit -replace=` + coreGoModule + `@$version=/deps/core-go
fi`})
	}
	return ctr
}

// Check runs Makefile targets (default: lint and test).
func (g *Go) Check(
	ctx context.Context,
	// Makefile targets to run, in order.
	// +optional
	// +default=["lint", "test"]
	targets []string,
) (string, error) {
	return makeTargets(g.Container(), targets).Stdout(ctx)
}

// Licenses fails if a dependency is not under an MIT-compatible license.
func (g *Go) Licenses(ctx context.Context) (string, error) {
	return g.Container().
		WithExec([]string{"go", "run", "github.com/google/go-licenses/v2@v2.0.1",
			"check", "./...", "--allowed_licenses=" + licensesAllowed}).
		Stdout(ctx)
}
