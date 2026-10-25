// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/oci"
	"github.com/brunovenceslau/romeu-e-julieta/internal/shquote"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/kits"
)

// GitSource is the seam personal kit materialization reads from git (I27).
// *gitsafe.Runner implements it directly.
type GitSource interface {
	Fsck(ctx context.Context, dir, commit string) error
	WalkTree(ctx context.Context, dir, commit string) ([]gitsafe.TreeEntry, error)
	CommitFile(ctx context.Context, dir, commit, name string, limit int) ([]byte, bool, error)
}

// KitMaterializerConfig holds the configuration for KitMaterializer.
type KitMaterializerConfig struct {
	// ProductFS is the filesystem holding product kits (06 6.4).
	// If nil, kits.FS is used.
	ProductFS fs.FS
	// Git is the GitSource reading the config repository.
	Git GitSource
	// ConfigDir is the path to the config repository directory.
	ConfigDir string
	// ConfigCommit is the object id of the config commit.
	ConfigCommit string
	// Root is $ROMEU_ROOT, used to resolve absolute paths.
	Root string
}

// kitMaterializer implements Materializer.
type kitMaterializer struct {
	cfg KitMaterializerConfig
}

// NewMaterializer returns a Materializer that writes product kits from cfg.ProductFS
// (defaulting to kits.FS) and personal kits from cfg.ConfigCommit via cfg.Git.
func NewMaterializer(cfg KitMaterializerConfig) Materializer {
	if cfg.ProductFS == nil {
		cfg.ProductFS = kits.FS
	}
	return &kitMaterializer{cfg: cfg}
}

// maxKitFileSize is the 1 MiB limit for reading a kit file from git (I27).
const maxKitFileSize = 1 << 20

// Materialize writes each kit of spec p into the candidate directory c.Dir (01 1.6,
// 06 6.1): product kits from the embed seam and personal kits from the config
// commit's tree through git fsck --strict and WalkTree (I27), and returns them
// in the spec's order.
//
//romeu:invariant I27 guard
//romeu:invariant I28 guard
func (m *kitMaterializer) Materialize(ctx context.Context, c Candidate, p spec.Project) ([]MaterializedKit, error) {
	if c.Dir == "" {
		return nil, fmt.Errorf("render: candidate directory is empty")
	}

	hasPersonal := false
	for _, k := range p.Kits {
		if k.Personal != "" {
			hasPersonal = true
			break
		}
	}

	var walkEntries []gitsafe.TreeEntry
	if hasPersonal {
		if m.cfg.Git == nil || m.cfg.ConfigDir == "" || m.cfg.ConfigCommit == "" {
			return nil, fmt.Errorf("render: personal kits require Git, ConfigDir and ConfigCommit")
		}
		if err := m.cfg.Git.Fsck(ctx, m.cfg.ConfigDir, m.cfg.ConfigCommit); err != nil {
			return nil, fmt.Errorf("render: git fsck: %w", err)
		}
		entries, err := m.cfg.Git.WalkTree(ctx, m.cfg.ConfigDir, m.cfg.ConfigCommit)
		if err != nil {
			return nil, fmt.Errorf("render: walk tree: %w", err)
		}
		walkEntries = entries
	}

	result := make([]MaterializedKit, 0, len(p.Kits))
	for i, k := range p.Kits {
		switch {
		case k.Product != "" && k.Personal == "":
			mat, err := m.materializeProductKit(ctx, c, p, k)
			if err != nil {
				return nil, fmt.Errorf("render: kit %d (%q): %w", i+1, k.Product, err)
			}
			result = append(result, mat)
		case k.Personal != "" && k.Product == "":
			mat, err := m.materializePersonalKit(ctx, c, p, k, walkEntries)
			if err != nil {
				return nil, fmt.Errorf("render: personal kit %d (%q): %w", i+1, k.Personal, err)
			}
			result = append(result, mat)
		default:
			return nil, fmt.Errorf("render: kit %d must specify exactly one of product or personal", i+1)
		}
	}

	return result, nil
}

