// Reusable CI pipelines for the opslab repositories.
//
// Every opslab repository keeps its build logic in its own Makefile (lint,
// test, build, verify-generated...). This module runs those targets in pinned
// toolchain containers, so a pipeline gives the same result on a laptop and in
// GitHub Actions:
//
//	dagger -m github.com/Sigeth/opslab-ci@v0.1.0 call go --source=. check
//	dagger -m github.com/Sigeth/opslab-ci@v0.1.0 call rust --source=. check --targets=lint,test,deny
//
// Before the shared libraries are published (or to try a contract change
// before tagging it), pass local checkouts with --core-go / --core-rs.
package main

import (
	"dagger/opslab-ci/internal/dagger"
)

// Target platform of every artifact (opslab supports linux/amd64 only).
const platform = dagger.Platform("linux/amd64")

// OpslabCi holds the pipelines.
type OpslabCi struct{}

// Image builds the repository's Dockerfile for linux/amd64.
func (m *OpslabCi) Image(
	// Repository root.
	// +ignore=["bin", "target", "node_modules", "dist", ".idea", ".vscode"]
	source *dagger.Directory,
	// Dockerfile path, relative to the source.
	// +optional
	// +default="Dockerfile"
	dockerfile string,
) *dagger.Container {
	return source.DockerBuild(dagger.DirectoryDockerBuildOpts{
		Platform:   platform,
		Dockerfile: dockerfile,
	})
}

// makeTargets runs `make <targets>` in a container prepared for the source.
func makeTargets(ctr *dagger.Container, targets []string) *dagger.Container {
	return ctr.WithExec(append([]string{"make"}, targets...))
}
