// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specFoo is a valid spec of the project foo, as J2 step 1 writes it.
const specFoo = `schema: project/v1
name: foo
repos:
  - url: https://github.com/example/foo-api
    primary: true
    ref: main
workload: docker.io/docker/sbx-kit-claude@sha256:c0132e89a89d94ddca3984513afdb36be57bd087fcc75823a4d14064b6a39ea5
agent: sbx-kit-claude
`

// TestJulietaSpecValidate runs julieta spec validate in the workload, in
// a config repo with no manifest (04 4.3, REQ-0422): a valid spec passes
// and the output names the rules romeu checks alone (REQ-0174); a spec
// whose name is not its file stem is a finding, and so is a file under
// projects/ that is not *.yaml, though the glob does not name it
// (REQ-0423), each exit 6, check-findings (RJ-328); a file that cannot
// be read is exit 2.
func TestJulietaSpecValidate(t *testing.T) {
	steps := scenario(t, `
mkdir -p /tmp/config/projects
cd /tmp/config
printf '%s' '`+specFoo+`' > projects/foo.yaml
step valid env -u JULIETA_MANIFEST "$J" spec validate projects/foo.yaml
sed 's/^name: foo$/name: bar/' projects/foo.yaml > projects/baz.yaml
step stem env -u JULIETA_MANIFEST "$J" spec validate projects/foo.yaml projects/baz.yaml
rm projects/baz.yaml
printf 'x' > projects/README.md
step stray env -u JULIETA_MANIFEST "$J" spec validate projects/*.yaml
step missing env -u JULIETA_MANIFEST "$J" spec validate projects/gone.yaml
`)
	r := step(t, steps, "valid")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "not checked, since they read host settings: config-project (REQ-0179), repo-url-host (REQ-0180), workload-repository (REQ-0190)\n"+
		"spec validate 1 files 0 findings\n", r.stdout)
	assert.Empty(t, r.stderr)

	r = step(t, steps, "stem")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Equal(t, "RJ-328", textErrorID(r))
	assert.Contains(t, r.stdout, "finding project-name-stem projects/baz.yaml: name: ")

	r = step(t, steps, "stray")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Equal(t, "RJ-328", textErrorID(r))
	assert.True(t, strings.HasPrefix(r.stdout, "finding projects-yaml-only projects/README.md: the file: "), r.stdout)
	assert.Contains(t, r.stdout, "spec validate 2 files 1 findings\n")

	r = step(t, steps, "missing")
	assert.Equal(t, 2, r.code, r.stderr)
	assert.Equal(t, "RJ-349", textErrorID(r))
	assert.Empty(t, r.stdout)
}

// textErrorID returns the id of the error report a command without
// --json prints first on stderr, "error <id> <slug>: <message>".
func textErrorID(r stepResult) string {
	f := strings.Fields(r.stderr)
	if len(f) < 2 || f[0] != "error" {
		return ""
	}
	return f[1]
}