// materializeProductKit reads a product kit from ProductFS, computes its kit-tree
// digest, writes its files through os.Root into the candidate, and checks args.
func (m *kitMaterializer) materializeProductKit(_ context.Context, c Candidate, p spec.Project, k spec.Kit) (MaterializedKit, error) {
	var files []gate.File
	kitDir := k.Product
	err := fs.WalkDir(m.cfg.ProductFS, kitDir, func(entryPath string, d fs.DirEntry, walkErr error) error {
		switch {
		case walkErr != nil:
			return walkErr
		case d.IsDir() && d.Name() == "testdata":
			return fs.SkipDir
		case d.IsDir():
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := fs.ReadFile(m.cfg.ProductFS, entryPath)
		if err != nil {
			return err
		}
		mode := gate.ModeRegular
		if info.Mode()&0111 != 0 {
			mode = gate.ModeExecutable
		}
		relPath := strings.TrimPrefix(entryPath, kitDir+"/")
		files = append(files, gate.File{
			Path: relPath,
			Mode: mode,
			Data: data,
		})
		return nil
	})
	if err != nil {
		return MaterializedKit{}, err
	}
	if len(files) == 0 {
		return MaterializedKit{}, fmt.Errorf("no files found in kit %q", k.Product)
	}

	descData, err := findDescriptor(k.Product, files)
	if err != nil {
		return MaterializedKit{}, err
	}
	parsedKit, err := oci.ParseDescriptor(descData)
	if err != nil {
		return MaterializedKit{}, fmt.Errorf("descriptor: %w", err)
	}

	digest, err := gate.KitTreeDigest(files)
	if err != nil {
		return MaterializedKit{}, fmt.Errorf("kit tree digest: %w", err)
	}

	mat := MaterializedKit{
		ID:           k.Product,
		Digest:       digest,
		Capabilities: parsedKit.Capabilities,
	}

	if err := writeKitFiles(c, mat.Dir(), files); err != nil {
		return MaterializedKit{}, err
	}

	mat.Rendered, err = m.renderArgs(p, k, parsedKit.Capabilities)
	if err != nil {
		return MaterializedKit{}, err
	}

	return mat, nil
}

// materializePersonalKit reads a personal kit from the config commit's tree,
// parses and enforces refusal rules on its capabilities (REQ-0574), computes
// its kit-tree digest, and writes its files through os.Root into the candidate.
func (m *kitMaterializer) materializePersonalKit(ctx context.Context, c Candidate, p spec.Project, k spec.Kit, entries []gitsafe.TreeEntry) (MaterializedKit, error) {
	prefix := "kits/" + k.Personal + "/"
	var files []gate.File
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		relPath := strings.TrimPrefix(entry.Path, prefix)
		if relPath == "" {
			continue
		}
		data, found, err := m.cfg.Git.CommitFile(ctx, m.cfg.ConfigDir, m.cfg.ConfigCommit, entry.Path, maxKitFileSize)
		if err != nil {
			return MaterializedKit{}, fmt.Errorf("read file %q: %w", entry.Path, err)
		}
		if !found {
			return MaterializedKit{}, fmt.Errorf("file %q not found in commit", entry.Path)
		}
		files = append(files, gate.File{
			Path: relPath,
			Mode: entry.Mode,
			Data: data,
		})
	}
	if len(files) == 0 {
		return MaterializedKit{}, fmt.Errorf("no files found under %s in config commit", prefix)
	}

	descData, err := findDescriptor(k.Personal, files)
	if err != nil {
		return MaterializedKit{}, err
	}
	parsedKit, err := oci.ParseDescriptor(descData)
	if err != nil {
		return MaterializedKit{}, fmt.Errorf("descriptor: %w", err)
	}

	// Refusal rules for personal kits (REQ-0574, I28): network, mount, volume,
	// credential and ssh-agent are refused.
	if parsedKit.Capabilities.Network != nil {
		return MaterializedKit{}, fmt.Errorf("personal kit %q declares network capability, which is refused (REQ-0574)", k.Personal)
	}
	if len(parsedKit.Capabilities.Volumes) > 0 {
		return MaterializedKit{}, fmt.Errorf("personal kit %q declares volume/mount capability, which is refused (REQ-0574)", k.Personal)
	}
	if len(parsedKit.Capabilities.Credentials) > 0 {
		return MaterializedKit{}, fmt.Errorf("personal kit %q declares credential capability, which is refused (REQ-0574)", k.Personal)
	}
	if len(parsedKit.Capabilities.SSHAgent) > 0 {
		return MaterializedKit{}, fmt.Errorf("personal kit %q declares ssh-agent capability, which is refused (REQ-0574)", k.Personal)
	}

	digest, err := gate.KitTreeDigest(files)
	if err != nil {
		return MaterializedKit{}, fmt.Errorf("kit tree digest: %w", err)
	}

	mat := MaterializedKit{
		ID:           k.Personal,
		Digest:       digest,
		Capabilities: parsedKit.Capabilities,
	}

	if err := writeKitFiles(c, mat.Dir(), files); err != nil {
		return MaterializedKit{}, err
	}

	mat.Rendered, err = m.renderArgs(p, k, parsedKit.Capabilities)
	if err != nil {
		return MaterializedKit{}, err
	}

	return mat, nil
}

// findDescriptor finds the descriptor file data in files, preferring <id>.yaml.
func findDescriptor(id string, files []gate.File) ([]byte, error) {
	for _, f := range files {
		if f.Path == id+".yaml" {
			return f.Data, nil
		}
	}
	for _, f := range files {
		if !strings.Contains(f.Path, "/") && (strings.HasSuffix(f.Path, ".yaml") || strings.HasSuffix(f.Path, ".yml")) {
			return f.Data, nil
		}
	}
	return nil, fmt.Errorf("no descriptor (%s.yaml) found", id)
}

// renderArgs renders reserved kit args like julietaBin (06 6.4).
func (m *kitMaterializer) renderArgs(p spec.Project, k spec.Kit, caps oci.Capabilities) (map[string]string, error) {
	var rendered map[string]string
	if caps.Args != nil {
		if _, ok := caps.Args["julietaBin"]; ok {
			binPath := path.Join(m.cfg.Root, p.Name+"-env", BinDir)
			quoted, err := shquote.Quote(binPath)
			if err != nil {
				return nil, fmt.Errorf("quote julietaBin: %w", err)
			}
			rendered = map[string]string{"julietaBin": quoted}
		}
	}
	return rendered, nil
}

// writeKitFiles writes files into candidate c's CandidateKitsDir/kitDir using an os.Root.
func writeKitFiles(c Candidate, kitDir string, files []gate.File) error {
	r, err := os.OpenRoot(c.Dir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = r.Close() }()

	targetDir := path.Join(CandidateKitsDir, kitDir)
	if err := r.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("render: mkdir %s: %w", targetDir, err)
	}
	for _, f := range files {
		filePath := path.Join(targetDir, f.Path)
		fileDir := path.Dir(filePath)
		if fileDir != targetDir && fileDir != "." {
			if err := r.MkdirAll(fileDir, 0o700); err != nil {
				return fmt.Errorf("render: mkdir %s: %w", fileDir, err)
			}
		}
		mode := os.FileMode(FileMode)
		if f.Mode == gate.ModeExecutable {
			mode = 0o700
		}
		if err := r.WriteFile(filePath, f.Data, mode); err != nil {
			return fmt.Errorf("render: write %s: %w", filePath, err)
		}
	}
	return nil
}
