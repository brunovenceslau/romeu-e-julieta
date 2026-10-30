// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDoctorTreeMode: a <name>-env/ or the .attic that its group or
// others can access fails tree-mode (REQ-0377).
func TestDoctorTreeMode(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"shop-env", ".attic"} {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			p := filepath.Join(f.root, dir)
			require.NoError(t, os.Mkdir(p, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(p, neverIndex), nil, 0o600))
			require.NoError(t, os.Chmod(p, 0o750))

			rep, out, err := f.output(t)

			c := check(t, rep, "tree-mode")
			assert.Equal(t, resultFail, c.Result, out)
			assert.Contains(t, c.Message, dir)
			assert.Equal(t, 6, exitOf(err))
		})
	}
}

// TestDoctorTreeModeReportsALink: a link in place of a <name>-env/ or of
// the .attic fails tree-mode as a link, not the directory romeu made,
// and is not followed.
func TestDoctorTreeModeReportsALink(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"shop-env", ".attic"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			target := filepath.Join(f.tmp, "elsewhere")
			require.NoError(t, os.Mkdir(target, 0o700))
			require.NoError(t, os.Symlink(target, filepath.Join(f.root, name)))

			rep, out, _ := f.output(t)

			c := check(t, rep, "tree-mode")
			assert.Equal(t, resultFail, c.Result, out)
			assert.Contains(t, c.Message, name+" is a link, not the directory romeu made")
		})
	}
}
