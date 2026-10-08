// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package adrdir_test

import (
	"os"
	"path/filepath"
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

func ptr(s string) *string { return &s }
