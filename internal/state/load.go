// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package state

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// ErrInvalidDigest is the error of a descriptor digest that is not
// sha256:<64 lowercase hex>: it would be a path in descriptors/.
var ErrInvalidDigest = errors.New("state: not an OCI sha256 digest")

// ErrImmutable is the error of a descriptor entry written again with
// other content: entries are immutable (REQ-0240).
var ErrImmutable = errors.New("state: a descriptor entry is immutable")

// readRecord reads the file name below root, at most MaxRecordSize
// bytes (one more tells the reader it is over the cap). A file that is
// absent is an error that wraps fs.ErrNotExist.
func readRecord(root *os.Root, name string) ([]byte, error) {
	if root == nil {
		// A subdirectory OpenRead found absent holds no record.
		return nil, fmt.Errorf("state: %s: %w", name, fs.ErrNotExist)
	}
	// O_NONBLOCK keeps the open of a FIFO from waiting for a writer, and
	// the Stat refuses anything but a regular file: a record is a file
	// romeu wrote, and a device or a pipe in its place is damage.
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is %s, not a regular file", ErrInvalid, name, info.Mode().Type())
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxRecordSize+1))
	if err != nil {
		return nil, fmt.Errorf("state: %w", err)
	}
	return data, nil
}

// unreadable returns the error record-unreadable (RJ-334) for the file
// at path whose content failed with err, or err itself when it is the
// error format-newer or no decode error at all. The file is never
// written over: the details name it (REQ-0165).
func unreadable(path string, err error) error {
	if _, ok := errors.AsType[*cli.Error](err); ok || !errors.Is(err, ErrInvalid) {
		return err
	}
	return RecordUnreadable.Wrap(err, "the record does not decode", cli.Detail{Key: "file", Value: path})
}

// load reads and decodes the record name below root, whose path for a
// message is path.
func load[T any](root *os.Root, name, path string, decode func([]byte) (*T, error)) (*T, error) {
	data, err := readRecord(root, name)
	if err != nil {
		return nil, unreadable(path, err)
	}
	v, err := decode(data)
	if err != nil {
		return nil, unreadable(path, err)
	}
	return v, nil
}

// LoadToolchain reads toolchain.json. A record that is absent is an
// error that wraps fs.ErrNotExist; one that does not decode is
// record-unreadable (RJ-334).
func (s *Store) LoadToolchain() (*Toolchain, error) {
	return load(s.root, toolchainName, filepath.Join(s.dir, toolchainName), DecodeToolchain)
}

// UpdateToolchain is the one writer of toolchain.json: it reads the
// record (a new one when it is absent), calls fn, and writes the result
// atomically. A record that does not decode stops it before fn runs
// and before any write, so it is never written over (REQ-0165). The
// lock lk is held (REQ-0283).
func (s *Store) UpdateToolchain(lk *Lock, fn func(*Toolchain) error) error {
	if err := lk.check(s); err != nil {
		return err
	}
	t, err := s.LoadToolchain()
	switch {
	case errors.Is(err, os.ErrNotExist):
		t = &Toolchain{Schema: ToolchainField}
	case err != nil:
		return err
	}
	if err := fn(t); err != nil {
		return err
	}
	data, err := EncodeToolchain(t)
	if err != nil {
		return err
	}
	return writeFile(s.root, toolchainName, data)
}

// projectFile returns the file name of a project's record, after the
// name has passed the rule of 03 3.1.
func projectFile(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	return name + ".json", nil
}

// LoadProject reads projects/<name>.json. A record that is absent is an
// error that wraps fs.ErrNotExist; one that does not decode, or whose
// approval is another project's, is record-unreadable (RJ-334), with
// the project's name filled into the hint.
func (s *Store) LoadProject(name string) (*Project, error) {
	file, err := projectFile(name)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(s.dir, projectsDir, file)
	p, err := load(s.projects, file, path, DecodeProject)
	if err == nil {
		if err = p.checkProject(name); err != nil {
			p, err = nil, unreadable(path, fmt.Errorf("%w: %w", ErrInvalid, err))
		}
	}
	if e, ok := errors.AsType[*cli.Error](err); ok && e.ID == RecordUnreadable.ID {
		e.Fill = map[string]string{"<name>": name}
	}
	return p, err
}

// UpdateProject is the one writer of projects/<name>.json: it reads the
// record (a new one when it is absent), calls fn, and writes the result
// atomically. A record that does not decode stops it before fn runs and
// before any write, so it is never written over (REQ-0165); neither is
// a record fn leaves in a form no reader would take. The lock lk is
// held (REQ-0283).
func (s *Store) UpdateProject(lk *Lock, name string, fn func(*Project) error) error {
	if err := lk.check(s); err != nil {
		return err
	}
	file, err := projectFile(name)
	if err != nil {
		return err
	}
	p, err := s.LoadProject(name)
	switch {
	case errors.Is(err, os.ErrNotExist):
		p = NewProject()
	case err != nil:
		return err
	}
	if err := fn(p); err != nil {
		return err
	}
	if err := p.checkProject(name); err != nil {
		return fmt.Errorf("state: refused to write the record of %s, which would not read: %w", name, err)
	}
	data, err := EncodeProject(p)
	if err != nil {
		return err
	}
	return writeFile(s.projects, file, data)
}

