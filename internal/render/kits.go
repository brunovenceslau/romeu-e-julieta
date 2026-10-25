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
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
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

// goarchForm is the form of a Go architecture name. PR #285 (bin.go)
// has the same regexp as archName; the second PR to land reuses the
// first's.
var goarchForm = regexp.MustCompile(`^[a-z0-9]+$`)

// ConfigKitsDir is the dir of the config repo's tree that holds the
// personal kits, one dir per kit, each with its descriptor <kit>.yaml
// (02 2.2).
const ConfigKitsDir = "kits"

// ErrPersonalCapability is the error of a personal kit that declares a
// network, mount, volume, credential or ssh-agent capability, which a
// personal kit may not (06 6.1, REQ-0574; I28, REQ-0514): a sync error.
var ErrPersonalCapability = errors.New("render: a capability a personal kit may not declare")

// KitMaterializer is the Materializer of kits/ (06 6.1, REQ-0568): it
// writes each kit of the spec into the candidate's kits dir, a product
// kit from FS and a personal kit from the config commit's tree.
type KitMaterializer struct {
	// FS holds one directory per product kit, each with its descriptor
	// <kit>.yaml: the one embed.FS of kits/embed.go in production
	// (kits.FS, T077) and a fixture in a test.
	FS fs.FS
	// EnvDir is the absolute path of <name>-env/, the root of the
	// .romeu/bin julieta lives in.
	EnvDir string
	// GOARCH is romeu's own GOARCH, which names the binary
	// (REQ-0584).
	GOARCH string
	// Git reads the config repo, hardened for the host. It, ConfigDir
	// and ConfigCommit are needed only when the spec names a personal
	// kit.
	Git *gitsafe.Runner
	// ConfigDir is the absolute path of the root of the config repo's
	// clone.
	ConfigDir string
	// ConfigCommit is the full object id of the config commit the spec
	// was read at (Record.SpecSource.Commit).
	ConfigCommit string
}

// Materialize writes each kit of p into the candidate c, under the name
// MaterializedKit.Dir gives, and returns them in the spec's order. For
// each kit it computes the kit-tree digest (gate.KitTreeDigest, 03
// 3.12), parses the descriptor's capability set (oci.ParseDescriptor,
// REQ-0569), and, when the descriptor declares the arg julietaBin,
// renders it as the absolute path of the project's .romeu/bin julieta,
// quoted through internal/shquote (REQ-0584, REQ-0596). Every kit is
// read and checked before any file is written, and the files are
// written through an os.Root on the candidate.
//
// A file of FS is a regular file: a link or a device is refused. An
// embed.FS reports no executable bit, so a product kit file is always
// written and digested as regular; a kit that needs an executable file
// needs another carrier. Only a fixture FS can say a file is executable.
//
// A personal kit is the dir kits/<kit>/ of the config commit's tree.
// When the spec names one, the commit gets two independent checks
// first (05 5.1, REQ-0465; I27): git fsck --strict, then the validated
// walk of the whole tree (gitsafe.WalkTree); a tree that either refuses
// fails the sync before anything is written. The bytes of the kit's
// files are read with gitsafe.ReadDir, one dir at a time, which holds
// a file to 1 MiB and a dir to 64 MiB, and must list exactly the files
// the walk listed. A personal kit that declares a capability of
// refusedCapability is ErrPersonalCapability; any other capability is
// kept in its set, for the gate.
//
// A failure while writing leaves the files already written in the
// candidate. The caller discards the candidate: internal/state deletes
// a leftover one at the next sync.
//
//romeu:invariant I27 guard
func (s KitMaterializer) Materialize(ctx context.Context, c Candidate, p spec.Project) ([]MaterializedKit, error) {
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
	var tree []gitsafe.TreeEntry
	if slices.ContainsFunc(p.Kits, func(k spec.Kit) bool { return k.Personal != "" }) {
		if tree, err = s.configTree(ctx); err != nil {
			return nil, err
		}
	}

	out := make([]MaterializedKit, 0, len(p.Kits))
	files := make([][]gate.File, 0, len(p.Kits))
	for _, k := range p.Kits {
		var m MaterializedKit
		var f []gate.File
		if k.Personal != "" {
			m, f, err = s.personal(ctx, tree, k.Personal)
		} else {
			m, f, err = s.product(k.Product)
		}
		if err != nil {
			return nil, err
		}
		if a, ok := m.Capabilities.Args[JulietaBinArg]; ok {
			if err := checkArg(m.ID, JulietaBinArg, quoted, a); err != nil {
				return nil, err
			}
			m.Rendered = map[string]string{JulietaBinArg: quoted}
		}
		out = append(out, m)
		files = append(files, f)
	}

	r, err := os.OpenRoot(c.Dir)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = r.Close() }()
	for i, m := range out {
		if err := writeKit(r, m.Dir(), files[i]); err != nil {
			return nil, fmt.Errorf("render: the kit %q: %w", m.ID, err)
		}
	}
	return out, nil
}

// checkArg holds the value rendered into the arg name of kit to the
// arg's declaration, so a value the install would refuse fails the
// sync, naming the cause. oci keeps the pattern as the descriptor
// writes it, and the sandbox matches the whole value, so it is
// anchored here.
func checkArg(kit, name, value string, a oci.Arg) error {
	if a.Pattern != nil {
		re, err := regexp.Compile(`\A(?:` + *a.Pattern + `)\z`)
		if err != nil {
			return fmt.Errorf("render: the kit %q: the pattern of the arg %q: %w", kit, name, err)
		}
		if !re.MatchString(value) {
			return fmt.Errorf("render: the kit %q: the value %q of the arg %q does not match its pattern %s", kit, value, name, *a.Pattern)
		}
	}
	if len(a.Enum) > 0 && !slices.Contains(a.Enum, value) {
		return fmt.Errorf("render: the kit %q: the value %q of the arg %q is not one of %q", kit, value, name, a.Enum)
	}
	return nil
}

