// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package license

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// The license texts of the fixtures: the opening words of each, enough
// for its markers, wrapped and indented as license files are.
const (
	mitText  = "The MIT License\n\nPermission is hereby granted, free of charge,\n  to any person obtaining a copy of this software.\nTHE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND.\n"
	bsd4Text = "Redistribution and use in source and binary forms are permitted.\nAll advertising materials mentioning features or use of this software\nmust display an acknowledgement. Neither the name nor the names of its\ncontributors may be used to endorse or promote products derived from this software.\n"
	cddlText = "COMMON DEVELOPMENT AND DISTRIBUTION LICENSE (CDDL)\nVersion 1.0\n"
	ownText  = "Copyright the authors. All rights reserved; ask before you use it.\n"
	bsd3Text = "Redistribution and use in source and binary forms, with or without\nmodification, are permitted. Neither the name of the project nor the\nnames of its contributors may be used to endorse or promote products\nderived from this software.\n"
	bsd2Text = "Redistribution and use in source and binary forms, with or without\nmodification, are permitted.\n"
)

// TestTheAllowlist parses the committed allowlist: each license has a
// reason, and the licenses the product may take in are allowed.
func TestTheAllowlist(t *testing.T) {
	a := committedAllowlist(t)
	var allowed, disallowed []string
	for _, l := range a.Licenses {
		assert.NotEmpty(t, strings.TrimSpace(l.Reason), l.ID)
		if l.Allowed {
			allowed = append(allowed, l.ID)
		} else {
			disallowed = append(disallowed, l.ID)
		}
	}
	assert.Equal(t, []string{"Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MIT"}, allowed)
	assert.Equal(t, []string{"BSD-4-Clause", "GPL-2.0", "CDDL-1.0", "EPL-1.0"}, disallowed)
	var riders []string
	for _, r := range a.Riders {
		riders = append(riders, r.Marker)
	}
	assert.Equal(t, []string{"Commons Clause", "non-commercial", "Business Source", "GNU Affero", "Mozilla Public License", "Lesser General Public"}, riders)
}

// TestParseAllowlistRefuses shows each malformed allowlist refused.
func TestParseAllowlistRefuses(t *testing.T) {
	for name, c := range map[string]struct{ src, want string }{
		"unknown key":  {"licenses:\n  - id: MIT\n    allowed: true\n    reason: r\n    markers: [m]\n    note: x\n", "field note not found"},
		"no id":        {"licenses:\n  - allowed: true\n    reason: r\n    markers: [m]\n", "entry 1 has no SPDX id"},
		"bad id":       {"licenses:\n  - id: M I T\n    reason: r\n    markers: [m]\n", "entry 1 has no SPDX id"},
		"twice":        {"licenses:\n  - {id: MIT, reason: r, markers: [m]}\n  - {id: MIT, reason: r, markers: [m]}\n", "MIT is listed twice"},
		"no reason":    {"licenses:\n  - {id: MIT, reason: ' ', markers: [m]}\n", "MIT has no reason"},
		"no marker":    {"licenses:\n  - {id: MIT, reason: r}\n", "MIT needs markers, none of them empty"},
		"empty marker": {"licenses:\n  - {id: MIT, reason: r, markers: [m], unless: ['--']}\n", "MIT needs markers, none of them empty"},
		"empty list":   {"licenses: []\n", "lists no license"},
		"rider marker": {"licenses:\n  - {id: MIT, reason: r, markers: [m]}\nriders:\n  - {marker: '', reason: r}\n", "rider 1 needs a marker and a reason"},
		"rider reason": {"licenses:\n  - {id: MIT, reason: r, markers: [m]}\nriders:\n  - {marker: m}\n", "rider 1 needs a marker and a reason"},
	} {
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
			_, err := ParseAllowlist([]byte(c.src))
			require.Error(t, err)
			assert.ErrorContains(t, err, c.want)
		})
	}
}

