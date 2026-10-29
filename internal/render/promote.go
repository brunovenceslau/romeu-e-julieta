// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
)

var (
	// ErrPromotionInterrupted is the refusal of a promotion or a drift
	// check while render.json holds promoting: recovery runs first
	// (REQ-0087), and an interrupted promotion is never drift.
	ErrPromotionInterrupted = errors.New("render: a promotion was interrupted; recovery runs first")
	// ErrPromotionAbandoned is recovery's report of a promotion it could
	// not finish: the caller maps it to interrupted-promotion-abandoned,
	// exit 4 (REQ-0429).
	ErrPromotionAbandoned = errors.New("render: the interrupted promotion was abandoned")
	// ErrNotApproved is Promote's refusal of a candidate whose widening
	// digest is not the one approved.
	ErrNotApproved = errors.New("render: the candidate is not the one approved")
	// ErrKitDirDiffers is the refusal of a live kit dir whose content is
	// not its digest: a kit dir is verified, never rewritten.
	ErrKitDirDiffers = errors.New("render: a live kit dir differs from its digest")
)

// Recovery is what Recover did.
type Recovery int

const (
	// RecoveryNone means render.json held no promoting, so there was
	// nothing to recover.
	RecoveryNone Recovery = iota
	// RecoveryFinished means the promotion was finished by repeating
	// steps 2 to 4.
	RecoveryFinished
	// RecoveryAbandoned means the promotion was abandoned
	// (ErrPromotionAbandoned).
	RecoveryAbandoned
)

// WorkspaceFile is one workspace file step 2 writes whole (REQ-0048):
// derived from host state, in $ROMEU_ROOT rather than <name>-env/, so
// render.json holds no digest of it.
type WorkspaceFile struct {
	// Name is DevWorkspaceFile or ReviewWorkspaceFile.
	Name string
	// Data is its bytes.
	Data []byte
}

// Live is one <name>-env/ that a promotion writes, with what step 2
// writes that does not come from the candidate: the julieta binary and
// the workspace files.
type Live struct {
	// Root is the absolute path of $ROMEU_ROOT, which holds the
	// workspace files.
	Root string
	// EnvDir is the absolute path of <name>-env/.
	EnvDir string
	// Bin is the julieta binary and SHA256SUMS of .romeu/bin.
	Bin Bin
	// Workspace are the workspace files, written again whole.
	Workspace []WorkspaceFile

	// afterStep, set by the tests alone, runs after each step and stops
	// the promotion when it returns an error, the way a kill would.
	afterStep func(step int) error
}

// snapshot is a candidate dir read once into memory: what the
// promotion verifies is what it writes, whatever the dir holds later.
type snapshot struct {
	digest canon.Sum[canon.CandidateFilesKind]
	record Record
	env    []byte
	// kits are the files of each kit dir, by the dir's name.
	kits map[string][]gate.File
}

