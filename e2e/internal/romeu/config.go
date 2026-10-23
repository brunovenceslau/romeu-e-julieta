// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PushConfigRepo creates the config repo owner/name on the origins: one
// commit on main that holds projects/<name>.yaml, the repo's own spec
// with itself as the primary repo (e2e/testdata/origins/
// config-project.yaml). It returns the repo's URL. files are added to
// the commit, and a file of the same path replaces the spec.
func (o *Origins) PushConfigRepo(t testing.TB, owner, name string, files map[string]string) string {
	t.Helper()
	tmpl, err := os.ReadFile(filepath.Join(repoRoot(t), "e2e", "testdata", "origins", "config-project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.NewReplacer("{{owner}}", owner, "{{name}}", name).Replace(string(tmpl))
	all := map[string]string{"projects/" + name + ".yaml": spec}
	for p, c := range files {
		all[p] = c
	}
	o.Push(t, owner+"/"+name, "main", all)
	return o.URL(owner + "/" + name)
}
