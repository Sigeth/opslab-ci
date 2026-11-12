package vmrecipe

import (
	"strings"
	"testing"
)

func TestParseRecipe(t *testing.T) {
	r, err := Parse([]byte("version: 1.0.0\nsource:\n  url: https://example.org/d.qcow2\n  sha512: abc\nrun: [customize.sh, sub/more.sh]\n"))
	if err != nil || r.Source.SHA512 != "abc" || len(r.Run) != 2 || r.Version != "1.0.0" {
		t.Fatalf("recipe = %+v, %v", r, err)
	}
	if _, err := Parse([]byte("source: {image: ghcr.io/sigeth/opslab-images/debian-13@sha256:00}\n")); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"no source":            "run: [a.sh]\n",
		"both sources":         "source: {url: u, sha256: s, image: i}\n",
		"no checksum":          "source: {url: u}\n",
		"two checksums":        "source: {url: u, sha256: a, sha512: b}\n",
		"checksum on an image": "source: {image: i, sha256: a}\n",
		"escaping script":      "source: {image: i}\nrun: [../x.sh]\n",
		"absolute script":      "source: {image: i}\nrun: [/x.sh]\n",
		"unknown field":        "source: {image: i}\nscripts: [a.sh]\n",
	} {
		if _, err := Parse([]byte(bad)); err == nil || !strings.Contains(err.Error(), File) {
			t.Errorf("%s: want an error, got %v", name, err)
		}
	}
}