// TestIdentify names the licenses of a text by their markers, whatever
// the case, the wrapping and the punctuation, on word boundaries, and
// tells the two BSD licenses apart by the endorsement clause.
func TestIdentify(t *testing.T) {
	a := committedAllowlist(t)
	assert.Equal(t, []string{"MIT"}, a.Identify([]byte(mitText)))
	assert.Equal(t, []string{"MIT"}, a.Identify([]byte(strings.ToUpper(strings.ReplaceAll(mitText, " ", "\n> ")))))
	assert.Equal(t, []string{"BSD-3-Clause"}, a.Identify([]byte(bsd3Text)))
	assert.Equal(t, []string{"BSD-2-Clause"}, a.Identify([]byte(bsd2Text)))
	assert.Equal(t, []string{"CDDL-1.0"}, a.Identify([]byte(cddlText)))
	assert.Equal(t, []string{"BSD-4-Clause"}, a.Identify([]byte(bsd4Text)), "the advertising clause is not read as BSD-3")
	assert.Equal(t, []string{"Apache-2.0", "MIT"}, a.Identify([]byte("Licensed under the Apache License, Version 2.0.\n"+mitText)))
	assert.Empty(t, a.Identify([]byte(ownText)))
	assert.Empty(t, a.Identify([]byte("Apache Licenses, Version 2.0")), "a marker matches whole words")
}

// writeModule writes the files of a fixture module and returns it.
func writeModule(t *testing.T, path string, files map[string]string) Module {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	}
	return Module{Path: path, Version: "v1.0.0", Dir: dir}
}

// TestJudgeModules passes a module under an allowed license and fails
// a disallowed and an unknown one, a module without a license file,
// and a module that copies in code under a disallowed license.
func TestJudgeModules(t *testing.T) {
	a := committedAllowlist(t)
	mods := []Module{
		writeModule(t, "example.com/allowed", map[string]string{"LICENSE": mitText, "main.go": "package main\n", "NOTICE": ownText}),
		writeModule(t, "example.com/disallowed", map[string]string{"LICENSE.txt": cddlText}),
		writeModule(t, "example.com/unknown", map[string]string{"COPYING": ownText}),
		writeModule(t, "example.com/none", map[string]string{"main.go": "package main\n"}),
		writeModule(t, "example.com/nested", map[string]string{"LICENSE.md": mitText, "third_party/x/Licence": cddlText}),
	}
	got, err := JudgeModules(mods, a)
	require.NoError(t, err)
	assert.Equal(t, []check.Finding{
		{Where: "example.com/disallowed@v1.0.0", Rule: "license disallowed", Msg: "LICENSE.txt is under CDDL-1.0, which the allowlist does not allow with GPL-3.0-only"},
		{Where: "example.com/unknown@v1.0.0", Rule: "license unknown", Msg: "COPYING names no license of the allowlist"},
		{Where: "example.com/none@v1.0.0", Rule: "license unknown", Msg: "has no LICENSE, LICENCE or COPYING file"},
		{Where: "example.com/nested@v1.0.0", Rule: "license disallowed", Msg: "third_party/x/Licence is under CDDL-1.0, which the allowlist does not allow with GPL-3.0-only"},
	}, got)

	_, err = JudgeModules([]Module{{Path: "example.com/gone", Version: "v1.0.0", Dir: filepath.Join(t.TempDir(), "gone")}}, a)
	require.Error(t, err)
	assert.ErrorContains(t, err, "example.com/gone@v1.0.0")
}

