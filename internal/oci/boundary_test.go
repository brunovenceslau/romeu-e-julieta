// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// descriptorMarkers are the texts only a reader of a kit descriptor has
// in its source: a capability type of the kit schema, the annotation of
// an image, and the key of the capability list as a YAML field tag.
var descriptorMarkers = []string{
	"com.docker.sandbox/",
	"vnd.docker.sandbox.kit",
	`yaml:"capabilities`,
}

// TestNoSecondDescriptorParser holds REQ-0570 (06 6.1, I28): internal/oci
// owns the one descriptor grammar, and render calls it for local kits. A
// Go file of the product outside internal/oci (internal/** and cmd/**,
// the code the binaries link) that carries a marker of a descriptor
// reader fails this test, so a second parser cannot grow beside it.
// Files of tools/ build and judge the repository and do not link into
// the binaries; they are not product code.
func TestNoSecondDescriptorParser(t *testing.T) {
	root := "../.."
	var checked int
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if d.IsDir() && rel == "internal/oci" {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			checked++
			for _, m := range descriptorMarkers {
				assert.NotContains(t, string(src), m, "%s reads a kit descriptor itself; call internal/oci (REQ-0570)", rel)
			}
			return nil
		})
		require.NoError(t, err)
	}
	assert.Positive(t, checked, "the walk found the product's Go files")
}

// TestSecondDescriptorParserIsCaught proves the marker scan fails on a
// file that has one: the rule is not vacuous.
func TestSecondDescriptorParserIsCaught(t *testing.T) {
	src := "package render\n\ntype kit struct {\n\tCapabilities []any `yaml:\"capabilities\"`\n}\n"
	var hit bool
	for _, m := range descriptorMarkers {
		hit = hit || strings.Contains(src, m)
	}
	assert.True(t, hit)
}
