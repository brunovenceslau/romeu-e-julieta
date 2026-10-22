// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package sbxdrv

import "github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"

var _ = registerBuilder("env-run-create", func(fx Fixture) []Call { return []Call{mustCall(EnvCreate(fx.Dir))} })
var _ = registerBuilder("env-run-start", func(fx Fixture) []Call { return []Call{mustCall(EnvStart(fx.Dir))} })

// EnvCreate is sbx env run -d --clone <dir>: the create of a sandbox
// (timeout class sbx-create, for image pulls and kit builds),
// interactive: stdio is the terminal's and raw, because sbx asks to
// approve the plan there (04 4.1, Output, REQ-0270). It never carries
// --auto-approve, nor the --force that replaced it in sbx v0.48.0
// (guard.go, I4).
//
// -d is --detach, "create/start the sandbox without attaching", and
// --clone overrides workspace.clone of sbxenv.yaml
// (https://docs.docker.com/reference/cli/sbx/env/run/).
func EnvCreate(dir string) (Call, error) {
	return (&argvBuilder{}).lit("env", "run", "-d", "--clone").slot(envDir, dir).call("env-run-create", procgroup.SbxCreate, Interactive)
}

// EnvStart is sbx env run -d <dir>: the start of an existing sandbox
// (timeout class sbx-start), interactive for the same reason. "If the
// sandbox already exists it is started and re-attached without
// re-provisioning" (https://docs.docker.com/reference/cli/sbx/env/run/).
func EnvStart(dir string) (Call, error) {
	return (&argvBuilder{}).lit("env", "run", "-d").slot(envDir, dir).call("env-run-start", procgroup.SbxStart, Interactive)
}