// TestModulesOf reads the module of each listed package, the main
// module and the standard library left out, and refuses a package that
// does not load and a module path or version not of its form.
func TestModulesOf(t *testing.T) {
	seen := map[string]bool{}
	require.NoError(t, modulesOf(strings.NewReader(`{"ImportPath": "fmt"}
{"ImportPath": "example.com/m/x", "Module": {"Path": "example.com/m", "Main": true}}
{"ImportPath": "a.example/x/y", "Module": {"Path": "a.example/x", "Version": "v1.0.0"}}
{"ImportPath": "a.example/x/z", "Module": {"Path": "a.example/x", "Version": "v1.0.0"}}`), seen))
	assert.Equal(t, map[string]bool{"a.example/x@v1.0.0": true}, seen)
	for src, want := range map[string]string{
		`{"ImportPath": "a.example/x", "Error": {"Err": "no Go files"}}`:             "a.example/x: no Go files",
		`{"ImportPath": "p", "Module": {"Path": "-modfile=x", "Version": "v1.0.0"}}`: "is not of the form",
		`{"ImportPath": "p", "Module": {"Path": "a.example/x", "Version": "1.0.0"}}`: "is not of the form",
		`{`: "unexpected EOF",
	} {
		err := modulesOf(strings.NewReader(src), map[string]bool{})
		require.Error(t, err, src)
		assert.ErrorContains(t, err, want)
	}
}

// TestAModuleNoBuildCompilesIsNotJudged shows that a module go.sum pins
// but no build of the module compiles, such as one a dependency's tests
// alone import, is not judged: the fixture module's go.sum names one,
// and the listing of its builds has none. On this repository,
// github.com/dlclark/regexp2, which only the tests of jsonschema import,
// is left out, and jsonschema is in.
func TestAModuleNoBuildCompilesIsNotJudged(t *testing.T) {
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	env := check.StepEnv(filepath.Dir(goBin))
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.27\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), []byte("example.com/a v1.0.0 h1:aa=\nexample.com/a v1.0.0/go.mod h1:bb=\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600))
	got, err := CompiledModules(t.Context(), goBin, root, env)
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = CompiledModules(t.Context(), goBin, citest.ModuleRoot(t), env)
	require.NoError(t, err)
	assert.Contains(t, got, "github.com/santhosh-tekuri/jsonschema/v6@v6.0.3")
	assert.NotContains(t, got, "github.com/dlclark/regexp2@v1.11.0")

	require.NoError(t, os.WriteFile(filepath.Join(root, "bad.go"), []byte("package main\n\nimport _ \"example.com/a\"\n"), 0o600))
	_, err = CompiledModules(t.Context(), goBin, root, env)
	require.Error(t, err, "a package that does not load")
}

// TestDecodeDownload reads the output of "go mod download -json", and
// fails a module it reports an error for or gives no directory.
func TestDecodeDownload(t *testing.T) {
	got, err := decodeDownload(strings.NewReader(`{"Path": "a.example/x", "Version": "v1.0.0", "Dir": "/cache/x"}` + "\n"))
	require.NoError(t, err)
	assert.Equal(t, []Module{{Path: "a.example/x", Version: "v1.0.0", Dir: "/cache/x"}}, got)
	for src, want := range map[string]string{
		`{"Path": "a.example/x", "Version": "v1.0.0", "Error": "not in the cache"}`: "a.example/x@v1.0.0: not in the cache",
		`{"Path": "a.example/x", "Version": "v1.0.0"}`:                              "a.example/x@v1.0.0: no directory in the module cache",
		`{`: "go mod download: jsontext: unexpected EOF",
	} {
		_, err := decodeDownload(strings.NewReader(src))
		require.Error(t, err, src)
		assert.ErrorContains(t, err, want)
	}
}

