// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package handoff

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"syscall"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/memstore"
)

// Dir is a memory dir the reader reads: its path, and the label of the
// repo it belongs to, which every item read from it carries (REQ-0666,
// REQ-0680).
type Dir struct {
	Label string
	Path  string
}

// Name is a handoff file name read without parsing the file: its ULID
// and its kind (REQ-0244).
type Name struct {
	ID   canon.ULID
	Kind Kind
}

// String returns the file name, <ULID>-<kind>.md.
func (n Name) String() string { return n.ID.String() + "-" + string(n.Kind) + ".md" }

// FileName returns the name of the handoff file with the id and kind.
func FileName(id canon.ULID, k Kind) string { return Name{ID: id, Kind: k}.String() }

// nameForm is the grammar of a handoff file name.
var nameForm = regexp.MustCompile(`^([0-9a-z]{26})-(clear|final|facts)\.md$`)

// ParseName reads a handoff file name, and reports false for a name of
// any other form (a ULID in uppercase among them).
func ParseName(s string) (Name, bool) {
	m := nameForm.FindStringSubmatch(s)
	if m == nil {
		return Name{}, false
	}
	u, err := canon.ParseULID(m[1])
	if err != nil {
		return Name{}, false
	}
	return Name{ID: u, Kind: Kind(m[2])}, true
}

// maxNames bounds what Names reads, ten times the cap of 08 8.6, so a
// handoff/ an agent filled stays bounded though a dir over the cap
// is read (REQ-0650).
const maxNames = 10 * memstore.MaxHandoffFiles

// maxTemps is how many temporary files handoff/ may hold beside its
// files, so a dir of a few leftovers still reads, as in internal/memstore.
const maxTemps = 1000

