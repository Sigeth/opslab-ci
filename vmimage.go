package main

import (
	"context"
	"fmt"

	"dagger/opslab-ci/internal/dagger"
	"dagger/opslab-ci/internal/vmrecipe"
)

// vmToolsImage is the base of the VM image toolchain.
const vmToolsImage = "docker.io/library/debian:13-slim"

// containerDiskUID owns the disk in a containerDisk (KubeVirt's qemu user).
const containerDiskUID = "107:107"

// VmImage builds a VM image recipe (image.yaml) into a KubeVirt
// containerDisk: the source disk, customized offline by the recipe's scripts
// (virt-customize), with a fresh machine-id, compressed. Publish it with
// `publish --address=...` or save it with `export --path=disk.tar`.
func (m *OpslabCi) VmImage(
	ctx context.Context,
	// Recipe directory (with image.yaml).
	// +ignore=[".git", "*.qcow2", "*.tar"]
	source *dagger.Directory,
	// Archive (OCI or docker) of the recipe's source.image, used instead of
	// pulling it: for development builds on a base not published yet.
	// +optional
	sourceImageArchive *dagger.File,
) (*dagger.Container, error) {
	data, err := source.File(vmrecipe.File).Contents(ctx)
	if err != nil {
		return nil, err
	}
	r, err := vmrecipe.Parse([]byte(data))
	if err != nil {
		return nil, err
	}

	tools := vmTools()
	var disk *dagger.File
	if r.Source.URL != "" {
		sum, tool := r.Source.SHA256, "sha256sum"
		if r.Source.SHA512 != "" {
			sum, tool = r.Source.SHA512, "sha512sum"
		}
		disk = tools.
			WithFile("/work/source.qcow2", dag.HTTP(r.Source.URL)).
			WithExec([]string{"sh", "-c", fmt.Sprintf("echo '%s  /work/source.qcow2' | %s -c -", sum, tool)}).
			File("/work/source.qcow2")
	} else {
		base := dag.Container(dagger.ContainerOpts{Platform: platform}).From(r.Source.Image)
		if sourceImageArchive != nil {
			base = dag.Container(dagger.ContainerOpts{Platform: platform}).Import(sourceImageArchive)
		}
		disks, err := base.Directory("/disk").Entries(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", r.Source.Image, err)
		}
		if len(disks) != 1 {
			return nil, fmt.Errorf("%s: want one disk in /disk, found %v", r.Source.Image, disks)
		}
		disk = base.File("/disk/" + disks[0])
	}

	// virt-customize relabels SELinux guests (Rocky) itself.
	args := []string{"virt-customize", "-a", "/work/disk.qcow2", "--network"}
	if len(r.Run) > 0 {
		args = append(args, "--copy-in", "/recipe:/tmp")
		for _, script := range r.Run {
			args = append(args, "--run-command", "cd /tmp/recipe && sh "+script)
		}
		args = append(args, "--delete", "/tmp/recipe")
	}
	// Every instance must get its own machine-id at boot.
	args = append(args, "--truncate", "/etc/machine-id")

	built := tools.
		WithMountedDirectory("/recipe", source).
		WithFile("/work/source.qcow2", disk).
		// A standalone copy: the source may have a backing file or be compressed.
		WithExec([]string{"qemu-img", "convert", "-O", "qcow2", "/work/source.qcow2", "/work/disk.qcow2"}).
		WithExec(args).
		WithExec([]string{"qemu-img", "convert", "-c", "-O", "qcow2", "/work/disk.qcow2", "/work/out.qcow2"}).
		File("/work/out.qcow2")

	return dag.Container(dagger.ContainerOpts{Platform: platform}).
		WithFile("/disk/disk.qcow2", built, dagger.ContainerWithFileOpts{Owner: containerDiskUID}), nil
}

// vmTools is the VM image toolchain: libguestfs (with a kernel for its
// appliance) and qemu-img. libguestfs uses KVM when the container has
// /dev/kvm, software emulation otherwise.
func vmTools() *dagger.Container {
	return dag.Container(dagger.ContainerOpts{Platform: platform}).
		From(vmToolsImage).
		WithEnvVariable("DEBIAN_FRONTEND", "noninteractive").
		WithExec([]string{"sh", "-c", "apt-get update && apt-get install -y --no-install-recommends " +
			"libguestfs-tools qemu-utils linux-image-amd64 ca-certificates && rm -rf /var/lib/apt/lists/*"}).
		WithEnvVariable("LIBGUESTFS_BACKEND", "direct")
}