// TestTheModuleGraphIsAllowed runs the step's logic on this
// repository: every module a build of it compiles carries a license
// of the allowlist. It reads the module cache, as the step does.
func TestTheModuleGraphIsAllowed(t *testing.T) {
	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	var out bytes.Buffer
	ok, err := reportModules(t.Context(), &out, goBin, citest.ModuleRoot(t), check.StepEnv(filepath.Dir(goBin)))
	require.NoError(t, err)
	assert.True(t, ok, out.String())
	assert.Equal(t, "licenses: ok\n", out.String())

	empty := t.TempDir()
	_, err = reportModules(t.Context(), &out, goBin, empty, check.StepEnv(filepath.Dir(goBin)))
	require.Error(t, err, "a tree without the allowlist")
	writeAllowlist(t, empty)
	_, err = reportModules(t.Context(), &out, goBin, empty, check.StepEnv(filepath.Dir(goBin)))
	require.Error(t, err, "a tree without go.mod")
	require.NoError(t, os.WriteFile(filepath.Join(empty, "go.mod"), []byte("module example.com/m\n"), 0o600))
	ok, err = reportModules(t.Context(), &out, goBin, empty, check.StepEnv(filepath.Dir(goBin)))
	require.NoError(t, err)
	assert.True(t, ok, "a tree without packages has no module to read")

	_, err = Download(t.Context(), goBin, t.TempDir(), check.StepEnv(filepath.Dir(goBin)), []string{"example.invalid/none@v1.0.0"})
	require.Error(t, err, "a module the cache lacks, with GOPROXY=off")
}

// TestModulesStep pins the second step of the check: the licenses
// subcommand, with the pinned go, in the environment of the steps, on
// every platform.
func TestModulesStep(t *testing.T) {
	p := check.Plan{Root: t.TempDir(), Tools: check.Tools{GoDir: "/pinned/go/bin"}}
	want := check.Step{Name: "license modules", Argv: []string{"/pinned/go/bin/go", "run", "./tools/ci", "licenses"}, Environ: check.StepEnv("/pinned/go/bin")}
	assert.Equal(t, want, ModulesStep(p))
	steps := Steps(t.Context(), p)
	require.Len(t, steps, 2)
	assert.Equal(t, want, steps[1])
}

// TestRunModulesUsage refuses an argument.
func TestRunModulesUsage(t *testing.T) {
	r := citest.NewTree(t)
	r.Commit("fixture")
	code, out := citest.Run(t, r, nil, RunModules, "x")
	assert.Equal(t, check.ExitError, code, out)
	assert.Contains(t, out, "licenses takes no argument")
}

// committedAllowlist parses the allowlist of this repository.
func committedAllowlist(t *testing.T) Allowlist {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(AllowlistPath)))
	require.NoError(t, err)
	a, err := ParseAllowlist(src)
	require.NoError(t, err)
	return a
}

// TestEveryMarkerCounts gives each license of the allowlist a fixture of
// its markers, one per line: the fixture names the license, and the
// fixture without any one of its markers does not, so each marker is
// needed. An unless marker added to the fixture takes the license away.
func TestEveryMarkerCounts(t *testing.T) {
	a := committedAllowlist(t)
	for _, l := range a.Licenses {
		t.Run(l.ID, func(t *testing.T) {
			full := strings.Join(l.Markers, "\n") + "\n"
			assert.Contains(t, a.Identify([]byte(full)), l.ID, full)
			for i := range l.Markers {
				without := strings.Join(slices.Delete(slices.Clone(l.Markers), i, i+1), "\n") + "\n"
				assert.NotContains(t, a.Identify([]byte(without)), l.ID, "without %q", l.Markers[i])
			}
			for _, u := range l.Unless {
				assert.NotContains(t, a.Identify([]byte(full+u+"\n")), l.ID, "with %q", u)
			}
		})
	}
}

// TestRiders fails a file of an allowed license that holds a rider,
// one fixture per rider.
func TestRiders(t *testing.T) {
	a := committedAllowlist(t)
	for _, r := range a.Riders {
		t.Run(strings.ReplaceAll(r.Marker, " ", "_"), func(t *testing.T) {
			m := writeModule(t, "example.com/rider", map[string]string{"LICENSE": mitText + "With the " + r.Marker + " terms.\n"})
			got, err := JudgeModules([]Module{m}, a)
			require.NoError(t, err)
			assert.Equal(t, []check.Finding{{Where: "example.com/rider@v1.0.0", Rule: "license rider",
				Msg: "LICENSE holds the rider " + r.Marker + ", whose terms the allowlist does not read"}}, got)
		})
	}
}

