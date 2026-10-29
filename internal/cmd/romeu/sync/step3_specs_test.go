// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// projectYAML returns the spec of the project name, whose primary repo
// is https://github.com/owner/<repo>, with the secrets given.
func projectYAML(name, repo string, secrets ...string) string {
	s := "schema: project/v1\nname: " + name + "\nrepos:\n  - url: https://github.com/owner/" + repo + "\n    primary: true\n" +
		"workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5\nagent: sbx-kit-claude\n"
	if len(secrets) > 0 {
		s += "secrets:\n"
		for _, x := range secrets {
			s += "  - " + x + "\n"
		}
	}
	return s
}

// specRun returns a run whose steps 1 and 2 are done on a config repo
// whose origin holds files at its head, the spec commit, with the flags
// f; the host settings allow github.com, the workload of projectYAML,
// and bind github@other.
func specRun(t *testing.T, files map[string]string, f flags) *run {
	t.Helper()
	c := newConfig(t, files)
	c.settings.GitHosts = []spec.GitHost{{Host: "github.com"}}
	c.settings.WorkloadRepositories = []string{"docker.io/docker/sbx-kit-claude"}
	c.settings.Secrets = map[string]spec.Secret{"github@other": {Service: "github", Argv: []string{"/usr/bin/true"}}}
	r := c.run(f)
	g, err := hostGit(c.settings, c.home)
	require.NoError(t, err)
	r.git, r.configClone, r.specCommit = g, c.clone, gitIn(t, c.origin, "rev-parse", "HEAD")
	return r
}

// TestReadSpecs is step 3 of 04 4.2: the specs at the spec commit,
// validated together, and the targets of the command line.
func TestReadSpecs(t *testing.T) {
	cfg := projectYAML("cfg", "cfg")
	tests := []struct {
		name  string
		files map[string]string
		flags flags
		// want are the names of the targets, when step 3 passes.
		want []string
		// id is the error id step 3 stops with, "" for none, and
		// mentions what its text names.
		id       string
		mentions []string
	}{
		{
			name:  "the projects named",
			files: map[string]string{"projects/cfg.yaml": cfg, "projects/shop.yaml": projectYAML("shop", "shop-api")},
			flags: flags{names: []string{"shop"}},
			want:  []string{"shop"},
		},
		{
			name:  "--all, in the order of projects/",
			files: map[string]string{"projects/cfg.yaml": cfg, "projects/shop.yaml": projectYAML("shop", "shop-api"), "projects/blog.yaml": projectYAML("blog", "blog")},
			flags: flags{all: true},
			want:  []string{"blog", "cfg", "shop"},
		},
		{
			name:  "a name the spec lacks",
			files: map[string]string{"projects/cfg.yaml": cfg},
			flags: flags{names: []string{"cfg", "shop"}},
			id:    "RJ-349", mentions: []string{"project-unknown", "shop"},
		},
		{
			name:  "no name and no --all",
			files: map[string]string{"projects/cfg.yaml": cfg},
			id:    "RJ-349", mentions: []string{"targets"},
		},
		{
			name:  "--all and a name",
			files: map[string]string{"projects/cfg.yaml": cfg},
			flags: flags{all: true, names: []string{"cfg"}},
			id:    "RJ-349", mentions: []string{"targets"},
		},
		{
			name: "two specs with one name and two with one URL list every error",
			files: map[string]string{
				"projects/cfg.yaml":  cfg,
				"projects/shop.yaml": projectYAML("shop", "shop-api"),
				"projects/twin.yaml": projectYAML("shop", "twin"),
				"projects/blog.yaml": projectYAML("blog", "shop-api"),
			},
			flags: flags{all: true},
			id:    "RJ-349", mentions: []string{"spec-invalid", "project-name-unique", "project-name-stem", "repo-url-unique", "projects/twin.yaml", "projects/shop.yaml"},
		},
		{
			name:  "a file under projects/ that is not *.yaml",
			files: map[string]string{"projects/cfg.yaml": cfg, "projects/README.md": "notes"},
			flags: flags{all: true},
			id:    "RJ-345", mentions: []string{"projects/README.md"},
		},
		{
			name:  "no spec whose primary repo is config.url",
			files: map[string]string{"projects/shop.yaml": projectYAML("shop", "shop-api")},
			flags: flags{all: true},
			id:    "RJ-346", mentions: []string{"config.url"},
		},
		{
			name:  "a spec that does not decode is one error among the others",
			files: map[string]string{"projects/cfg.yaml": cfg, "projects/bad.yaml": "schema: project/v1\nname: bad\nunknown: 1\n", "projects/notes.txt": "x"},
			flags: flags{all: true},
			id:    "RJ-345", mentions: []string{"projects/bad.yaml", "projects/notes.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			r := specRun(t, tt.files, tt.flags)

			err := readSpecs(t.Context(), r)

			if tt.id != "" {
				require.Error(t, err)
				is.Equal(tt.id, cli.Of(err).ID, err.Error())
				is.Equal(2, cli.ExitCode(err))
				for _, m := range tt.mentions {
					is.Contains(err.Error(), m)
				}
				is.Nil(r.targets)
				return
			}
			require.NoError(t, err)
			var got []string
			for _, p := range r.targets {
				got = append(got, p.Name)
			}
			is.Equal(tt.want, got)
		})
	}
}

