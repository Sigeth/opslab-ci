package main

import (
	"context"
	"time"

	"dagger/opslab-ci/internal/dagger"
)

// K3sVersion is the default Kubernetes version CRDs are validated against.
const K3sVersion = "v1.37.2-k3s1"

// k3s starts a k3s control plane (no kubelet: only the API server matters
// here) and returns a client container with kubectl configured for it.
func k3s(version, name string) *dagger.Container {
	config := dag.CacheVolume("opslab-k3s-" + name)
	image := "docker.io/rancher/k3s:" + version

	server := dag.Container(dagger.ContainerOpts{Platform: platform}).
		From(image).
		WithMountedCache("/etc/rancher/k3s", config).
		WithMountedTemp("/var/lib/rancher/k3s").
		WithMountedTemp("/var/log").
		WithExposedPort(6443).
		AsService(dagger.ContainerAsServiceOpts{
			Args: []string{"sh", "-c", "rm -f /etc/rancher/k3s/k3s.yaml && " +
				"exec k3s server --disable-agent --tls-san=k3s --write-kubeconfig-mode=644 " +
				"--disable=traefik,metrics-server,servicelb,local-storage,coredns"},
			InsecureRootCapabilities: true,
		})

	return dag.Container(dagger.ContainerOpts{Platform: platform}).
		From(image).
		WithMountedCache("/cache", config).
		WithDirectory("/root", dag.Directory()).
		WithServiceBinding("k3s", server).
		WithEnvVariable("KUBECONFIG", "/root/kubeconfig").
		// Every run starts a new server with new certificates: never reuse a
		// cached client setup.
		WithEnvVariable("OPSLAB_K3S_RUN", time.Now().Format(time.RFC3339Nano)).
		// The shared volume may still hold the previous server's kubeconfig
		// until the new one is written: re-read it on every attempt.
		WithExec([]string{"sh", "-euc", `
for i in $(seq 1 180); do
  if [ -s /cache/k3s.yaml ]; then
    sed 's#https://127.0.0.1:6443#https://k3s:6443#' /cache/k3s.yaml > /root/kubeconfig
    kubectl get --raw /readyz >/dev/null 2>&1 && exit 0
  fi
  sleep 1
done
echo "k3s did not become ready" >&2; exit 1`})
}

// Crds installs CRDs on a real Kubernetes API server and server-side
// validates sample objects against them (schemas and CEL rules are only fully
// checked by an API server).
func (m *OpslabCi) Crds(
	ctx context.Context,
	// Directory of CustomResourceDefinition manifests.
	crds *dagger.Directory,
	// Directory of sample custom resources that must be accepted.
	// +optional
	samples *dagger.Directory,
	// Directory of custom resources that must each be rejected.
	// +optional
	rejected *dagger.Directory,
	// Namespaces to create before applying the samples.
	// +optional
	// +default=["opslab-instances"]
	namespaces []string,
	// k3s image tag (Kubernetes version).
	// +optional
	// +default="v1.37.2-k3s1"
	version string,
) (string, error) {
	ctr := k3s(version, "crds").
		WithMountedDirectory("/crds", crds).
		WithExec([]string{"kubectl", "apply", "--server-side", "-f", "/crds"}).
		WithExec([]string{"kubectl", "wait", "--for=condition=Established", "--timeout=60s", "crd", "--all"})
	for _, ns := range namespaces {
		ctr = ctr.WithExec([]string{"kubectl", "create", "namespace", ns})
	}
	if samples != nil {
		ctr = ctr.
			WithMountedDirectory("/samples", samples).
			WithExec([]string{"kubectl", "apply", "--dry-run=server", "-f", "/samples"})
	}
	if rejected != nil {
		ctr = ctr.
			WithMountedDirectory("/rejected", rejected).
			WithExec([]string{"sh", "-euc", `
for f in /rejected/*.yaml; do
  if out=$(kubectl apply --dry-run=server -f "$f" 2>&1); then
    echo "ACCEPTED but must be rejected: $f" >&2; exit 1
  fi
  echo "rejected as expected: $(basename "$f"): $(echo "$out" | tail -n1)"
done`})
	}
	return ctr.Stdout(ctx)
}