// descriptorFile returns the file name of a descriptor, the hex of its
// OCI digest, after the digest has passed its form.
func descriptorFile(digest string) (string, error) {
	sub := ociDigest.FindStringSubmatch(digest)
	if sub == nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidDigest, clip(digest))
	}
	return sub[1] + ".json", nil
}

// GetDescriptor reads the descriptor cached under an OCI digest. An
// entry that is absent is an error that wraps fs.ErrNotExist (the
// descriptor cache is read first by sync, step 6, which then fetches);
// one that does not decode, or that holds another digest than its name
// says, is record-unreadable (RJ-334).
func (s *Store) GetDescriptor(digest string) (*Descriptor, error) {
	file, err := descriptorFile(digest)
	if err != nil {
		return nil, err
	}
	d, err := load(s.descriptors, file, filepath.Join(s.dir, descriptorsDir, file), DecodeDescriptor)
	if err != nil {
		return nil, err
	}
	if d.Digest != digest {
		return nil, unreadable(filepath.Join(s.dir, descriptorsDir, file),
			fmt.Errorf("%w: the entry holds the digest %s", ErrInvalid, clip(d.Digest)))
	}
	return d, nil
}

// FindDescriptor returns the cached entry reached from the workload for
// the platform. REQ-0341 gives the digest of the workload (the index of
// a multi-arch image), while an entry is keyed by the digest of the
// platform manifest, so the lookup reads every entry of descriptors/.
//
// By design an entry is skipped when it is not a regular file (a
// symlink, a FIFO), does not decode, or holds another digest than its
// name says: GetDescriptor of it is still record-unreadable (RJ-334),
// and one bad entry does not fail every sync. Any other fault is
// returned, not read as a miss: an I/O error, or an entry of a newer
// format (format-newer), as GetDescriptor gives them, so that an offline
// sync does not report registry-unreachable for a cache it could not
// read (REQ-0165).
//
// An index digest names one manifest per platform, so two entries of
// one pair with different digests are damage or a planted entry, and
// the lookup is record-unreadable rather than let a file name pick the
// capability set that gate 2 shows. A pair with no entry is an error
// that wraps fs.ErrNotExist, as GetDescriptor gives an absent digest. A
// local kit's entry has no workload and never matches, and neither does
// an empty workload or platform.
func (s *Store) FindDescriptor(workload, platform string) (*Descriptor, error) {
	if workload == "" || platform == "" {
		return nil, descriptorAbsent(workload, platform)
	}
	dir := filepath.Join(s.dir, descriptorsDir)
	entries, err := fs.ReadDir(s.descriptors.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("state: reading %s: %w", dir, err)
	}
	var found *Descriptor
	for _, e := range entries {
		hex, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := readRecord(s.descriptors, e.Name())
		if err != nil {
			if skippable(err) {
				continue
			}
			return nil, unreadable(path, err)
		}
		d, err := DecodeDescriptor(data)
		if err != nil {
			if skippable(err) {
				continue
			}
			return nil, unreadable(path, err)
		}
		if d.Digest != "sha256:"+hex || d.Workload != workload || d.Platform != platform {
			continue
		}
		if found != nil && found.Digest != d.Digest {
			return nil, unreadable(dir, fmt.Errorf("%w: two entries hold the workload %s on %s",
				ErrInvalid, clip(workload), clip(platform)))
		}
		found = d
	}
	if found == nil {
		return nil, descriptorAbsent(workload, platform)
	}
	return found, nil
}

// skippable reports whether err is the damage of one entry that
// FindDescriptor leaves to GetDescriptor: an invalid record, and not a
// format-newer error or an I/O fault.
func skippable(err error) bool {
	_, typed := errors.AsType[*cli.Error](err)
	return !typed && errors.Is(err, ErrInvalid)
}

// descriptorAbsent is the error of a pair with no cached entry.
func descriptorAbsent(workload, platform string) error {
	return fmt.Errorf("state: no descriptor for %s on %s: %w", clip(workload), clip(platform), fs.ErrNotExist)
}

// PutDescriptor caches a descriptor under its digest. Entries are
// immutable: the same content again writes nothing, other content is
// ErrImmutable, and an entry that does not decode is never written over
// (RJ-334). The schema field is set to this build's.
func (s *Store) PutDescriptor(lk *Lock, d Descriptor) error {
	if err := lk.check(s); err != nil {
		return err
	}
	d.Schema = DescriptorField
	file, err := descriptorFile(d.Digest)
	if err != nil {
		return err
	}
	data, err := EncodeDescriptor(&d)
	if err != nil {
		return err
	}
	have, err := readRecord(s.descriptors, file)
	if errors.Is(err, ErrInvalid) {
		return unreadable(filepath.Join(s.dir, descriptorsDir, file), err)
	}
	switch {
	case err == nil:
		old, derr := DecodeDescriptor(have)
		if derr != nil {
			return unreadable(filepath.Join(s.dir, descriptorsDir, file), derr)
		}
		// The values are compared, not the bytes, so a Go type that
		// spells the same capability another way is the same content.
		want, derr := DecodeDescriptor(data)
		if derr != nil {
			return derr
		}
		// The workload is where the manifest was first reached from; another
		// index that lists the same manifest is not other content.
		want.Workload = old.Workload
		if !reflect.DeepEqual(old, want) {
			return fmt.Errorf("%w: %s holds other content", ErrImmutable, strings.TrimSuffix(file, ".json"))
		}
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	return writeFile(s.descriptors, file, data)
}