// TestReadSpecsRefusesALinkUnderProjects: a symbolic link under
// projects/ is refused by the hardened read of the tree, exit 2, and
// is never followed.
func TestReadSpecsRefusesALinkUnderProjects(t *testing.T) {
	c := newConfig(t, map[string]string{"projects/cfg.yaml": projectYAML("cfg", "cfg")})
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(c.origin, "projects", "link.yaml")))
	head := commitFiles(t, c.origin, nil)
	gitIn(t, c.clone, "fetch", "-q", c.origin, "+refs/heads/*:refs/romeu/origin/cfg/*")
	r := c.run(flags{all: true})
	g, err := hostGit(c.settings, c.home)
	require.NoError(t, err)
	r.git, r.configClone, r.specCommit = g, c.clone, head

	err = readSpecs(t.Context(), r)

	require.Error(t, err)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, err.Error(), "projects-refused")
}

// TestCheckSecrets: a target's spec secret needs the binding
// <secret>@<project> of that project exactly (I8); github@other does
// not satisfy project shop, and the hint names the entry to paste. A
// project that is not a target is not checked.
//
//romeu:invariant I8 test
func TestCheckSecrets(t *testing.T) {
	files := map[string]string{
		"projects/cfg.yaml":   projectYAML("cfg", "cfg"),
		"projects/shop.yaml":  projectYAML("shop", "shop-api", "github", "npm"),
		"projects/other.yaml": projectYAML("other", "other", "github"),
	}
	tests := []struct {
		name  string
		names []string
		// unbound are the entries the error names, none when step 3
		// passes.
		unbound []string
	}{
		{name: "github@other does not satisfy shop", names: []string{"shop"}, unbound: []string{"github@shop", "npm@shop"}},
		{name: "the exact binding satisfies other", names: []string{"other"}},
		{name: "a project that is not a target is not checked", names: []string{"cfg"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			r := specRun(t, files, flags{names: tt.names})

			err := readSpecs(t.Context(), r)

			if tt.unbound == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			e := cli.Of(err)
			is.Equal("RJ-315", e.ID)
			is.Equal(2, cli.ExitCode(err))
			for _, k := range tt.unbound {
				is.Contains(err.Error(), k)
			}
			is.Contains(e.Hint(), "add the entry github@shop")
			is.Contains(e.Hint(), "romeu sync shop")
		})
	}
}

// TestReadSpecsRefusesACommitFsckRefuses: a spec commit that git fsck
// --strict refuses, here one whose author has no email, is exit 2
// before projects/ is read (I27).
func TestReadSpecsRefusesACommitFsckRefuses(t *testing.T) {
	r := specRun(t, map[string]string{"projects/cfg.yaml": projectYAML("cfg", "cfg")}, flags{all: true})
	tree := gitIn(t, r.configClone, "rev-parse", r.specCommit+"^{tree}")
	body := "tree " + tree + "\nauthor nobody 0 +0000\ncommitter nobody 0 +0000\n\nx\n"
	f := filepath.Join(t.TempDir(), "commit")
	require.NoError(t, os.WriteFile(f, []byte(body), 0o600))
	r.specCommit = gitIn(t, r.configClone, "hash-object", "--literally", "-t", "commit", "-w", f)

	err := readSpecs(t.Context(), r)

	require.Error(t, err)
	assert.Equal(t, 2, cli.ExitCode(err))
	assert.Contains(t, err.Error(), "spec-commit-unsafe")
	assert.Nil(t, r.targets)
}
