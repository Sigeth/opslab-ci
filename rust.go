package main

import (
	"context"

	"dagger/opslab-ci/internal/dagger"
)

const (
	coreRsGitURL     = "https://github.com/Sigeth/opslab-core-rs"
	cargoDenyVersion = "0.20.2"
	kopiumVersion    = "0.24.1"
	postgresImage    = "docker.io/library/postgres:18"
)

// Rust is the toolchain of a Rust repository.
type Rust struct {
	// +private
	Source *dagger.Directory
	// +private
	Version string
	// +private
	CoreRs *dagger.Directory
	// +private
	CoreGo *dagger.Directory
	// +private
	Postgres bool
}

// Rust returns the Rust toolchain for a repository.
func (m *OpslabCi) Rust(
	// Repository root (with its Makefile).
	// +ignore=["bin", "target", "node_modules", "dist", ".idea", ".vscode"]
	source *dagger.Directory,
	// Rust version (rust image tag).
	// +optional
	// +default="1.99.1"
	version string,
	// Local checkout of opslab-core-rs (with its .git and tags) used instead
	// of the published repository.
	// +optional
	// +ignore=["bin", "target", "node_modules", "dist", ".idea", ".vscode"]
	coreRs *dagger.Directory,
	// Local checkout of opslab-core-go whose CRDs are used to generate Rust
	// types (sets CORE_GO_DIR for the Makefile).
	// +optional
	// +ignore=["bin", "target", "node_modules", "dist", ".idea", ".vscode"]
	coreGo *dagger.Directory,
	// Start a PostgreSQL server for the tests, reachable through
	// OPSLAB_TEST_DATABASE_URL (superuser, may create databases).
	// +optional
	postgres bool,
) *Rust {
	return &Rust{Source: source, Version: version, CoreRs: coreRs, CoreGo: coreGo, Postgres: postgres}
}

// Container with the toolchain (rustfmt, clippy, cargo-deny, kopium), caches
// and the source at /src.
func (r *Rust) Container() *dagger.Container {
	ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
		From("docker.io/library/rust:"+r.Version).
		WithExec([]string{"sh", "-c", "apt-get update -qq && apt-get install -y -qq --no-install-recommends make >/dev/null"}).
		WithExec([]string{"rustup", "component", "add", "rustfmt", "clippy"}).
		WithExec([]string{"cargo", "install", "--locked", "cargo-deny@" + cargoDenyVersion, "kopium@" + kopiumVersion}).
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "*"}).
		WithMountedCache("/usr/local/cargo/registry", dag.CacheVolume("opslab-cargo-registry")).
		WithMountedCache("/usr/local/cargo/git", dag.CacheVolume("opslab-cargo-git")).
		WithMountedCache("/src/target", dag.CacheVolume("opslab-cargo-target-"+r.Version)).
		WithEnvVariable("CARGO_TERM_COLOR", "never").
		WithMountedDirectory("/src", r.Source).
		WithWorkdir("/src")
	if r.CoreRs != nil {
		// Fetch the "published" repository from the local checkout, then
		// patch it with the working tree so uncommitted changes are used too.
		ctr = ctr.
			WithMountedDirectory("/deps/core-rs", r.CoreRs).
			WithEnvVariable("CARGO_NET_GIT_FETCH_WITH_CLI", "true").
			WithExec([]string{"git", "config", "--global", "url.file:///deps/core-rs.insteadOf", coreRsGitURL}).
			WithNewFile("/usr/local/cargo/config.toml", `[patch."`+coreRsGitURL+`"]
opslab-types = { path = "/deps/core-rs/crates/opslab-types" }
opslab-auth = { path = "/deps/core-rs/crates/opslab-auth" }
`)
	}
	if r.CoreGo != nil {
		ctr = ctr.
			WithMountedDirectory("/deps/core-go", r.CoreGo).
			WithEnvVariable("CORE_GO_DIR", "/deps/core-go")
	}
	if r.Postgres {
		db := dag.Container(dagger.ContainerOpts{Platform: platform}).
			From(postgresImage).
			WithEnvVariable("POSTGRES_PASSWORD", "test").
			WithExposedPort(5432).
			AsService(dagger.ContainerAsServiceOpts{UseEntrypoint: true})
		ctr = ctr.
			WithServiceBinding("postgres", db).
			WithEnvVariable("OPSLAB_TEST_DATABASE_URL", "postgres://postgres:test@postgres:5432/postgres")
	}
	return ctr
}

// Check runs Makefile targets (default: lint and test).
func (r *Rust) Check(
	ctx context.Context,
	// Makefile targets to run, in order.
	// +optional
	// +default=["lint", "test"]
	targets []string,
) (string, error) {
	return makeTargets(r.Container(), targets).Stdout(ctx)
}
