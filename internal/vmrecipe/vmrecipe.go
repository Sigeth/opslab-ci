// Package vmrecipe reads VM image recipes (image.yaml, D32).
package vmrecipe

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"sigs.k8s.io/yaml"
)

// File describes a VM image (D32): its source disk and the scripts
// customizing it.
const File = "image.yaml"

// Recipe is the content of image.yaml.
type Recipe struct {
	// Version of the image, used as its tag when published.
	Version string `json:"version"`
	// Source disk: an upstream cloud image or another containerDisk.
	Source struct {
		// URL of a qcow2 cloud image, with its SHA-256 or SHA-512.
		URL    string `json:"url"`
		SHA256 string `json:"sha256"`
		SHA512 string `json:"sha512"`
		// Image is a containerDisk to build on (a base image).
		Image string `json:"image"`
	} `json:"source"`
	// Run lists scripts (relative to the recipe) run as root in the disk, in
	// order, with network access. The recipe directory is at /tmp/recipe
	// while they run, and removed afterwards.
	Run []string `json:"run"`
}

// Parse decodes and checks image.yaml.
func Parse(data []byte) (*Recipe, error) {
	var r Recipe
	if err := yaml.UnmarshalStrict(data, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	s := r.Source
	switch {
	case (s.URL == "") == (s.Image == ""):
		return nil, errors.New(File + ": source needs exactly one of url and image")
	case s.URL != "" && (s.SHA256 == "") == (s.SHA512 == ""):
		return nil, errors.New(File + ": source.url needs exactly one of sha256 and sha512")
	case s.Image != "" && (s.SHA256 != "" || s.SHA512 != ""):
		return nil, errors.New(File + ": checksums apply to source.url only (pin source.image by digest)")
	}
	for _, script := range r.Run {
		if path.IsAbs(script) || strings.HasPrefix(path.Clean(script), "..") {
			return nil, fmt.Errorf("%s: run: %q must stay inside the recipe", File, script)
		}
	}
	return &r, nil
}
