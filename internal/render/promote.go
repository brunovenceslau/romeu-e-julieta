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
	// ErrPromotionAbandoned is recovery's report of a promotion whose
	// candidate no longer verifies before the commit point: the caller
	// maps it to interrupted-promotion-abandoned, exit 4 (REQ-0429).
	ErrPromotionAbandoned = errors.New("render: the interrupted promotion was abandoned; its candidate no longer verifies")
	// ErrKitDirDiffers is step 2's refusal of a live kit dir whose
	// content is not its digest: a kit dir is verified, never rewritten.
	ErrKitDirDiffers = errors.New("render: a live kit dir differs from its digest")
)

// Recovery is what Recover did.
type Recovery int

const (
	// RecoveryNone means render.json held no promoting, so there was
	// nothing to recover.
	RecoveryNone Recovery = iota
	// RecoveryFinished means the promotion was finished by repeating steps 2
	// to 4.
	RecoveryFinished
	// RecoveryAbandoned means the promotion was abandoned (ErrPromotionAbandoned).
	RecoveryAbandoned
)

// WorkspaceFile is one workspace file step 2 writes whole (REQ-0048):
// derived from host state, under $ROMEU_ROOT rather than <name>-env/,
// so render.json holds no digest of it.
type WorkspaceFile struct {
	// Path is the absolute path of the file.
	Path string
	// Data is its bytes.
	Data []byte
}

// Live is one <name>-env/ that a promotion writes, with what step 2
// writes that does not come from the candidate: the julieta binary and
// the workspace files.
type Live struct {
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

// Promote runs the promotion commit of 01 1.6 for the candidate c,
// which holds the render.json the gate approved: (1) render.json with
// promoting, (2) the kit dirs, .romeu/bin and the workspace files,
// (3) sbxenv.yaml renamed into place, the commit point, (4) render.json
// without promoting, the candidate dir and the unreferenced kit dirs
// deleted. The caller holds the lock and has run Recover.
func (l Live) Promote(c Candidate) error {
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
	data, err := root.ReadFile(path.Join(candidateDir(id), CandidateRecord))
	if err != nil {
		return fmt.Errorf("render: read the candidate's render.json: %w", err)
	}
	rec, err := DecodeRecord(data)
	if err != nil {
		return err
	}
	digest, err := CandidateFilesDigest(c.Dir)
	if err != nil {
		return err
	}
	previous := map[string]string{}
	if found {
		previous = live.Files
	}
	rec.Promoting = &Promoting{CandidateID: id, FilesDigest: digest, PreviousFiles: previous}
	if err := writeLiveRecord(root, rec); err != nil {
		return err
	}
	if err := l.step(1); err != nil {
		return err
	}
	return l.finish(root, rec)
}

// Recover finishes or abandons a promotion that render.json says was
// interrupted (REQ-0087). It runs after the lock in every mutating
// command. A candidate that still verifies against its filesDigest, or
// a live sbxenv.yaml that is already the new one (the commit point
// passed), is finished by repeating steps 2 to 4, each idempotent.
// Otherwise render.json is written back with the previous digests plus
// the digests of what step 2 had written, and the result is
// ErrPromotionAbandoned; the old live sbxenv.yaml and its kit dirs are
// intact.
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
	verifies, err := candidateVerifies(root, *rec.Promoting)
	if err != nil {
		return RecoveryNone, err
	}
	committed, err := liveMatches(root, LiveEnvFile, rec.Files[LiveEnvFile])
	if err != nil {
		return RecoveryNone, err
	}
	if verifies || committed {
		if err := l.finish(root, rec); err != nil {
			return RecoveryNone, err
		}
		return RecoveryFinished, nil
	}
	if err := abandon(root, rec); err != nil {
		return RecoveryNone, err
	}
	return RecoveryAbandoned, ErrPromotionAbandoned
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

// finish runs steps 2 to 4 for rec, the record step 1 wrote.
func (l Live) finish(root *os.Root, rec Record) error {
	cand := candidateDir(rec.Promoting.CandidateID)
	if err := l.writeDerived(root, rec, cand); err != nil {
		return err
	}
	if err := l.step(2); err != nil {
		return err
	}
	if err := commitEnv(root, rec, cand); err != nil {
		return err
	}
	if err := l.step(3); err != nil {
		return err
	}
	rec.Promoting = nil
	if err := writeLiveRecord(root, rec); err != nil {
		return err
	}
	if err := removeCandidateDir(root, cand); err != nil {
		return err
	}
	return pruneKitDirs(root, rec.Files)
}

// writeDerived is step 2: each kit dir rec names, copied from the
// candidate when it is not live yet, the bin and the workspace files.
func (l Live) writeDerived(root *os.Root, rec Record, cand string) error {
	for _, p := range slices.Sorted(maps.Keys(rec.Files)) {
		if !isKitPath(p) {
			continue
		}
		if err := ensureKitDir(root, p, rec.Files[p], path.Join(cand, CandidateKitsDir, path.Base(p))); err != nil {
			return err
		}
	}
	for p, d := range l.Bin.Files() {
		if rec.Files[p] != d {
			return fmt.Errorf("render: render.json holds another digest for %s than the julieta this romeu embeds", p)
		}
	}
	if err := l.Bin.WriteTo(root); err != nil {
		return err
	}
	for _, w := range l.Workspace {
		if err := writeWorkspace(w); err != nil {
			return err
		}
	}
	return nil
}

// commitEnv is step 3, the commit point: the candidate's sbxenv.yaml,
// checked against its digest, replaces the live one by rename. The
// candidate keeps its copy, so it still verifies until step 4. A live
// file that is already the new one is left as it is.
func commitEnv(root *os.Root, rec Record, cand string) error {
	want := rec.Files[LiveEnvFile]
	done, err := liveMatches(root, LiveEnvFile, want)
	if err != nil || done {
		return err
	}
	data, err := root.ReadFile(path.Join(cand, CandidateEnvFile))
	if err != nil {
		return fmt.Errorf("render: read the candidate's sbxenv.yaml: %w", err)
	}
	if plainSum(data) != want {
		return fmt.Errorf("render: the candidate's sbxenv.yaml is not the one render.json names")
	}
	if err := replaceFile(root, LiveEnvFile, data, FileMode); err != nil {
		return err
	}
	return syncDir(root)
}

// abandon writes render.json back without promoting: the previous
// digests, plus each path of the new files whose live content step 2
// had already written, then deletes the candidate dir.
func abandon(root *os.Root, rec Record) error {
	files := maps.Clone(rec.Promoting.PreviousFiles)
	if files == nil {
		files = map[string]string{}
	}
	for p, d := range rec.Files {
		if p == LiveEnvFile {
			continue
		}
		ok, err := liveMatches(root, p, d)
		if err != nil {
			return err
		}
		if ok {
			files[p] = d
		}
	}
	cand := candidateDir(rec.Promoting.CandidateID)
	rec.Files = files
	rec.Promoting = nil
	if err := writeLiveRecord(root, rec); err != nil {
		return err
	}
	return removeCandidateDir(root, cand)
}

// ensureKitDir makes the live kit dir p hold the kit of digest want: a
// dir that exists is verified, never rewritten; a missing one is copied
// from the candidate's kit dir src into a temporary dir, checked, and
// renamed to its new name.
func ensureKitDir(root *os.Root, p, want, src string) error {
	if _, err := root.Stat(p); err == nil {
		got, err := kitDirDigest(root, p)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("%w: %s", ErrKitDirDiffers, p)
		}
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("render: %w", err)
	}
	sub, err := root.OpenRoot(src)
	if err != nil {
		return fmt.Errorf("render: the candidate has no kit dir for %s: %w", p, err)
	}
	files, err := kitFiles(sub.FS())
	_ = sub.Close()
	if err != nil {
		return fmt.Errorf("render: read the candidate's kit dir %s: %w", src, err)
	}
	got, err := gate.KitTreeDigest(files)
	if err != nil {
		return err
	}
	if got.String() != want {
		return fmt.Errorf("render: the candidate's kit dir %s is not the one render.json names", src)
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

// pruneKitDirs deletes every dir under LiveKitsDir that files does not
// name: the kit dirs the live sbxenv.yaml no longer references, and a
// temporary dir a killed step 2 left. It runs after the commit point
// alone. It is an admitted call site of the I16 scan (05 5.2).
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
		if _, kept := files[p]; kept {
			continue
		}
		if err := root.RemoveAll(p); err != nil {
			return fmt.Errorf("render: %w", err)
		}
	}
	return nil
}