// product returns the product kit id of FS, without its Rendered args,
// and its files.
func (s KitMaterializer) product(id string) (MaterializedKit, []gate.File, error) {
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
	m, err := kitOf(id, files)
	if err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the product kit %q: %w", id, err)
	}
	return m, files, nil
}

// configTree checks the config commit with fsck and then the walk, and
// returns the files the walk listed.
func (s KitMaterializer) configTree(ctx context.Context) ([]gitsafe.TreeEntry, error) {
	if s.Git == nil || s.ConfigDir == "" || s.ConfigCommit == "" {
		return nil, errors.New("render: the spec names a personal kit and there is no config commit to read it from")
	}
	if err := s.Git.Fsck(ctx, s.ConfigDir, s.ConfigCommit); err != nil {
		return nil, fmt.Errorf("render: the config commit %s: %w", s.ConfigCommit, err)
	}
	tree, err := s.Git.WalkTree(ctx, s.ConfigDir, s.ConfigCommit)
	if err != nil {
		return nil, fmt.Errorf("render: the config commit %s: %w", s.ConfigCommit, err)
	}
	return tree, nil
}

// personal returns the personal kit id of the config commit, whose tree
// the walk listed, without its Rendered args, and its files.
func (s KitMaterializer) personal(ctx context.Context, tree []gitsafe.TreeEntry, id string) (MaterializedKit, []gate.File, error) {
	if err := oneElement(id); err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the personal kit %q: %w", id, err)
	}
	prefix := ConfigKitsDir + "/" + id + "/"
	modes := map[string]string{} // the walk's files of the kit, by path in the kit
	var dirs []string
	for _, e := range tree {
		rel, ok := strings.CutPrefix(e.Path, prefix)
		if !ok {
			continue
		}
		modes[rel] = e.Mode
		if d := path.Dir(e.Path); !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	if len(modes) == 0 {
		return MaterializedKit{}, nil, fmt.Errorf("render: the personal kit %q is not in the config commit (no %s)", id, prefix)
	}
	files := make([]gate.File, 0, len(modes))
	for _, d := range dirs {
		got, err := s.Git.ReadDir(ctx, s.ConfigDir, s.ConfigCommit, d)
		if err != nil {
			return MaterializedKit{}, nil, fmt.Errorf("render: the personal kit %q: %w", id, err)
		}
		for _, f := range got {
			rel := strings.TrimPrefix(d+"/"+f.Name, prefix)
			if mode, ok := modes[rel]; !ok || mode != f.Mode {
				return MaterializedKit{}, nil, fmt.Errorf("render: the personal kit %q: the read of %q is not the file the walk listed", id, rel)
			}
			delete(modes, rel)
			files = append(files, gate.File{Path: rel, Mode: f.Mode, Data: f.Data})
		}
	}
	for rel := range modes {
		return MaterializedKit{}, nil, fmt.Errorf("render: the personal kit %q: the walk listed %q and the read did not", id, rel)
	}
	m, err := kitOf(id, files)
	if err != nil {
		return MaterializedKit{}, nil, fmt.Errorf("render: the personal kit %q: %w", id, err)
	}
	if typ := refusedCapability(m.Capabilities); typ != "" {
		return MaterializedKit{}, nil, fmt.Errorf("%w: the personal kit %q declares a %s capability", ErrPersonalCapability, id, typ)
	}
	return m, files, nil
}

// refusedCapability returns the first capability type of c a personal
// kit may not declare, in the order of REQ-0574, or "" when c declares
// none: network, mount (a host mount), volume, credential and
// ssh-agent.
func refusedCapability(c oci.Capabilities) string {
	switch {
	case c.Network != nil:
		return "network"
	case slices.ContainsFunc(c.Volumes, func(v oci.Volume) bool { return v.Kind != "volume" }):
		return "mount"
	case len(c.Volumes) > 0:
		return "volume"
	case len(c.Credentials) > 0:
		return "credential"
	case len(c.SSHAgent) > 0:
		return "ssh-agent"
	}
	return ""
}

// kitOf returns the kit id of files, its digest and the capability set
// of its descriptor <id>.yaml.
func kitOf(id string, files []gate.File) (MaterializedKit, error) {
	i := slices.IndexFunc(files, func(f gate.File) bool { return f.Path == id+".yaml" })
	if i < 0 {
		return MaterializedKit{}, fmt.Errorf("no descriptor %s.yaml", id)
	}
	kit, err := oci.ParseDescriptor(files[i].Data)
	if err != nil {
		return MaterializedKit{}, fmt.Errorf("%s.yaml: %w", id, err)
	}
	digest, err := gate.KitTreeDigest(files)
	if err != nil {
		return MaterializedKit{}, err
	}
	return MaterializedKit{ID: id, Digest: digest, Capabilities: kit.Capabilities}, nil
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
		// Chmod follows OpenFile because the umask lowers the mode
		// OpenFile asked for.
		_, werr := h.Write(f.Data)
		if err := errors.Join(werr, h.Chmod(mode), h.Close()); err != nil {
			return err
		}
	}
	return nil
}
