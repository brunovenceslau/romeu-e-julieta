// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package pins reads and writes kits/pins.yaml, the one pin file (02
// 2.1): the kit frontend, the workload, herdr and mise. tools/kitpin
// writes it; the harness, the container e2e, layout and the kits read
// it, so a probe and a kit cannot disagree on a pin.
//
// The file is YAML, and a key it does not know is an error. An entry
// not pinned yet is absent:
//
//	frontend:
//	  ref: docker/sandbox-kit:3.0.0-m.<N>@sha256:<64 hex>
//	workload:
//	  ref: <registry>/<repository>@sha256:<64 hex>
//	  home:
//	    path: <the workload user's home>
//	    source: sitting M2
//	herdr:
//	  version: <version>
//	  sha256:
//	    linux-amd64: <64 hex>
//	    linux-arm64: <64 hex>
//	mise:
//	  version: <version>
//	  sha256:
//	    linux-amd64: <64 hex>
//	    linux-arm64: <64 hex>
//
// Encode writes the file whole, with its header comment, so that what
// it says is always there.
package pins

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Path is the pin file's path from the top of the repository.
const Path = "kits/pins.yaml"

// Pins is the content of the pin file.
type Pins struct {
	Frontend *Frontend `yaml:"frontend,omitempty"`
	Workload *Workload `yaml:"workload,omitempty"`
	Herdr    *Release  `yaml:"herdr,omitempty"`
	Mise     *Release  `yaml:"mise,omitempty"`
}

// Frontend is the kit frontend pin (06 6.1).
type Frontend struct {
	Ref string `yaml:"ref"`
}

// Workload is the product's own workload pin (06 6.2), with the
// workload user's home, which secondary repositories are cloned under
// (03 3.6).
type Workload struct {
	Ref  string `yaml:"ref"`
	Home *Fact  `yaml:"home,omitempty"`
}

// Fact is a value read by hand or by a probe, with where it was read.
type Fact struct {
	Path   string `yaml:"path"`
	Source string `yaml:"source"`
}

// Release is a tool pinned by version, with the sha256 of its linux
// binary for each architecture (12 12.1).
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
	sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// FrontendRef is the form of the frontend pin (06 6.1).
	FrontendRef = regexp.MustCompile(`^docker/sandbox-kit:3\.0\.0-m\.(0|[1-9][0-9]*)@sha256:[0-9a-f]{64}$`)
	// Version is the form of a herdr or mise version: three numbers.
	Version = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	// registry is localhost or a host name with a dot, each with an
	// optional port, or a one-label host name with a port, the forms
	// in which a reference's first component names a registry; label
	// is one label of a host name.
	label    = `[a-z0-9](?:[a-z0-9-]*[a-z0-9])?`
	registry = `(?:(?:localhost|` + label + `(?:\.` + label + `)+)(?::[0-9]+)?|` + label + `:[0-9]+)`
	// component is a path component of the OCI distribution
	// specification.
	component = `[a-z0-9]+(?:(?:\.|_|__|-+)[a-z0-9]+)*`
	// WorkloadRef is the form of a workload reference (03 3.1):
	// <registry>/<repository>@sha256:<64 hex>.
	WorkloadRef = regexp.MustCompile(`^` + registry + `/` + component + `(?:/` + component + `)*@sha256:[0-9a-f]{64}$`)
)

// SHA256 reports whether s is a sha256 in lower-case hex.
func SHA256(s string) bool { return sha256Hex.MatchString(s) }

// Parse reads the pin file and checks the form of each entry.
func Parse(src []byte) (Pins, error) {
	var p Pins
	dec := yaml.NewDecoder(bytes.NewReader(src))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		if errors.Is(err, io.EOF) {
			return Pins{}, nil
		}
		return Pins{}, fmt.Errorf("%s: %w", Path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Pins{}, fmt.Errorf("%s: more than one YAML document", Path)
	}
	if err := p.Check(); err != nil {
		return Pins{}, fmt.Errorf("%s: %w", Path, err)
	}
	return p, nil
}

// Check returns an error naming each entry whose form is wrong.
func (p Pins) Check() error {
	var errs []error
	if p.Frontend != nil && !FrontendRef.MatchString(p.Frontend.Ref) {
		errs = append(errs, fmt.Errorf("frontend.ref %q is not docker/sandbox-kit:3.0.0-m.<N>@sha256:<64 hex>", p.Frontend.Ref))
	}
	if w := p.Workload; w != nil {
		if !WorkloadRef.MatchString(w.Ref) {
			errs = append(errs, fmt.Errorf("workload.ref %q is not <registry>/<repository>@sha256:<64 hex>", w.Ref))
		}
		if w.Home != nil {
			if !path.IsAbs(w.Home.Path) || path.Clean(w.Home.Path) != w.Home.Path {
				errs = append(errs, fmt.Errorf("workload.home.path %q is not a clean absolute path", w.Home.Path))
			}
			if strings.TrimSpace(w.Home.Source) == "" {
				errs = append(errs, errors.New("workload.home.source is empty"))
			}
		}
	}
	for _, r := range []struct {
		name string
		rel  *Release
	}{{"herdr", p.Herdr}, {"mise", p.Mise}} {
		if r.rel == nil {
			continue
		}
		if !Version.MatchString(r.rel.Version) {
			errs = append(errs, fmt.Errorf("%s.version %q is not three numbers", r.name, r.rel.Version))
		}
		for _, a := range [][2]string{{"linux-amd64", r.rel.SHA256.Amd64}, {"linux-arm64", r.rel.SHA256.Arm64}} {
			if !SHA256(a[1]) {
				errs = append(errs, fmt.Errorf("%s.sha256.%s %q is not 64 lower-case hex digits", r.name, a[0], a[1]))
			}
		}
	}
	return errors.Join(errs...)
}

// header is the comment Encode writes first. Its last paragraph is the
// rule a check cannot make (12 12.1, REQ-1069).
const header = `# SPDX-FileCopyrightText: 2026 Bruno Venceslau
# SPDX-License-Identifier: GPL-3.0-only
#
# The one pin file (02 2.1): the kit frontend, the workload, herdr and
# mise. The probes, the container e2e, layout and the kits read it.
# Only tools/kitpin writes it ("go run ./tools/kitpin" with frontend,
# workload <reference>, herdr <version> or mise <version>), and it
# writes the file whole; an entry not pinned yet is absent. The mise
# hashes come from kits/mise/SHASUMS256.txt, whose signature tools/kitpin
# verified with kits/mise-minisign.pub.
#
# A pin that changes after block O3 needs the probe that read it run
# again: the probes that 11 11.4 lists for that pin.
`

// Encode returns the pin file for p, after Check.
func Encode(p Pins) ([]byte, error) {
	if err := p.Check(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(header)
	if p == (Pins{}) {
		return b.Bytes(), nil
	}
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