// writeWorkspace replaces one workspace file by the atomic write rule.
func writeWorkspace(w WorkspaceFile) error {
	if !filepath.IsAbs(w.Path) {
		return fmt.Errorf("render: the workspace file %s is not an absolute path", w.Path)
	}
	dir, err := os.OpenRoot(filepath.Dir(w.Path))
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	defer func() { _ = dir.Close() }()
	if err := replaceFile(dir, filepath.Base(w.Path), w.Data, FileMode); err != nil {
		return err
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

// candidateVerifies reports whether the candidate dir of p still holds
// the files whose digest p recorded; a missing dir does not verify.
func candidateVerifies(root *os.Root, p Promoting) (bool, error) {
	sub, err := root.OpenRoot(candidateDir(p.CandidateID))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("render: %w", err)
	}
	defer func() { _ = sub.Close() }()
	// A candidate that cannot be read, or holds what is not a regular
	// file, does not verify.
	got, err := candidateFilesDigest(sub.FS())
	return err == nil && got == p.FilesDigest, nil
}

// candidateDir returns the path of the candidate id in <name>-env/.
func candidateDir(id canon.ULID) string { return path.Join(CandidatesDir, id.String()) }

// isKitPath reports whether the render.json path p is a kit dir.
func isKitPath(p string) bool { return strings.HasPrefix(p, LiveKitsDir+"/") }

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
	return candidateFilesDigest(root.FS())
}

func candidateFilesDigest(fsys fs.FS) (canon.Sum[canon.CandidateFilesKind], error) {
	var files candidateFiles
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%q is not a regular file", p)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		s := sha256.Sum256(data)
		files = append(files, candidateFile{Path: p, SHA256: hex.EncodeToString(s[:])})
		return nil
	})
	if err != nil {
		return canon.Sum[canon.CandidateFilesKind]{}, fmt.Errorf("render: the candidate's files: %w", err)
	}
	slices.SortFunc(files, func(a, b candidateFile) int { return cmp.Compare(a.Path, b.Path) })
	return canon.Digest(canon.CandidateFiles, files)
}

// step runs the test hook after step n.
func (l Live) step(n int) error {
	if l.afterStep == nil {
		return nil
	}
	return l.afterStep(n)
}
