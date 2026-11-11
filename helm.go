package main

import (
	"context"

	"dagger/opslab-ci/internal/dagger"
)

// Helm is the toolchain of a Helm chart repository.
type Helm struct {
	// +private
	Source *dagger.Directory
	// +private
	Version string
	// +private
	CoreGo *dagger.Directory
	// +private
	Operator *dagger.Directory
}

// Helm returns the Helm toolchain for a chart repository.
func (m *OpslabCi) Helm(
	// Repository root (with its Makefile).
	// +ignore=["rendered", ".idea", ".vscode"]
	source *dagger.Directory,
	// Helm version (alpine/helm image tag).
	// +optional
	// +default="4.3.1"
	version string,
	// Local checkout of opslab-core-go providing the CRDs (sets CORE_GO_DIR).
	// +optional
	// +ignore=["*", "!crds"]
	coreGo *dagger.Directory,
	// Local checkout of opslab-operator providing the RBAC role (sets OPERATOR_DIR).
	// +optional
	// +ignore=["*", "!config"]
	operator *dagger.Directory,
) *Helm {
	return &Helm{Source: source, Version: version, CoreGo: coreGo, Operator: operator}
}

// Container with helm, make and the source at /src.
func (h *Helm) Container() *dagger.Container {
	ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
		From("docker.io/alpine/helm:"+h.Version).
		WithExec([]string{"apk", "add", "--no-cache", "make", "bash", "git", "curl", "tar"}).
		WithExec([]string{"git", "config", "--global", "--add", "safe.directory", "*"}).
		WithMountedDirectory("/src", h.Source).
		WithWorkdir("/src").
		WithoutEntrypoint()
	if h.CoreGo != nil {
		ctr = ctr.WithMountedDirectory("/deps/core-go", h.CoreGo).WithEnvVariable("CORE_GO_DIR", "/deps/core-go")
	}
	if h.Operator != nil {
		ctr = ctr.WithMountedDirectory("/deps/operator", h.Operator).WithEnvVariable("OPERATOR_DIR", "/deps/operator")
	}
	return ctr
}

// Check runs Makefile targets (default: lint and template).
func (h *Helm) Check(
	ctx context.Context,
	// Makefile targets to run, in order.
	// +optional
	// +default=["lint", "template"]
	targets []string,
) (string, error) {
	return makeTargets(h.Container(), targets).Stdout(ctx)
}

// Validate renders the chart (make template) and server-side validates every
// rendered manifest on k3s, after installing the chart's CRDs.
func (h *Helm) Validate(
	ctx context.Context,
	// Directory of the chart's CRDs, relative to the source.
	// +optional
	// +default="opslab/crds"
	crds string,
	// Namespace the chart is rendered for (created before validation).
	// +optional
	// +default="opslab-system"
	namespace string,
	// k3s image tag (Kubernetes version).
	// +optional
	// +default="v1.37.2-k3s1"
	kubernetes string,
) (string, error) {
	rendered := makeTargets(h.Container(), []string{"template"}).Directory("/src/rendered")
	return k3s(kubernetes, "helm").
		WithMountedDirectory("/crds", h.Source.Directory(crds)).
		WithExec([]string{"kubectl", "apply", "--server-side", "-f", "/crds"}).
		WithExec([]string{"kubectl", "wait", "--for=condition=Established", "--timeout=60s", "crd", "--all"}).
		WithExec([]string{"kubectl", "create", "namespace", namespace}).
		WithMountedDirectory("/rendered", rendered).
		WithExec([]string{"sh", "-euc", `
for f in /rendered/*.yaml; do
  echo "== $(basename "$f")"
  # A server dry-run does not create namespaces, so namespaced objects in a
  # namespace the chart creates would be rejected: create those first.
  for ns in $(awk '/^kind: Namespace$/{n=1;next} n && /^  name: /{print $2; n=0} /^---/{n=0}' "$f"); do
    kubectl get namespace "$ns" >/dev/null 2>&1 || kubectl create namespace "$ns"
  done
  kubectl apply --dry-run=server -f "$f"
done`}).
		Stdout(ctx)
}
