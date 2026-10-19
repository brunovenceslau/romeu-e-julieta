// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package container

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// PinsPath is the pin file's path from the top of the repository (02
// 2.1). tools/kitpin writes it; its package tools/kitpin/pins reads it,
// but a package outside tools/ imports none of tools/ (the call-site
// scan of internal/procgroup), so the harness reads the three entries it
// needs itself, strictly, and checks the form of each.
const PinsPath = "kits/pins.yaml"

// Pins are the entries of the pin file the harness reads.
type Pins struct {
	Workload *WorkloadPin `yaml:"workload"`
	Herdr    *Release     `yaml:"herdr"`
	Mise     *Release     `yaml:"mise"`
}

// WorkloadPin is the workload pin: the image reference, by digest.
type WorkloadPin struct {
	Ref string `yaml:"ref"`
}

// Release is a tool pinned by version, with the sha256 of its linux
// binary for each architecture.
type Release struct {
	Version string `yaml:"version"`
	SHA256  Arches `yaml:"sha256"`
}

// Arches holds one sha256 per linux architecture.
type Arches struct {
	Amd64 string `yaml:"linux-amd64"`
	Arm64 string `yaml:"linux-arm64"`
}

var (
	sha256Form  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	versionForm = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	digestRef   = regexp.MustCompile(`^[a-z0-9.:-]+(/[a-z0-9._-]+)+@sha256:[0-9a-f]{64}$`)
)

// isSHA256 reports whether s is a sha256 in lower-case hex.
func isSHA256(s string) bool { return sha256Form.MatchString(s) }

// ParsePins reads the workload, herdr and mise entries of the pin file,
// leaving the others to their readers, and checks that each is there in
// its form: a reference by digest, and three numbers with a sha256 per
// architecture.
func ParsePins(src []byte) (Pins, error) {
	var p Pins
	if err := yaml.NewDecoder(bytes.NewReader(src)).Decode(&p); err != nil {
		return Pins{}, fmt.Errorf("%s: %w", PinsPath, err)
	}
	var errs []error
	if p.Workload == nil || !digestRef.MatchString(p.Workload.Ref) {
		errs = append(errs, errors.New("workload.ref is not <registry>/<repository>@sha256:<64 hex>"))
	}
	for name, r := range map[string]*Release{"herdr": p.Herdr, "mise": p.Mise} {
		if r == nil || !versionForm.MatchString(r.Version) || !isSHA256(r.SHA256.Amd64) || !isSHA256(r.SHA256.Arm64) {
			errs = append(errs, fmt.Errorf("%s is not a version with a sha256 per architecture", name))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return Pins{}, fmt.Errorf("%s, run go run ./tools/kitpin: %w", PinsPath, err)
	}
	return p, nil
}
