// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decoders are the imports that read a document into Go values.
var decoders = []string{"go.yaml.in/yaml/v3", "encoding/json/v2"}

// exempt are the packages that may decode and name a capabilities member:
// internal/oci is the one parser of descriptors, and internal/spec
// decodes the files a person writes.
var exempt = []string{"internal/oci", "internal/spec"}

// capabilitiesTag is a struct tag that names the member, in any tag
// style (json, yaml, toml, mapstructure).
var capabilitiesTag = regexp.MustCompile(`(?i)(^|[:"',\s])capabilities([",\s]|$)`)

// findSecondParsers returns the packages below root/internal and
// root/cmd, outside exempt, that import a decoder and name a
// capabilities member (REQ-0570): a struct field whose tag names it or
// that is called Capabilities, or the string "capabilities". A field of
// the type oci.Capabilities is a record of a parsed set, not a reader,
// and does not count.
func findSecondParsers(t *testing.T, root string) []string {
	t.Helper()
	type pkg struct{ decodes, names bool }
	pkgs := map[string]*pkg{}
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // a tree without this top directory
			}
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return err
			}
			dir, _ := filepath.Rel(root, filepath.Dir(p))
			dir = filepath.ToSlash(dir)
			if slices.ContainsFunc(exempt, func(e string) bool { return dir == e || strings.HasPrefix(dir, e+"/") }) {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			if pkgs[dir] == nil {
				pkgs[dir] = &pkg{}
			}
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if slices.Contains(decoders, path) {
					pkgs[dir].decodes = true
				}
			}
			ast.Inspect(file, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.Field:
					if namesCapabilities(n) && !isOCICapabilities(n.Type) {
						pkgs[dir].names = true
					}
				case *ast.BasicLit:
					if n.Kind == token.STRING {
						if s, err := strconv.Unquote(n.Value); err == nil && strings.EqualFold(s, "capabilities") {
							pkgs[dir].names = true
						}
					}
				}
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
	var out []string
	for dir, p := range pkgs {
		if p.decodes && p.names {
			out = append(out, dir)
		}
	}
	slices.Sort(out)
	return out
}

// namesCapabilities reports whether a struct field is called Capabilities
// or has a tag that names the member.
func namesCapabilities(f *ast.Field) bool {
	for _, name := range f.Names {
		if strings.EqualFold(name.Name, "capabilities") {
			return true
		}
	}
	if f.Tag != nil {
		tag, err := strconv.Unquote(f.Tag.Value)
		return err == nil && capabilitiesTag.MatchString(tag)
	}
	return false
}

// isOCICapabilities reports whether t is oci.Capabilities or a pointer
// to it.
func isOCICapabilities(t ast.Expr) bool {
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "oci" && sel.Sel.Name == "Capabilities"
}

// TestNoSecondDescriptorParser holds REQ-0570 (06 6.1, I28): internal/oci
// owns the one descriptor grammar, and render calls it for local kits. A
// product package (internal/** and cmd/**, the code the binaries link)
// outside internal/oci and internal/spec that imports a decoder of YAML or
// JSON and names a capabilities member fails here, so a second parser
// cannot grow beside it. Files of tools/ build and judge the repository
// and are not linked into the binaries.
func TestNoSecondDescriptorParser(t *testing.T) {
	assert.Empty(t, findSecondParsers(t, "../.."), "a package reads a kit descriptor itself; call internal/oci (REQ-0570)")
}

func TestTheScannerFindsASecondParser(t *testing.T) {
	write := func(t *testing.T, root, rel, src string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(src), 0o600))
	}
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"a yaml tag", map[string]string{"internal/render/k.go": "package render\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\ntype K struct {\n\tC []any `yaml:\"capabilities\"`\n}\n"}, []string{"internal/render"}},
		{"a json tag with options", map[string]string{"internal/render/k.go": "package render\nimport json \"encoding/json/v2\"\nvar _ = json.Unmarshal\ntype K struct {\n\tC []any `json:\"capabilities,omitzero\"`\n}\n"}, []string{"internal/render"}},
		{"a toml tag", map[string]string{"internal/render/k.go": "package render\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\ntype K struct {\n\tC []any `toml:\"Capabilities\"`\n}\n"}, []string{"internal/render"}},
		{"a field with no tag", map[string]string{"internal/render/k.go": "package render\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\ntype K struct {\n\tCapabilities []any\n}\n"}, []string{"internal/render"}},
		{"the string, in another file of the package", map[string]string{
			"cmd/romeu/a.go": "package main\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\n",
			"cmd/romeu/b.go": "package main\nvar key = \"capabilities\"\n",
		}, []string{"cmd/romeu"}},
		{"a package that names it and decodes nothing", map[string]string{"internal/render/k.go": "package render\ntype K struct {\n\tC []any `json:\"capabilities\"`\n}\n"}, nil},
		{"a package that decodes and does not name it", map[string]string{"internal/render/k.go": "package render\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\ntype K struct {\n\tCaps []any `yaml:\"caps\"`\n}\n"}, nil},
		{"a record of a parsed set", map[string]string{"internal/state/r.go": "package state\nimport (\n\tjson \"encoding/json/v2\"\n\t\"x/internal/oci\"\n)\nvar _ = json.Unmarshal\ntype D struct {\n\tCapabilities *oci.Capabilities `json:\"capabilities\"`\n}\n"}, nil},
		{"internal/oci and internal/spec", map[string]string{
			"internal/oci/k.go":  "package oci\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\nvar key = \"capabilities\"\n",
			"internal/spec/k.go": "package spec\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\nvar key = \"capabilities\"\n",
		}, nil},
		{"a test file", map[string]string{"internal/render/k_test.go": "package render\nimport \"go.yaml.in/yaml/v3\"\nvar _ = yaml.Unmarshal\nvar key = \"capabilities\"\n"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for rel, src := range tt.files {
				write(t, root, rel, src)
			}

			assert.Equal(t, tt.want, findSecondParsers(t, root))
		})
	}
}
