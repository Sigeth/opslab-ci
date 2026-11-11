# opslab-ci

Reusable [Dagger](https://dagger.io) pipelines for the opslab (working name) repositories.

Each repository keeps its build logic in its own `Makefile` (`lint`, `test`, `build`, `verify-generated`…). This module runs those targets in pinned linux/amd64 toolchain containers, so the result is the same on a laptop and in GitHub Actions. Repositories don't need their own Dagger module: their CI calls this one.

## Functions

| Call | What |
|---|---|
| `go --source=. check [--targets=lint,test,…]` | Go toolchain (golang image + make), runs Makefile targets |
| `go --source=. licenses` | Fails on dependencies whose license is not MIT-compatible |
| `rust --source=. check [--targets=lint,test,deny,…]` | Rust toolchain (rustfmt, clippy, cargo-deny, kopium), runs Makefile targets |
| `nodejs --source=. check [--targets=lint,test,…]` | Node.js toolchain (`npm ci` from the lock file), runs Makefile targets |
| `nodejs --source=. lock` | Resolves package.json in a container and returns package-lock.json (install scripts disabled) |
| `helm --source=. check [--targets=lint,template,…]` | Helm toolchain, runs Makefile targets |
| `helm --source=. validate` | Renders the chart and validates every manifest on k3s (server-side dry run) |
| `challenge-lint --source=.` | `opslab challenge lint` on a challenge repository (`--cli`/`--core-go` to use local checkouts) |
| `image --source=.` | Builds the Dockerfile for linux/amd64 |
| `crds --crds=crds --samples=samples --rejected=samples/invalid` | Starts k3s, installs the CRDs, and checks that samples are accepted and invalid ones rejected |

Before the shared libraries are published, or to try a contract change before tagging it, use local checkouts instead of the published versions:

- `--core-go=../core-go` on `go` (for Go repositories),
- `--core-rs=../core-rs` and `--core-go=../core-go` on `rust` (for Rust repositories),
- `--api=../api` on `nodejs` (the web UI's API client is generated from the api's `openapi.json`).

## Usage

```sh
# from a repository checkout
dagger -m github.com/Sigeth/opslab-ci@v0.1.0 call go --source=. check --targets=lint,test,build
```

GitHub Actions:

```yaml
- uses: dagger/dagger-for-github@v8.4.1
  with:
    version: "0.21.11"
    module: github.com/Sigeth/opslab-ci@v0.1.0
    verb: call
    args: go --source=. check --targets=lint,test
```

### Container runtime

The Dagger engine runs in a privileged container. With rootless podman (no Docker daemon), expose podman's Docker-compatible API for the session and point the docker CLI at it:

```sh
podman system service --time=0 unix://$XDG_RUNTIME_DIR/podman/podman.sock &
export DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock
```

## License

[MIT](LICENSE)
