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
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The registry client reads anonymously (REQ-0210, REQ-0515, I28). The
// library it uses can read the Docker config and run credential helpers
// (authn.DefaultKeychain, the packages that build on it); this test holds
// that nothing in the module reaches them.

const ggcr = "github.com/google/go-containerregistry/"

// allowedFromLibrary are the packages of the library the module may
// import: names, the v1 types, the remote client and its transport, and
// authn for authn.Anonymous. The packages that build a keychain (crane,
// the cloud and Kubernetes helpers) are not on the list.
var allowedFromLibrary = []string{
	ggcr + "pkg/authn", ggcr + "pkg/name", ggcr + "pkg/v1",
	ggcr + "pkg/v1/remote", ggcr + "pkg/v1/remote/transport",
}

// keychainNames are the selectors that bring credentials into a request.
var keychainNames = []string{"DefaultKeychain", "NewMultiKeychain", "WithAuthFromKeychain", "NewKeychainFromHelper", "Basic", "Bearer", "FromConfig"}

// findCredentialUse returns the files below the roots of root that use
// the library beyond the rules above, and each why.
func findCredentialUse(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	for _, top := range []string{"internal", "cmd", "tools", "e2e"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(p string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			file, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			names := map[string]string{} // local name -> import path
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if !strings.HasPrefix(path, ggcr) {
					continue
				}
				if !slices.Contains(allowedFromLibrary, path) {
					out = append(out, rel+" imports "+path)
					continue
				}
				if strings.HasSuffix(path, "/remote") && !strings.HasPrefix(rel, "internal/oci/") {
					out = append(out, rel+" imports the remote client outside internal/oci")
				}
				local := path[strings.LastIndex(path, "/")+1:]
				if imp.Name != nil {
					local = imp.Name.Name
				}
				names[local] = path
			}
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || !slices.Contains(keychainNames, sel.Sel.Name) {
					return true
				}
				if path, ok := names[pkg.Name]; ok && strings.HasSuffix(path, "/authn") || path == ggcr+"pkg/v1/remote" {
					out = append(out, rel+" uses "+pkg.Name+"."+sel.Sel.Name)
				}
				return true
			})
			return nil
		})
		require.NoError(t, err)
	}
	slices.Sort(out)
	return out
}

func TestNoKeychainAnywhere(t *testing.T) {
	assert.Empty(t, findCredentialUse(t, "../.."), "the module must not give the library a credential or a keychain (REQ-0210)")
}

func TestTheScannerFindsCredentialUse(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(src), 0o600))
	}
	write("internal/oci/ok.go", "package oci\nimport (\n\t\"github.com/google/go-containerregistry/pkg/authn\"\n\t\"github.com/google/go-containerregistry/pkg/v1/remote\"\n)\nvar _ = remote.WithAuth(authn.Anonymous)\n")
	write("internal/a/keychain.go", "package a\nimport \"github.com/google/go-containerregistry/pkg/authn\"\nvar _ = authn.DefaultKeychain\n")
	write("internal/b/renamed.go", "package b\nimport k \"github.com/google/go-containerregistry/pkg/authn\"\nvar _ = k.NewMultiKeychain\n")
	write("internal/c/crane.go", "package c\nimport _ \"github.com/google/go-containerregistry/pkg/crane\"\n")
	write("internal/d/remote.go", "package d\nimport \"github.com/google/go-containerregistry/pkg/v1/remote\"\nvar _ = remote.Get\n")
	write("tools/e/helper.go", "package e\nimport _ \"github.com/google/go-containerregistry/pkg/authn/github\"\n")
	write("internal/oci/fromkeychain.go", "package oci\nimport \"github.com/google/go-containerregistry/pkg/v1/remote\"\nvar _ = remote.WithAuthFromKeychain\n")

	got := findCredentialUse(t, root)

	assert.Equal(t, []string{
		"internal/a/keychain.go uses authn.DefaultKeychain",
		"internal/b/renamed.go uses k.NewMultiKeychain",
		"internal/c/crane.go imports github.com/google/go-containerregistry/pkg/crane",
		"internal/d/remote.go imports the remote client outside internal/oci",
		"internal/oci/fromkeychain.go uses remote.WithAuthFromKeychain",
		"tools/e/helper.go imports github.com/google/go-containerregistry/pkg/authn/github",
	}, got)
}
