// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package identifiers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestIdentifiersOfThisRepository holds every identifier the
// specification names in tools/ci to a declaration there.
func TestIdentifiersOfThisRepository(t *testing.T) {
	root := citest.ModuleRoot(t)
	refs, err := Refs(filepath.Join(root, specDir))
	require.NoError(t, err)
	assert.NotEmpty(t, refs, "the specification names identifiers of tools/ci")
	findings, err := Run(root)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// fixtureCode declares one name of each kind at the top level, a method,
// and a name only a test file declares.
var fixtureCode = []string{
	"tools/ci/internal/mise/mise.go", "package mise\n\nvar HookAllow = []string{}\n\nconst A, B = 1, 2\n\ntype T int\n\nfunc F() {}\n\nfunc (T) M() {}\n",
	"tools/ci/internal/mise/mise_test.go", "package mise\n\nvar OnlyInTest = 1\n",
	"tools/ci/internal/mise/testdata/x.go", "package mise\n\nvar InTestdata = 1\n",
	"tools/ci/.hidden/x.go", "package mise\n\nvar Hidden = 1\n",
	"tools/ci/internal/lint/lint.go", "package lint\n\nvar Args = []string{}\n",
}

// newFixture writes the code of fixtureCode and the files given, and
// returns the root.
func newFixture(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	all := append(append([]string{}, fixtureCode...), files...)
	for i := 0; i < len(all); i += 2 {
		p := filepath.Join(root, all[i])
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(all[i+1]), 0o644))
	}
	return root
}

// TestIdentifiersFindings covers each case, a fixture each way.
func TestIdentifiersFindings(t *testing.T) {
	tests := []struct {
		name string
		page string
		want []string
	}{
		{"every kind of declaration", "`HookAllow` of the mise package of `tools/ci`, `A` and `B` of the mise package of `tools/ci`, `T` of the mise package of `tools/ci`, `F` of the mise package of `tools/ci` and `Args` in the lint package of `tools/ci`.", nil},
		{"a reference that wraps", "The list is `HookAllow` of\nthe mise\n  package of\n`tools/ci`.", nil},
		{"a name the package lacks", "`hookAllow` of the mise package of `tools/ci`", []string{
			"a.md: names hookAllow of the mise package of tools/ci, which that package does not declare",
		}},
		{"a package tools/ci lacks", "`approvalLine` of the pr package of `tools/ci`", []string{
			"a.md: names approvalLine of the pr package of tools/ci, and tools/ci has no package pr",
		}},
		{"a method is no top-level name", "`M` of the mise package of `tools/ci`", []string{
			"a.md: names M of the mise package of tools/ci, which that package does not declare",
		}},
		{"a name only a test file declares", "`OnlyInTest` of the mise package of `tools/ci`", []string{
			"a.md: names OnlyInTest of the mise package of tools/ci, which that package does not declare",
		}},
		{"a name only testdata or a hidden directory declares", "`InTestdata` and `Hidden`: `InTestdata` of the mise package of `tools/ci`, `Hidden` of the mise package of `tools/ci`", []string{
			"a.md: names Hidden of the mise package of tools/ci, which that package does not declare",
			"a.md: names InTestdata of the mise package of tools/ci, which that package does not declare",
		}},
		{"the same miss twice is one line", "`x` of the mise package of `tools/ci`; again `x` of the mise package of `tools/ci`", []string{
			"a.md: names x of the mise package of tools/ci, which that package does not declare",
		}},
		{"another form is not read", "the threat model of the mise package of `tools/ci`; `x` of the mise package of `tools/new`", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newFixture(t, "docs/spec/a.md", tt.page+"\n")
			findings, err := Run(root)
			require.NoError(t, err)
			assert.Equal(t, tt.want, findings)
		})
	}
}

// TestIdentifiersReadsOnlyTheSpecification checks that a record under
// docs/adr/ and a page below docs/spec/ are not read: a record is
// history and may name what was renamed since.
func TestIdentifiersReadsOnlyTheSpecification(t *testing.T) {
	stale := "`hookAllow` of the mise package of `tools/ci`\n"
	root := newFixture(t,
		"docs/spec/a.md", "`HookAllow` of the mise package of `tools/ci`\n",
		"docs/spec/old/b.md", stale,
		"docs/adr/0001-x.md", stale,
		"docs/spec.md", stale,
	)
	findings, err := Run(root)
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// TestIdentifiersErrors covers the inputs it cannot read.
func TestIdentifiersErrors(t *testing.T) {
	t.Run("no page", func(t *testing.T) {
		_, err := Run(newFixture(t))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no page in")
	})
	t.Run("a page that is a directory", func(t *testing.T) {
		root := newFixture(t, "docs/spec/a.md/x", "x\n")
		_, err := Run(root)
		require.Error(t, err)
	})
	t.Run("a Go file that does not parse", func(t *testing.T) {
		_, err := Run(newFixture(t, "docs/spec/a.md", "x\n", "tools/ci/broken.go", "package"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})
	t.Run("no tools/ci", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "spec"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "spec", "a.md"), []byte("x\n"), 0o644))
		_, err := Run(root)
		require.Error(t, err)
	})
	t.Run("a bad glob root", func(t *testing.T) {
		_, err := Refs("[")
		require.Error(t, err)
	})
}