// TestLicenseFileNames reads the files named as licenses, and leaves out
// the Go files, the names that only start like one, and testdata.
func TestLicenseFileNames(t *testing.T) {
	m := writeModule(t, "example.com/names", map[string]string{
		"LICENSE": mitText, "COPYING-MIT": mitText, "licence_x.md": mitText, "sub/License.txt": mitText,
		"license.go": "package x\n", "copying.go": "package x\n", "licensed.txt": ownText, "LICENSES": ownText,
		"testdata/LICENSE": ownText, "sub/testdata/COPYING": ownText,
	})
	files, err := licenseFiles(m.Dir)
	require.NoError(t, err)
	var names []string
	for _, f := range files {
		names = append(names, f.name)
	}
	assert.Equal(t, []string{"COPYING-MIT", "LICENSE", "licence_x.md", "sub/License.txt"}, names)
}

// TestGoModDirectives fails a replace or an exclude directive of go.mod
// before any module is read, one fixture each, in a block too.
func TestGoModDirectives(t *testing.T) {
	assert.Empty(t, goModFindings([]byte("module example.com/m\n\ngo 1.27\n\nrequire example.com/a v1.0.0\n")))
	assert.Equal(t, []check.Finding{
		{Where: "go.mod", Rule: "license go.mod", Msg: "line 3 holds a replace directive, so the step cannot read the license of the module the build uses"},
		{Where: "go.mod", Rule: "license go.mod", Msg: "line 4 holds a exclude directive, so the step cannot read the license of the module the build uses"},
		{Where: "go.mod", Rule: "license go.mod", Msg: "line 5 holds a replace directive, so the step cannot read the license of the module the build uses"},
	}, goModFindings([]byte("module example.com/m\n\nreplace example.com/a => ../a\nexclude example.com/b v1.0.0\nreplace (\n\texample.com/c => ../c\n)\n")))

	goBin, err := procgroup.LookPath("go")
	require.NoError(t, err)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\nreplace example.com/a => ../a\n"), 0o600))
	writeAllowlist(t, root)
	var out bytes.Buffer
	ok, err := reportModules(t.Context(), &out, goBin, root, check.StepEnv(filepath.Dir(goBin)))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "go.mod: license go.mod: line 3 holds a replace directive, so the step cannot read the license of the module the build uses\nlicenses: 1 finding(s)\n", out.String())
}

// writeAllowlist copies the committed allowlist into the tree at root.
func writeAllowlist(t *testing.T, root string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), filepath.FromSlash(AllowlistPath)))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(filepath.FromSlash(AllowlistPath))), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(AllowlistPath)), src, 0o600))
}

// TestVerifyFails shows a "go mod verify" that fails, as it does on a
// module whose extracted tree was edited in the cache, stopping the
// step with its output; a go command stands in for it.
func TestVerifyFails(t *testing.T) {
	goBin := filepath.Join(t.TempDir(), "go")
	require.NoError(t, os.WriteFile(goBin, []byte("#!/bin/sh\necho 'example.com/a v1.0.0: dir has been modified'\nexit 1\n"), 0o700))
	err := Verify(t.Context(), goBin, t.TempDir(), nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "go mod verify")
	require.ErrorContains(t, err, "dir has been modified")

	root := t.TempDir()
	writeAllowlist(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.sum"), []byte("example.com/a v1.0.0 h1:aa=\n"), 0o600))
	var out bytes.Buffer
	_, err = reportModules(t.Context(), &out, goBin, root, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "dir has been modified")
}

// TestTagSets pins the listings' tags: none, and those of
// taggedsuites.Suites (REQ-1254), spelled out here.
func TestTagSets(t *testing.T) {
	assert.Equal(t, [][]string{nil, {"-tags", "e2e"}}, tagSets)
}
