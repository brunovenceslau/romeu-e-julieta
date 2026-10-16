// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package generated

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateProblems pins the rule of the one go:generate line on the
// forms that break it, among them the ones a reader of comments does
// not see and go generate runs: a line in a raw string or in a block
// comment.
func TestGenerateProblems(t *testing.T) {
	line := "//go:generate go run . generate"
	site := "package main\n\n" + line + "\n"
	tests := []struct {
		name  string
		files map[string]string
		ok    bool
	}{
		{"the directive", map[string]string{Site: site}, true},
		{"with trailing space", map[string]string{Site: site[:len(site)-1] + " \n"}, true},
		{"trailing spaces and tabs before a CRLF", map[string]string{Site: "package main\r\n\r\n" + line + " \t\r\n"}, true},
		{"a form feed at the end", map[string]string{Site: site[:len(site)-1] + "\f\n"}, false},
		{"a vertical tab at the end", map[string]string{Site: site[:len(site)-1] + "\v\n"}, false},
		{"a carriage return before a space is part of the word", map[string]string{Site: site[:len(site)-1] + "\r \n"}, false},
		{"a CRLF line", map[string]string{Site: "package main\r\n\r\n" + line + "\r\n"}, true},
		{"none", map[string]string{Site: "package main\n"}, false},
		{"no site file", map[string]string{"a.go": "package a\n"}, false},
		{"another command", map[string]string{Site: "package main\n\n//go:generate go run . generated\n"}, false},
		{"an added argument", map[string]string{Site: "package main\n\n" + line + " x\n"}, false},
		{"a tab after the word", map[string]string{Site: "package main\n\n//go:generate\tgo run . generate\n"}, false},
		{"twice", map[string]string{Site: site + line + "\n"}, false},
		{"a second command", map[string]string{Site: site + "\n// A.\n//go:generate echo hi\n"}, false},
		{"a second directive in a raw string of the site", map[string]string{Site: site + "\nvar s = `\n//go:generate echo hi\n`\n"}, false},
		{"a stray comment", map[string]string{Site: site, "a/a.go": "package a\n\n//go:generate echo hi\n"}, false},
		{"a stray in a raw string", map[string]string{Site: site, "zz/z.go": "package z\n\nvar s = `\n//go:generate echo hi\n`\n"}, false},
		{"a stray in a block comment", map[string]string{Site: site, "zz/z.go": "package z\n\n/*\n//go:generate echo hi\n*/\n"}, false},
		{"a stray with CRLF", map[string]string{Site: site, "zz/z.go": "package z\r\n\r\n//go:generate echo hi\r\n"}, false},
		{"the text is not a line start", map[string]string{Site: site, "zz/z.go": "package z\n\nvar s = \"//go:generate echo hi\"\n"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string][]byte{}
			for name, src := range tt.files {
				files[name] = []byte(src)
			}
			problems := generateProblems(files)
			assert.Equal(t, tt.ok, len(problems) == 0, "%v", problems)
		})
	}
}

// TestCheckGenerateDirectivesReadsEveryGoFile pins that the scan on disk
// reads what go generate would skip or might reach: testdata, vendor, a
// "_" directory, a file with a build constraint that excludes it, a
// test file and a link to a file; and leaves out the .git directories,
// a link to a directory and a dangling link.
func TestCheckGenerateDirectivesReadsEveryGoFile(t *testing.T) {
	stray := "package z\n\nvar s = `\n//go:generate echo hi\n`\n"
	site := "package main\n\n//go:generate go run . generate\n"
	for _, rel := range []string{
		"testdata/z.go", "vendor/z.go", "_x/z.go", ".hidden/z.go", "z_test.go",
		"z_windows.go", "sub/mod/z.go", "link.go",
	} {
		t.Run(rel, func(t *testing.T) {
			dir := t.TempDir()
			write := func(rel, src string) {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(src), 0o644))
			}
			write(Site, site)
			switch rel {
			case "link.go":
				write("real.txt", stray)
				require.NoError(t, os.Symlink("real.txt", filepath.Join(dir, rel)))
			case "sub/mod/z.go":
				write("sub/mod/go.mod", "module m\n")
				write(rel, stray)
			default:
				write(rel, stray)
			}
			problems, err := CheckDirectives(dir)
			require.NoError(t, err)
			require.Len(t, problems, 1)
			assert.Equal(t, rel, problems[0].Where)
		})
	}
	// What the scan leaves out: a .git directory at any depth (a nested
	// one, of a submodule or a vendored checkout, included), a dangling
	// link, and a link to a directory, which it does not enter: the
	// files of the target are read once, through the real path.
	t.Run("what the scan leaves out", func(t *testing.T) {
		dir := t.TempDir()
		write := func(rel, src string) {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(src), 0o644))
		}
		write(Site, site)
		write(".git/z.go", stray)
		write("sub/.git/z.go", stray)
		require.NoError(t, os.Symlink("missing", filepath.Join(dir, "dangling.go")))
		write("real/ok.go", "package ok\n")
		require.NoError(t, os.Symlink("real", filepath.Join(dir, "dirlink")))
		require.NoError(t, os.Symlink("dirlink", filepath.Join(dir, "chain")))
		require.NoError(t, os.Symlink(".git", filepath.Join(dir, "gitlink")))
		problems, err := CheckDirectives(dir)
		require.NoError(t, err)
		assert.Empty(t, problems)
	})
	t.Run("a link to a directory is not entered", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "real", "z.go"), []byte(stray), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(Site)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, Site), []byte(site), 0o644))
		require.NoError(t, os.Symlink("real", filepath.Join(dir, "dirlink")))
		problems, err := CheckDirectives(dir)
		require.NoError(t, err)
		require.Len(t, problems, 1, "the target is read once, by its real path")
		assert.Equal(t, "real/z.go", problems[0].Where)
	})
	t.Run("a link to a directory with a .go name is an error, not a pass", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(Site)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, Site), []byte(site), 0o644))
		require.NoError(t, os.Symlink("real", filepath.Join(dir, "dir.go")))
		_, err := CheckDirectives(dir)
		require.Error(t, err)
	})
}