// Promote runs the promotion commit of 01 1.6 for the candidate c,
// whose widening digest must be approved: (1) render.json with
// promoting, (2) the kit dirs, .romeu/bin and the workspace files,
// (3) sbxenv.yaml renamed into place, the commit point, (4) render.json
// without promoting, the candidate dir and the unreferenced kit dirs
// deleted. What can fail on the candidate's content is checked before
// step 1. The caller holds the lock and has run Recover.
func (l Live) Promote(c Candidate, approved canon.Sum[canon.WideningKind]) error {
	root, err := os.OpenRoot(l.EnvDir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	id, err := canon.ParseULID(filepath.Base(c.Dir))
	if err != nil {
		return fmt.Errorf("render: the candidate dir is not named by a candidate id: %w", err)
	}
	if filepath.Clean(c.Dir) != filepath.Join(l.EnvDir, filepath.FromSlash(candidateDir(id))) {
		return fmt.Errorf("render: the candidate dir %s is not under %s", c.Dir, CandidatesDir)
	}
	live, found, err := readLiveRecord(root)
	if err != nil {
		return err
	}
	if found && live.Promoting != nil {
		return ErrPromotionInterrupted
	}
	snap, err := readSnapshot(root, candidateDir(id))
	if err != nil {
		return err
	}
	if snap.record.WideningDigest != approved {
		return ErrNotApproved
	}
	if err := l.preflight(root, snap); err != nil {
		return err
	}
	// Step 1 keeps the live record whole and adds the marker, so an
	// abandoned promotion leaves nothing of the candidate live. Before
	// the first promotion there is no live record: the candidate's
	// stands in with no files, and an abandon removes it again.
	marker := live
	if !found {
		marker = snap.record
		marker.Files = map[string]string{}
	}
	marker.Promoting = &Promoting{CandidateID: id, FilesDigest: snap.digest, PreviousFiles: maps.Clone(marker.Files)}
	if err := writeLiveRecord(root, marker); err != nil {
		return err
	}
	if err := l.step(1); err != nil {
		return err
	}
	return l.finish(root, snap, candidateDir(id))
}

// Recover finishes or abandons a promotion that render.json says was
// interrupted (REQ-0087). It runs after the lock in every mutating
// command. A candidate that still verifies against its filesDigest is
// finished by repeating steps 2 to 4, each idempotent, with the record
// the candidate holds, which the digest covers. Otherwise, or when the
// finish fails, the live record, which step 1
// kept whole, is written back without promoting and with the digests of
// what step 2 had written, and the result is ErrPromotionAbandoned.
// Before the commit point the old live sbxenv.yaml and its kit dirs are
// intact; after it, the live sbxenv.yaml is the approved candidate's,
// and the next drift check names it, since no trusted digest of it is
// left.
func (l Live) Recover() (Recovery, error) {
	root, err := os.OpenRoot(l.EnvDir)
	if err != nil {
		return RecoveryNone, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	rec, found, err := readLiveRecord(root)
	if err != nil || !found || rec.Promoting == nil {
		return RecoveryNone, err
	}
	cand := candidateDir(rec.Promoting.CandidateID)
	// A candidate that cannot be read, or holds what is not a regular
	// file, does not verify.
	snap, err := readSnapshot(root, cand)
	if err == nil && snap.digest == rec.Promoting.FilesDigest {
		ferr := l.preflight(root, snap)
		if ferr == nil {
			ferr = l.finish(root, snap, cand)
		}
		if ferr == nil {
			return RecoveryFinished, nil
		}
		// A finish that fails would fail again on every command: abandon
		// it, so the next sync renders afresh.
		if err := l.abandon(root, rec); err != nil {
			return RecoveryNone, err
		}
		return RecoveryAbandoned, fmt.Errorf("%w: %w", ErrPromotionAbandoned, ferr)
	}
	if err := l.abandon(root, rec); err != nil {
		return RecoveryNone, err
	}
	return RecoveryAbandoned, fmt.Errorf("%w: its candidate no longer verifies", ErrPromotionAbandoned)
}

// Interrupted reports whether render.json holds promoting. It takes no
// lock, so status can report an interrupted promotion (REQ-0087).
func Interrupted(envDir string) (bool, error) {
	root, err := os.OpenRoot(envDir)
	if err != nil {
		return false, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	rec, found, err := readLiveRecord(root)
	return found && rec.Promoting != nil, err
}

// preflight checks, before any write, what makes step 2 or 3 fail on
// the candidate's content: each kit path of its record in the form
// <kit>-<digest12> of its digest, each kit dir present in the
// candidate with that digest, a live kit dir of that name with it too,
// the bin digests of the julieta this romeu embeds, and the candidate's
// sbxenv.yaml with its recorded digest.
func (l Live) preflight(root *os.Root, snap snapshot) error {
	files := snap.record.Files
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if !isKitPath(p) {
			continue
		}
		name := path.Base(p)
		if path.Dir(p) != LiveKitsDir || strings.HasPrefix(name, ".tmp-") || !kitDirName.MatchString(name) ||
			!strings.HasPrefix(files[p], name[len(name)-12:]) {
			return fmt.Errorf("render: render.json names the kit dir %s, which is not <kit>-<digest12> of its digest", p)
		}
		kit, ok := snap.kits[name]
		if !ok {
			return fmt.Errorf("render: the candidate has no kit dir %s", name)
		}
		got, err := gate.KitTreeDigest(kit)
		if err != nil {
			return err
		}
		if got.String() != files[p] {
			return fmt.Errorf("render: the candidate's kit dir %s is not the one render.json names", name)
		}
		if ok, err := liveExists(root, p); err != nil {
			return err
		} else if ok {
			live, err := kitDirDigest(root, p)
			if err != nil {
				return err
			}
			if live != files[p] {
				return fmt.Errorf("%w: %s", ErrKitDirDiffers, p)
			}
		}
	}
	for p, d := range l.Bin.Files() {
		if files[p] != d {
			return fmt.Errorf("render: render.json holds another digest for %s than the julieta this romeu embeds", p)
		}
	}
	if snap.env == nil || plainSum(snap.env) != files[LiveEnvFile] {
		return fmt.Errorf("render: the candidate's sbxenv.yaml is not the one render.json names")
	}
	for _, w := range l.Workspace {
		if w.Name != DevWorkspaceFile && w.Name != ReviewWorkspaceFile {
			return fmt.Errorf("render: %q is not a workspace file", w.Name)
		}
	}
	return nil
}

// finish runs steps 2 to 4 for the candidate snap, whose dir is cand.
func (l Live) finish(root *os.Root, snap snapshot, cand string) error {
	next := snap.record
	for _, p := range slices.Sorted(maps.Keys(next.Files)) {
		if isKitPath(p) {
			if err := ensureKitDir(root, p, next.Files[p], snap.kits[path.Base(p)]); err != nil {
				return err
			}
		}
	}
	if err := l.Bin.WriteTo(root); err != nil {
		return err
	}
	if err := l.writeWorkspace(); err != nil {
		return err
	}
	if err := l.step(2); err != nil {
		return err
	}
	if err := commitEnv(root, next.Files[LiveEnvFile], snap.env); err != nil {
		return err
	}
	if err := l.step(3); err != nil {
		return err
	}
	next.Promoting = nil
	if err := writeLiveRecord(root, next); err != nil {
		return err
	}
	if err := removeCandidateDir(root, cand); err != nil {
		return err
	}
	return pruneKitDirs(root, next.Files)
}

// commitEnv is step 3, the commit point: env, whose digest is want,
// replaces the live sbxenv.yaml by rename. A live file that is already
// the new one is left as it is.
func commitEnv(root *os.Root, want string, env []byte) error {
	done, err := liveMatches(root, LiveEnvFile, want)
	if err != nil || done {
		return err
	}
	if err := replaceFile(root, LiveEnvFile, env, FileMode); err != nil {
		return err
	}
	return syncDir(root)
}

// abandon writes the live record back without promoting: the previous
// digests, plus the digests of what step 2 had written that the
// previous record does not name, each read from the live file and
// trusted without the candidate: the bin files whose content is the
// julieta this romeu embeds, and each kit dir whose kit-tree digest
// begins with the 12 hex digits of its own name. A dir that cannot be
// read is left out. An abandoned first promotion has no previous record
// to write back, so its render.json is removed. Then the candidate dir
// is deleted.
func (l Live) abandon(root *os.Root, rec Record) error {
	cand := candidateDir(rec.Promoting.CandidateID)
	previous := rec.Promoting.PreviousFiles
	if len(previous) == 0 {
		if err := removeFirstRecord(root); err != nil {
			return err
		}
		return removeCandidateDir(root, cand)
	}
	files := maps.Clone(previous)
	for p, d := range l.Bin.Files() {
		ok, err := liveMatches(root, p, d)
		if err != nil {
			return err
		}
		if ok {
			files[p] = d
		}
	}
	entries, err := fs.ReadDir(root.FS(), LiveKitsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("render: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		p := path.Join(LiveKitsDir, name)
		if _, known := previous[p]; known || !e.IsDir() || strings.HasPrefix(name, ".tmp-") || !kitDirName.MatchString(name) {
			continue
		}
		d, err := kitDirDigest(root, p)
		if err == nil && strings.HasPrefix(d, name[len(name)-12:]) {
			files[p] = d
		}
	}
	rec.Files = files
	rec.Promoting = nil
	if err := writeLiveRecord(root, rec); err != nil {
		return err
	}
	return removeCandidateDir(root, cand)
}

// ensureKitDir makes the live kit dir p hold files, whose kit-tree
// digest is want: a dir that exists is verified again here, since
// preflight's check may be stale by now, and never rewritten; a missing
// one is written into a temporary dir and renamed to its new name.
func ensureKitDir(root *os.Root, p, want string, files []gate.File) error {
	if ok, err := liveExists(root, p); err != nil {
		return err
	} else if ok {
		got, err := kitDirDigest(root, p)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%w: %s", ErrKitDirDiffers, p)
		}
		return nil
	}
	tmp := path.Join(LiveKitsDir, ".tmp-"+path.Base(p))
	if err := removeKitTemp(root, tmp); err != nil {
		return err
	}
	if err := copyKit(root, tmp, files); err != nil {
		return fmt.Errorf("render: copy the kit dir %s: %w", p, err)
	}
	if err := root.Rename(tmp, p); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	kits, err := root.OpenRoot(LiveKitsDir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = kits.Close() }()
	return syncDir(kits)
}

// copyKit writes files below dir of root, each created new with the
// kit modes and synced.
func copyKit(root *os.Root, dir string, files []gate.File) error {
	if err := root.MkdirAll(dir, kitDirMode); err != nil {
		return err
	}
	for _, f := range files {
		name := path.Join(dir, f.Path)
		if err := root.MkdirAll(path.Dir(name), kitDirMode); err != nil {
			return err
		}
		mode := os.FileMode(kitFileMode)
		if f.Mode == gate.ModeExecutable {
			mode = kitExecMode
		}
		h, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		// Chmod follows OpenFile because the umask lowers the mode
		// OpenFile asked for.
		_, werr := h.Write(f.Data)
		if err := errors.Join(werr, h.Chmod(mode), h.Sync(), h.Close()); err != nil {
			return err
		}
	}
	return nil
}

// removeKitTemp deletes the temporary kit dir tmp that a killed step 2
// may have left half written. It is an admitted call site of the I16
// scan (05 5.2).
func removeKitTemp(root *os.Root, tmp string) error {
	if err := root.RemoveAll(tmp); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// removeCandidateDir deletes the candidate dir cand, a path below
// CandidatesDir, once promoted or abandoned. It is an admitted call
// site of the I16 scan (05 5.2).
func removeCandidateDir(root *os.Root, cand string) error {
	if !strings.HasPrefix(cand, CandidatesDir+"/") {
		return fmt.Errorf("render: %s is not a candidate dir", cand)
	}
	if err := root.RemoveAll(cand); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// removeFirstRecord deletes the render.json an abandoned first
// promotion wrote at its step 1, where none was before, so nothing of
// the candidate is recorded as live. It is an admitted call site of the
// I16 scan (05 5.2).
func removeFirstRecord(root *os.Root) error {
	if err := root.Remove(LiveRecord); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

// kitDirName is the form of a kit dir's name, <kit>-<digest12>, and of
// the temporary dir step 2 copies it into, .tmp-<kit>-<digest12>.
var kitDirName = regexp.MustCompile(`^(?:\.tmp-)?[a-z0-9][a-z0-9-]*-[0-9a-f]{12}$`)

// pruneKitDirs deletes each dir under LiveKitsDir that has the form of
// a kit dir and that files does not name: the kit dirs the live
// sbxenv.yaml no longer references, and a temporary dir a killed step 2
// left. An entry of another name or kind is not romeu's to judge and
// stays. It runs after the commit point alone. It is an admitted call
// site of the I16 scan (05 5.2).
func pruneKitDirs(root *os.Root, files map[string]string) error {
	entries, err := fs.ReadDir(root.FS(), LiveKitsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	for _, e := range entries {
		p := path.Join(LiveKitsDir, e.Name())
		if _, kept := files[p]; kept || !e.IsDir() || !kitDirName.MatchString(e.Name()) {
			continue
		}
		if err := root.RemoveAll(p); err != nil {
			return fmt.Errorf("render: %w", err)
		}
	}
	return nil
}

// writeWorkspace replaces each workspace file in l.Root by the atomic
// write rule, through an os.Root on $ROMEU_ROOT.
func (l Live) writeWorkspace() error {
	if len(l.Workspace) == 0 {
		return nil
	}
	dir, err := os.OpenRoot(l.Root)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = dir.Close() }()
	for _, w := range l.Workspace {
		if w.Name != DevWorkspaceFile && w.Name != ReviewWorkspaceFile {
			return fmt.Errorf("render: %q is not a workspace file", w.Name)
		}
		if err := replaceFile(dir, w.Name, w.Data, FileMode); err != nil {
			return err
		}
	}
	return syncDir(dir)
}

// writeLiveRecord replaces render.json with rec.
func writeLiveRecord(root *os.Root, rec Record) error {
	data, err := EncodeRecord(rec)
	if err != nil {
		return err
	}
	dir := path.Dir(LiveRecord)
	if err := root.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	sub, err := root.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = sub.Close() }()
	if err := replaceFile(sub, path.Base(LiveRecord), data, FileMode); err != nil {
		return err
	}
	return syncDir(sub)
}

// readLiveRecord reads render.json; found is false when there is none,
// before the first promotion.
func readLiveRecord(root *os.Root) (rec Record, found bool, err error) {
	data, err := root.ReadFile(LiveRecord)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("render: %w", err)
	}
	rec, err = DecodeRecord(data)
	if err != nil {
		return Record{}, false, err
	}
	return rec, true, nil
}

// readSnapshot reads the candidate dir cand once: its candidate-files
// digest, and the record, the sbxenv.yaml and the kit files taken from
// the bytes the digest covers.
func readSnapshot(root *os.Root, cand string) (snapshot, error) {
	sub, err := root.OpenRoot(cand)
	if err != nil {
		return snapshot{}, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = sub.Close() }()
	files, err := candidateTree(sub.FS())
	if err != nil {
		return snapshot{}, err
	}
	digest, err := candidateFilesDigest(files)
	if err != nil {
		return snapshot{}, err
	}
	snap := snapshot{digest: digest, kits: map[string][]gate.File{}}
	for _, f := range files {
		switch f.Path {
		case CandidateRecord:
			snap.record, err = DecodeRecord(f.Data)
			if err != nil {
				return snapshot{}, err
			}
		case CandidateEnvFile:
			snap.env = f.Data
		default:
			if rest, ok := strings.CutPrefix(f.Path, CandidateKitsDir+"/"); ok {
				if name, inner, ok := strings.Cut(rest, "/"); ok {
					snap.kits[name] = append(snap.kits[name], gate.File{Path: inner, Mode: f.Mode, Data: f.Data})
				}
			}
		}
	}
	if snap.record.Schema == "" {
		return snapshot{}, fmt.Errorf("render: the candidate has no %s", CandidateRecord)
	}
	return snap, nil
}

// candidateTree reads every file below the root of fsys, refusing what
// is not a regular file or a dir.
func candidateTree(fsys fs.FS) ([]gate.File, error) {
	files, err := kitFiles(fsys)
	if err != nil {
		return nil, fmt.Errorf("render: the candidate's files: %w", err)
	}
	return files, nil
}

// candidateDir returns the path of the candidate id in <name>-env/.
func candidateDir(id canon.ULID) string { return path.Join(CandidatesDir, id.String()) }

// isKitPath reports whether the render.json path p is a kit dir.
func isKitPath(p string) bool { return strings.HasPrefix(p, LiveKitsDir+"/") }

// liveExists reports whether the live path p exists.
func liveExists(root *os.Root, p string) (bool, error) {
	_, err := root.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("render: %w", err)
	}
	return true, nil
}

// candidateFile is one entry of the candidate-files digest's input.
type candidateFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// candidateFiles is the input of the candidate-files digest (03 3.12),
// sorted by path.
type candidateFiles []candidateFile

// CanonKind declares the one kind of a candidate's files.
func (candidateFiles) CanonKind() canon.CandidateFilesKind { return canon.CandidateFiles }

// CandidateFilesDigest returns the candidate-files digest of the
// candidate dir dir (03 3.12): the sorted {path, sha256} of every file
// below it. Anything that is not a regular file or a dir is refused.
// Promotion recovery compares it with the filesDigest of render.json.
func CandidateFilesDigest(dir string) (canon.Sum[canon.CandidateFilesKind], error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return canon.Sum[canon.CandidateFilesKind]{}, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = root.Close() }()
	files, err := candidateTree(root.FS())
	if err != nil {
		return canon.Sum[canon.CandidateFilesKind]{}, err
	}
	return candidateFilesDigest(files)
}

func candidateFilesDigest(files []gate.File) (canon.Sum[canon.CandidateFilesKind], error) {
	in := make(candidateFiles, 0, len(files))
	for _, f := range files {
		s := sha256.Sum256(f.Data)
		in = append(in, candidateFile{Path: f.Path, SHA256: hex.EncodeToString(s[:])})
	}
	slices.SortFunc(in, func(a, b candidateFile) int { return cmp.Compare(a.Path, b.Path) })
	return canon.Digest(canon.CandidateFiles, in)
}

// step runs the test hook after step n.
func (l Live) step(n int) error {
	if l.afterStep == nil {
		return nil
	}
	return l.afterStep(n)
}
