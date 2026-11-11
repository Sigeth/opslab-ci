package main

import (
	"context"

	"dagger/opslab-ci/internal/dagger"
)

// Nodejs is the toolchain of a Node.js (web) repository.
type Nodejs struct {
	// +private
	Source *dagger.Directory
	// +private
	Version string
	// +private
	API *dagger.Directory
}

// Nodejs returns the Node.js toolchain for a repository.
func (m *OpslabCi) Nodejs(
	// Repository root (with its Makefile and package.json).
	// +ignore=["node_modules", "dist", "coverage", ".idea", ".vscode"]
	source *dagger.Directory,
	// Node.js version (node image tag).
	// +optional
	// +default="24"
	version string,
	// Local checkout of opslab-api whose openapi.json generates the API
	// client (sets OPENAPI_FILE for the Makefile).
	// +optional
	// +ignore=["*", "!openapi.json"]
	api *dagger.Directory,
) *Nodejs {
	return &Nodejs{Source: source, Version: version, API: api}
}

func (n *Nodejs) base() *dagger.Container {
	ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
		From("docker.io/library/node:"+n.Version).
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "*"}).
		WithMountedCache("/root/.npm", dag.CacheVolume("opslab-npm")).
		WithEnvVariable("CI", "true").
		WithMountedDirectory("/src", n.Source).
		WithWorkdir("/src")
	if n.API != nil {
		ctr = ctr.
			WithMountedDirectory("/deps/api", n.API).
			WithEnvVariable("OPENAPI_FILE", "/deps/api/openapi.json")
	}
	return ctr
}

// Container with the toolchain and the dependencies installed from the lock file.
func (n *Nodejs) Container() *dagger.Container {
	return n.base().WithExec([]string{"npm", "ci", "--no-audit", "--no-fund"})
}

// Check runs Makefile targets (default: lint and test).
func (n *Nodejs) Check(
	ctx context.Context,
	// Makefile targets to run, in order.
	// +optional
	// +default=["lint", "test"]
	targets []string,
) (string, error) {
	return makeTargets(n.Container(), targets).Stdout(ctx)
}

// Lock resolves the dependencies of package.json and returns the new
// package-lock.json (install scripts are not run).
func (n *Nodejs) Lock() *dagger.File {
	return n.base().
		WithExec([]string{"npm", "install", "--package-lock-only", "--ignore-scripts", "--no-audit", "--no-fund"}).
		File("/src/package-lock.json")
}

// Build runs `make build` and returns the dist directory.
func (n *Nodejs) Build() *dagger.Directory {
	return makeTargets(n.Container(), []string{"build"}).Directory("/src/dist")
}
