// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package adrdir_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/adrdir"
)

// TestName pins the shape of a record's filename.
func TestName(t *testing.T) {
	tests := []struct {
		file   string
		number string // empty means no match
	}{
		{"0001-first.md", "0001"},
		{"9999-last.md", "9999"},
		{"0007-a-b.md", "0007"},
		{"README.md", ""},
		{"12-short.md", ""},
		{"10000-five.md", ""},
		{"0001-.md", ""},
		{"0001-first.txt", ""},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			m := adrdir.Name.FindStringSubmatch(tt.file)
			if tt.number == "" {
				assert.Nil(t, m)
				return
			}
			require.NotNil(t, m)
			assert.Equal(t, tt.number, m[1])
		})
	}
}

// TestDir covers the directory .adr-dir names and each way it is wrong.
func TestDir(t *testing.T) {
	tests := []struct {
		name    string
		content *string // nil means no file
		want    string
		err     string
	}{
		{"a directory", ptr("docs/adr\n"), "docs/adr", ""},
		{"white space around it", ptr("  decisions \r\n"), "decisions", ""},
		{"a trailing slash", ptr("docs/adr/\n"), "docs/adr", ""},
		{"a leading dot", ptr("./docs/adr\n"), "docs/adr", ""},
		{"a doubled slash", ptr("docs//adr\n"), "docs/adr", ""},
		{"no file", nil, "", ".adr-dir"},
		{"an absolute path", ptr("/etc\n"), "", "not a directory inside the repository"},
		{"a path that climbs out", ptr("../out\n"), "", "not a directory inside the repository"},
		{"the .git directory", ptr(".git\n"), "", "inside the .git directory"},
		{"below .git", ptr(".git/adr\n"), "", "inside the .git directory"},
		{"below .git by a dot", ptr("./.git/adr\n"), "", "inside the .git directory"},
		{"below .GIT", ptr(".GIT/adr\n"), "", "inside the .git directory"},
		{"a name that starts like .git", ptr(".github/adr\n"), ".github/adr", ""},
		{"an empty file", ptr(""), "", "not a directory inside the repository"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.content != nil {
				require.NoError(t, os.WriteFile(filepath.Join(dir, adrdir.File), []byte(*tt.content), 0o644))
			}
			root, err := os.OpenRoot(dir)
			require.NoError(t, err)
			t.Cleanup(func() { _ = root.Close() })
			got, err := adrdir.Dir(root)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestDirFollowsLinks checks that a directory reached through symbolic
// links is judged by where the links lead: into .git is refused, a loop
// and a link out of the repository are errors, and a link to a place
// inside passes.
func TestDirFollowsLinks(t *testing.T) {
	type linkCase = struct {
		name  string
		named string
		links map[string]string // link path -> target
		dirs  []string
		err   string // empty means the directory passes
	}
	tests := []linkCase{
		{"a link to .git", "adr", map[string]string{"adr": ".git"}, nil, "inside the .git directory"},
		{"a link below .git", "adr", map[string]string{"adr": ".git/hooks"}, nil, "inside the .git directory"},
		{"a link to .GIT", "adr", map[string]string{"adr": ".GIT"}, nil, "inside the .git directory"},
		{"a chain of links", "a", map[string]string{"a": "b", "b": "c", "c": ".git"}, nil, "inside the .git directory"},
		{"a link on the way", "x/adr", map[string]string{"x": ".git"}, nil, "inside the .git directory"},
		{"a link that climbs to .git", "docs/adr", map[string]string{"docs/adr": "../.git"}, []string{"docs"}, "inside the .git directory"},
		{"a dangling link to a place in .git", "adr", map[string]string{"adr": ".git/missing"}, nil, "inside the .git directory"},
		{"a link to a directory inside", "adr", map[string]string{"adr": "docs/adr"}, []string{"docs/adr"}, ""},
		{"a dangling link inside the repository", "adr", map[string]string{"adr": "nowhere"}, nil, ""},
		{"a .git below a directory", "sub/.git/x", nil, []string{"sub/.git"}, "inside the .git directory"},
		{"a link to a .git below a directory", "adr", map[string]string{"adr": "sub/.git"}, []string{"sub/.git"}, "inside the .git directory"},
		{"the name as written, before any link", ".git/l", map[string]string{".git/l": "/etc"}, nil, `names ".git/l", which is inside the .git directory`},
		{"a trailing slash", "adr/", map[string]string{"adr": ".git"}, nil, "inside the .git directory"},
		{"a missing component and then ..", "adr", map[string]string{"adr": "missing/../.git"}, nil, "inside the .git directory"},
		{"a link to a regular file", "adr", map[string]string{"adr": "file"}, nil, ""},
		{"a loop", "a", map[string]string{"a": "b", "b": "a"}, nil, "too many links"},
		{"an absolute target", "adr", map[string]string{"adr": "/etc"}, nil, "leads out of the repository"},
		{"a target that climbs out", "adr", map[string]string{"adr": ".."}, nil, "leads out of the repository"},
	}
	// A chain of forty links (the last one included) passes, forty-one fail.
	chain := func(n int) map[string]string {
		links := map[string]string{"real": "docs"}
		prev := "real"
		for i := range n {
			name := "l" + strconv.Itoa(i)
			links[name] = prev
			prev = name
		}
		links["adr"] = prev
		return links
	}
	tests = append(tests,
		linkCase{"forty links", "adr", chain(38), []string{"docs"}, ""},
		linkCase{"forty-one links", "adr", chain(39), []string{"docs"}, "too many links"},
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "file"), []byte("x"), 0o644))
			for _, d := range tt.dirs {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, d), 0o755))
			}
			for link, target := range tt.links {
				require.NoError(t, os.Symlink(target, filepath.Join(dir, link)))
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, adrdir.File), []byte(tt.named+"\n"), 0o644))
			root, err := os.OpenRoot(dir)
			require.NoError(t, err)
			t.Cleanup(func() { _ = root.Close() })
			got, err := adrdir.Dir(root)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, filepath.Clean(tt.named), got)
		})
	}
}

func ptr(s string) *string { return &s }