// Names lists the handoff files of the memory dir whose root is root,
// oldest first, from their names alone. A memory dir without handoff/
// has none. A temporary name of julieta is never read (REQ-0654);
// handoff/ that is no directory (a symlink among them), any other member
// of it that is not a file <ULID>-<kind>.md, and more than maxNames
// files are memstore.ErrLayout. A handoff/ over memstore.MaxHandoffFiles
// still reads: it warns in sync and run and never refuses (REQ-0650).
func Names(root *os.Root) ([]Name, error) {
	info, err := root.Lstat(memstore.HandoffDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", memstore.ErrLayout, memstore.HandoffDir)
	}
	f, err := root.Open(memstore.HandoffDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []Name
	temps, seen := 0, 0
	for {
		batch, err := f.Readdirnames(256)
		for _, s := range batch {
			if seen++; seen > maxNames+maxTemps {
				return nil, fmt.Errorf("%w: %s holds more than %d names", memstore.ErrLayout, memstore.HandoffDir, maxNames+maxTemps)
			}
			if memstore.IsTemp(s) {
				if temps++; temps > maxTemps {
					return nil, fmt.Errorf("%w: %s holds more than %d temporary files", memstore.ErrLayout, memstore.HandoffDir, maxTemps)
				}
				continue
			}
			n, ok := ParseName(s)
			if !ok {
				return nil, fmt.Errorf("%w: %s/%s is not a file <ULID>-<kind>.md", memstore.ErrLayout, memstore.HandoffDir, clip(s))
			}
			out = append(out, n)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if len(out) > maxNames {
		return nil, fmt.Errorf("%w: %s holds more than %d files", memstore.ErrLayout, memstore.HandoffDir, maxNames)
	}
	slices.SortFunc(out, func(a, b Name) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return out, nil
}

// Path returns the slash path of the handoff file n from the memory dir.
func Path(n Name) string { return memstore.HandoffDir + "/" + n.String() }

// Read returns the handoff file n of the memory dir whose root is
// root: a regular file, judged with Lstat and again on the open file,
// opened without following a symlink and without waiting on a FIFO, of
// at most memstore.MaxFileSize bytes, that decodes strictly (Decode),
// whose kind is the one in its name. The ULID of the name is not in the
// file; it is the id of the handoff.
func Read(root *os.Root, n Name) (File, error) {
	p := Path(n)
	info, err := root.Lstat(p)
	if err != nil {
		return File{}, err
	}
	if !info.Mode().IsRegular() {
		return File{}, fmt.Errorf("%w: %s is not a regular file", memstore.ErrLayout, p)
	}
	f, err := root.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return File{}, err
	}
	defer func() { _ = f.Close() }()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return File{}, fmt.Errorf("%w: %s is not a regular file", memstore.ErrLayout, p)
	}
	data, err := io.ReadAll(io.LimitReader(f, memstore.MaxFileSize+1))
	if err != nil {
		return File{}, err
	}
	h, err := Decode(data)
	if err != nil {
		return File{}, fmt.Errorf("%s: %w", p, err)
	}
	if h.Kind != n.Kind {
		return File{}, fmt.Errorf("%s: %w: the kind %s is not the one in the file's name", p, ErrInvalid, h.Kind)
	}
	return h, nil
}

// Located is a handoff file with the label of the repo whose memory
// dir it was read from, and its id.
type Located struct {
	Repo string
	ID   canon.ULID
	File File
}

// Latest is what the handoff reader answers (REQ-0680): the latest
// narrative and the newest facts over a set of memory dirs, either nil
// when no dir holds one.
type Latest struct {
	// Narrative is the greatest ULID among the clear and final files.
	Narrative *Located
	// Facts is the greatest ULID among the facts files. A facts file
	// never hides a narrative (REQ-0680, REQ-0685): the two are chosen
	// apart.
	Facts *Located
}

// ReadLatest is the handoff reader of 08 8.3 (REQ-0680), the one
// function julieta and romeu use: "latest narrative" is the greatest
// ULID among the clear and final files over every dir, labelled with the
// dir it was read from, and "newest facts" the greatest ULID among the
// facts files. Files are chosen by name, and only the two chosen are
// decoded, so a bad file that is not chosen is not read. A dir that is
// not a memory dir (memstore.ErrUnmounted) or whose handoff/ breaks the
// layout is an error naming it. Of two files with one ULID the first
// dir wins.
func ReadLatest(dirs []Dir) (Latest, error) {
	var narr, facts *pick
	for _, d := range dirs {
		r, err := memstore.Open(d.Path)
		if err != nil {
			return Latest{}, fmt.Errorf("the memory dir of %s: %w", clip(d.Label), err)
		}
		names, err := Names(r.Root())
		if err != nil {
			_ = r.Close()
			return Latest{}, fmt.Errorf("the memory dir of %s: %w", clip(d.Label), err)
		}
		for _, n := range names {
			slot := &narr
			if n.Kind == Facts {
				slot = &facts
			}
			if *slot == nil || bytes.Compare(n.ID[:], (*slot).name.ID[:]) > 0 {
				*slot = &pick{dir: d, name: n}
			}
		}
		_ = r.Close()
	}
	var out Latest
	var err error
	if out.Narrative, err = load(narr); err != nil {
		return Latest{}, err
	}
	if out.Facts, err = load(facts); err != nil {
		return Latest{}, err
	}
	return out, nil
}

// pick is the file chosen so far from a dir.
type pick struct {
	dir  Dir
	name Name
}

// load reads the chosen file, or returns nil for none.
func load(p *pick) (*Located, error) {
	if p == nil {
		return nil, nil
	}
	r, err := memstore.Open(p.dir.Path)
	if err != nil {
		return nil, fmt.Errorf("the memory dir of %s: %w", clip(p.dir.Label), err)
	}
	defer func() { _ = r.Close() }()
	f, err := Read(r.Root(), p.name)
	if err != nil {
		return nil, fmt.Errorf("the memory dir of %s: %w", clip(p.dir.Label), err)
	}
	return &Located{Repo: p.dir.Label, ID: p.name.ID, File: f}, nil
}
