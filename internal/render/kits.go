// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/shquote"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// The modes of a kit's files and dirs in a candidate: nothing but romeu
// and sbx, as the user, reads them (REQ-0141). A kit file keeps only
// whether it is executable, the two git modes of the kit-tree digest.
const (
	kitFileMode = 0o600
	kitExecMode = 0o700
	kitDirMode  = 0o700
)

// JulietaBinArg is the reserved kit arg romeu renders (06 6.4,
// REQ-0596).
const JulietaBinArg = "julietaBin"

// goarchForm is the form of a Go architecture name.
var goarchForm = regexp.MustCompile(`^[a-z0-9]+$`)

// ErrPersonalKit is the error of a spec kit that names a personal kit,
// which this source does not materialize: personal kits come from the
// config commit's tree through gitsafe.WalkTree (I27), a source that
// joins ProductKits.Materialize beside the product one.
var ErrPersonalKit = errors.New("render: a personal kit is not materialized yet")

// ProductKits is the product-kit half of the Materializer seam (06
// 6.1, REQ-0568): it writes each product kit of the spec from FS, the
// one embed.FS of kits/embed.go in production (kits.FS, T077) and a
// fixture in a test, into the candidate's kits dir. A personal kit of
// the spec is ErrPersonalKit until the personal source lands.
type ProductKits struct {
	// FS holds one directory per product kit, each with its descriptor
	// <kit>.yaml.
	FS fs.FS
	// EnvDir is the absolute path of <name>-env/, the root of the
	// .romeu/bin julieta lives in.
	EnvDir string
	// GOARCH is romeu's own GOARCH, which names the binary
	// (REQ-0584).
	GOARCH string
}

// Materialize writes each kit of p into the candidate c, under the name
// MaterializedKit.Dir gives, and returns them in the spec's order. For
// each kit it computes the kit-tree digest (gate.KitTreeDigest, 03
// 3.12), parses the descriptor's capability set (oci.ParseDescriptor,
// REQ-0569), and, when the descriptor declares the arg julietaBin,
// renders it as the absolute path of the project's .romeu/bin julieta,
// quoted through internal/shquote (REQ-0584, REQ-0596). The files are
// written through an os.Root on the candidate.
//
// A file of FS is a regular file: a link or a device is refused. An
// embed.FS holds no executable bit, so a product kit file is executable
// only in a fixture FS that says so.
func (s ProductKits) Materialize(_ context.Context, c Candidate, p spec.Project) ([]MaterializedKit, error) {
	if !path.IsAbs(s.EnvDir) || path.Clean(s.EnvDir) != s.EnvDir {
		return nil, fmt.Errorf("render: the env dir %q is not an absolute clean path", s.EnvDir)
	}
	if !goarchForm.MatchString(s.GOARCH) {
		return nil, fmt.Errorf("render: %q is not a Go GOARCH name", s.GOARCH)
	}
	bin := path.Join(s.EnvDir, BinDir, "julieta-linux-"+s.GOARCH)
	quoted, err := shquote.Quote(bin)
	if err != nil {
		return nil, fmt.Errorf("render: the path of julieta: %w", err)
	}
	r, err := os.OpenRoot(c.Dir)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = r.Close() }()

	out := make([]MaterializedKit, 0, len(p.Kits))
	for _, k := range p.Kits {
		if k.Personal != "" {
			return nil, fmt.Errorf("%w: %q", ErrPersonalKit, k.Personal)
		}
		m, files, err := s.read(k.Product)
		if err != nil {
			return nil, err
		}
		if _, ok := m.Capabilities.Args[JulietaBinArg]; ok {
			m.Rendered = map[string]string{JulietaBinArg: quoted}
		}
		if err := writeKit(r, m.Dir(), files); err != nil {
			return nil, fmt.Errorf("render: the kit %q: %w", m.ID, err)
		}
		out = append(out, m)
	}
	return out, nil
}

// read returns the product kit id of FS, without its Rendered args, and
// its files.
func (s ProductKits) read(id string) (MaterializedKit, []gate.File, error) {
	if err := oneElement(id); err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q: %w", id, err)
	}
	sub, err := fs.Sub(s.FS, id)
	if err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q: %w", id, err)
	}
	files, err := kitFiles(sub)
	if err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q: %w", id, err)
	}
	i := slices.IndexFunc(files, func(f gate.File) bool { return f.Path == id+".yaml" })
	if i < 0 {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q has no descriptor %s.yaml", id, id)
	}
	kit, err := oci.ParseDescriptor(files[i].Data)
	if err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q: %s.yaml: %w", id, id, err)
	}
	digest, err := gate.KitTreeDigest(files)
	if err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q: %w", id, err)
	}
	return MaterializedKit{ID: id, Digest: digest, Capabilities: kit.Capabilities}, files, nil
}

// kitFiles reads every file below the root of fsys, in the walk's
// order, refusing what is not a regular file.
func kitFiles(fsys fs.FS) ([]gate.File, error) {
	var files []gate.File
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%q is not a regular file", p)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		mode := gate.ModeRegular
		if fi.Mode().Perm()&0o111 != 0 {
			mode = gate.ModeExecutable
		}
		files = append(files, gate.File{Path: p, Mode: mode, Data: data})
		return nil
	})
	return files, err
}

// writeKit writes files below kits/<dir> of r, each created new, so a
// kit written twice, or a name already there, fails.
func writeKit(r *os.Root, dir string, files []gate.File) error {
	for _, f := range files {
		name := path.Join(CandidateKitsDir, dir, f.Path)
		if err := r.MkdirAll(path.Dir(name), kitDirMode); err != nil {
			return err
		}
		mode := os.FileMode(kitFileMode)
		if f.Mode == gate.ModeExecutable {
			mode = kitExecMode
		}
		h, err := r.OpenFile(filepath.FromSlash(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		_, werr := h.Write(f.Data)
		if err := errors.Join(werr, h.Chmod(mode), h.Close()); err != nil {
			return err
		}
	}
	return nil
}
