// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package render_test

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
)

// errKilled is what the step hook returns to stop a promotion after a
// step, the way a killed romeu would leave it.
var errKilled = errors.New("killed by the test")

// promoKit is one kit of a fixture candidate.
type promoKit struct {
	id    string
	files []gate.File
}

// candidateFixture writes a candidate dir under envDir with the env
// bytes, the kits and a render.json whose files map names them and the
// bin, and returns it with its id.
func candidateFixture(t *testing.T, envDir string, idByte byte, env string, bin render.Bin, kits ...promoKit) (render.Candidate, canon.ULID) {
	t.Helper()
	id := testULID(t, idByte)
	dir := filepath.Join(envDir, filepath.FromSlash(render.CandidatesDir), id.String())
	require.NoError(t, os.MkdirAll(dir, 0o700))
	c := render.Candidate{Dir: dir}
	files := map[string]string{"sbxenv.yaml": sumOf([]byte(env))}
	maps.Copy(files, bin.Files())
	for _, k := range kits {
		digest, err := gate.KitTreeDigest(k.files)
		require.NoError(t, err)
		mk := render.MaterializedKit{ID: k.id, Digest: digest}
		for _, f := range k.files {
			name := filepath.Join(dir, render.CandidateKitsDir, mk.Dir(), filepath.FromSlash(f.Path))
			require.NoError(t, os.MkdirAll(filepath.Dir(name), 0o700))
			mode := os.FileMode(0o600)
			if f.Mode == gate.ModeExecutable {
				mode = 0o700
			}
			require.NoError(t, os.WriteFile(name, f.Data, mode))
		}
		files[mk.LivePath()] = digest.String()
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, render.CandidateEnvFile), []byte(env), 0o600))
	rec := shopRecord(t)
	rec.Files = files
	// Each candidate carries its own metadata, so a test can tell whose
	// record is live.
	c0 := "0123456789abcdef"[idByte%16]
	rec.SpecSource.Commit = strings.Repeat(string(c0), 40)
	rec.WideningDigest = hexSum[canon.WideningKind](t, c0)
	rec.RecreateDigest = hexSum[canon.RecreateKind](t, c0)
	require.NoError(t, render.WriteRecord(c, rec))
	return c, id
}

// testULID returns a ULID whose bytes are all b.
func testULID(t *testing.T, b byte) canon.ULID {
	t.Helper()
	var u canon.ULID
	for i := range u {
		u[i] = b
	}
	got, err := canon.ParseULID(u.String())
	require.NoError(t, err)
	return got
}

func kitA() promoKit {
	return promoKit{id: "julieta", files: []gate.File{
		{Path: "julieta.yaml", Mode: gate.ModeRegular, Data: []byte("name: julieta\n")},
		{Path: "bin/setup", Mode: gate.ModeExecutable, Data: []byte("#!/bin/sh\n")},
	}}
}

func kitB(content string) promoKit {
	return promoKit{id: "my-home", files: []gate.File{
		{Path: "my-home.yaml", Mode: gate.ModeRegular, Data: []byte(content)},
	}}
}

// promoEnv is one <name>-env/ under test, with its workspace root.
type promoEnv struct {
	envDir string
	ws     string
	bin    render.Bin
}

func newPromoEnv(t *testing.T) promoEnv {
	t.Helper()
	root := t.TempDir()
	envDir := filepath.Join(root, "shop-env")
	require.NoError(t, os.MkdirAll(envDir, 0o700))
	bin, err := render.NewBin("arm64", []byte("julieta binary"))
	require.NoError(t, err)
	return promoEnv{envDir: envDir, ws: filepath.Join(root, "dev.code-workspace"), bin: bin}
}

func (e promoEnv) live() render.Live {
	return render.Live{
		EnvDir:    e.envDir,
		Bin:       e.bin,
		Workspace: []render.WorkspaceFile{{Path: e.ws, Data: []byte("{\"folders\": []}\n")}},
	}
}

// tree returns every regular file under dir with its bytes, by its
// slash path, leaving out the candidates dir.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		require.NoError(t, err)
		rel = filepath.ToSlash(rel)
		if rel == render.CandidatesDir {
			return fs.SkipDir
		}
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			require.NoError(t, err)
			out[rel] = string(b)
		}
		return nil
	}))
	return out
}

func readRecord(t *testing.T, envDir string) render.Record {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(envDir, filepath.FromSlash(render.LiveRecord)))
	require.NoError(t, err)
	r, err := render.DecodeRecord(b)
	require.NoError(t, err)
	return r
}

func TestPromoteWritesTheLiveFilesAndDeletesTheCandidate(t *testing.T) {
	e := newPromoEnv(t)
	c, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
	cand := tree(t, c.Dir)

	require.NoError(t, e.live().Promote(c))

	got := tree(t, e.envDir)
	assert.Equal(t, "env: one\n", got[render.LiveEnvFile])
	assert.Equal(t, "julieta binary", got[".romeu/bin/julieta-linux-arm64"])
	assert.Contains(t, got[".romeu/bin/SHA256SUMS"], "julieta-linux-arm64")
	for p, data := range cand {
		if rest, ok := strings.CutPrefix(p, render.CandidateKitsDir+"/"); ok {
			assert.Equal(t, data, got[path.Join(render.LiveKitsDir, rest)], p)
		}
	}
	rec := readRecord(t, e.envDir)
	assert.Nil(t, rec.Promoting)
	assert.NoDirExists(t, c.Dir)
	ws, err := os.ReadFile(e.ws)
	require.NoError(t, err)
	assert.JSONEq(t, `{"folders": []}`, string(ws))

	drifted, err := render.CheckDrift(e.envDir)
	require.NoError(t, err)
	assert.Empty(t, drifted)
	interrupted, err := render.Interrupted(e.envDir)
	require.NoError(t, err)
	assert.False(t, interrupted)
}

func TestPromoteDeletesOnlyTheKitDirsNoLongerReferenced(t *testing.T) {
	e := newPromoEnv(t)
	c1, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA(), kitB("v: 1\n"))
	require.NoError(t, e.live().Promote(c1))
	first := readRecord(t, e.envDir)

	c2, _ := candidateFixture(t, e.envDir, 2, "env: two\n", e.bin, kitA(), kitB("v: 2\n"))
	require.NoError(t, e.live().Promote(c2))
	second := readRecord(t, e.envDir)

	for p := range first.Files {
		if !strings.HasPrefix(p, render.LiveKitsDir+"/") {
			continue
		}
		if _, kept := second.Files[p]; kept {
			assert.DirExists(t, filepath.Join(e.envDir, filepath.FromSlash(p)))
		} else {
			assert.NoDirExists(t, filepath.Join(e.envDir, filepath.FromSlash(p)))
		}
	}
	drifted, err := render.CheckDrift(e.envDir)
	require.NoError(t, err)
	assert.Empty(t, drifted)
}

// The fault injection of 01 1.6 (REQ-0081, I7): after each step a
// killed promotion is finished by recovery, and the end state is the
// one a clean promotion leaves.
func TestRecoverFinishesAPromotionKilledAfterEachStep(t *testing.T) {
	clean := newPromoEnv(t)
	c, _ := candidateFixture(t, clean.envDir, 1, "env: one\n", clean.bin, kitA())
	require.NoError(t, clean.live().Promote(c))
	want := tree(t, clean.envDir)

	for step := 1; step <= 3; step++ {
		t.Run(stepName(step), func(t *testing.T) {
			e := newPromoEnv(t)
			c, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
			l := e.live()
			render.SetAfterStep(&l, func(n int) error {
				if n == step {
					return errKilled
				}
				return nil
			})
			require.ErrorIs(t, l.Promote(c), errKilled)

			interrupted, err := render.Interrupted(e.envDir)
			require.NoError(t, err)
			assert.True(t, interrupted)
			_, err = render.CheckDrift(e.envDir)
			require.ErrorIs(t, err, render.ErrPromotionInterrupted, "an interrupted promotion is never drift")
			require.ErrorIs(t, e.live().Promote(c), render.ErrPromotionInterrupted, "recovery runs first")

			got, err := e.live().Recover()
			require.NoError(t, err)
			assert.Equal(t, render.RecoveryFinished, got)
			assert.Equal(t, want, tree(t, e.envDir))
			assert.NoDirExists(t, c.Dir)

			again, err := e.live().Recover()
			require.NoError(t, err)
			assert.Equal(t, render.RecoveryNone, again)
		})
	}
}

// REQ-0087: a candidate that no longer verifies, before the commit
// point, is abandoned: render.json holds the previous digests plus
// step 2's, the exit is 4, and drift is clean afterwards.
func TestRecoverAbandonsAPromotionWhoseCandidateNoLongerVerifies(t *testing.T) {
	for step := 1; step <= 2; step++ {
		t.Run(stepName(step), func(t *testing.T) {
			e := newPromoEnv(t)
			c0, _ := candidateFixture(t, e.envDir, 1, "env: zero\n", e.bin, kitB("v: 0\n"))
			require.NoError(t, e.live().Promote(c0))
			before := readRecord(t, e.envDir)

			c, _ := candidateFixture(t, e.envDir, 2, "env: one\n", e.bin, kitA())
			l := e.live()
			render.SetAfterStep(&l, func(n int) error {
				if n == step {
					return errKilled
				}
				return nil
			})
			require.ErrorIs(t, l.Promote(c), errKilled)
			require.NoError(t, os.WriteFile(filepath.Join(c.Dir, render.CandidateEnvFile), []byte("env: tampered\n"), 0o600))

			got, err := e.live().Recover()
			require.ErrorIs(t, err, render.ErrPromotionAbandoned)
			assert.Equal(t, render.RecoveryAbandoned, got)

			live, err := os.ReadFile(filepath.Join(e.envDir, render.LiveEnvFile))
			require.NoError(t, err)
			assert.Equal(t, "env: zero\n", string(live), "nothing unapproved is live")
			rec := readRecord(t, e.envDir)
			assert.Nil(t, rec.Promoting)
			for p, d := range before.Files {
				assert.Equal(t, d, rec.Files[p], p)
			}
			// Nothing of the candidate's record is live.
			assert.Equal(t, before.SpecSource, rec.SpecSource)
			assert.Equal(t, before.Egress, rec.Egress)
			assert.Equal(t, before.WideningDigest, rec.WideningDigest)
			assert.Equal(t, before.RecreateDigest, rec.RecreateDigest)
			assert.Equal(t, before.CatalogDigest, rec.CatalogDigest)
			drifted, err := render.CheckDrift(e.envDir)
			require.NoError(t, err)
			assert.Empty(t, drifted)
			assert.NoDirExists(t, c.Dir)
		})
	}
}

// A double fault: killed after the commit point, then the candidate
// changed. No trusted digest of the new sbxenv.yaml is left, so the
// promotion is abandoned and the next drift check names sbxenv.yaml,
// which a re-sync or --overwrite-drift resolves. The live file is the
// approved candidate's, so nothing unapproved is live.
func TestRecoverAbandonsAfterTheCommitPointWhenTheCandidateChanged(t *testing.T) {
	e := newPromoEnv(t)
	c0, _ := candidateFixture(t, e.envDir, 1, "env: zero\n", e.bin, kitB("v: 0\n"))
	require.NoError(t, e.live().Promote(c0))
	before := readRecord(t, e.envDir)

	c, _ := candidateFixture(t, e.envDir, 2, "env: one\n", e.bin, kitA())
	l := e.live()
	render.SetAfterStep(&l, func(n int) error {
		if n == 3 {
			return errKilled
		}
		return nil
	})
	require.ErrorIs(t, l.Promote(c), errKilled)
	require.NoError(t, os.WriteFile(filepath.Join(c.Dir, render.CandidateEnvFile), []byte("env: tampered\n"), 0o600))

	got, err := e.live().Recover()
	require.ErrorIs(t, err, render.ErrPromotionAbandoned)
	assert.Equal(t, render.RecoveryAbandoned, got)
	rec := readRecord(t, e.envDir)
	assert.Nil(t, rec.Promoting)
	assert.Equal(t, before.WideningDigest, rec.WideningDigest)
	drifted, err := render.CheckDrift(e.envDir)
	require.NoError(t, err)
	assert.Equal(t, []string{render.LiveEnvFile}, drifted)
}

// The first promotion has no live record: step 1 writes the
// candidate's with no files. Abandoned before the commit point, nothing
// is live, and the record names only what step 2 wrote.
func TestRecoverAbandonsTheFirstPromotion(t *testing.T) {
	for step := 1; step <= 2; step++ {
		t.Run(stepName(step), func(t *testing.T) {
			e := newPromoEnv(t)
			c, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
			l := e.live()
			render.SetAfterStep(&l, func(n int) error {
				if n == step {
					return errKilled
				}
				return nil
			})
			require.ErrorIs(t, l.Promote(c), errKilled)
			require.NoError(t, os.WriteFile(filepath.Join(c.Dir, render.CandidateEnvFile), []byte("env: tampered\n"), 0o600))

			got, err := e.live().Recover()
			require.ErrorIs(t, err, render.ErrPromotionAbandoned)
			assert.Equal(t, render.RecoveryAbandoned, got)
			assert.NoFileExists(t, filepath.Join(e.envDir, render.LiveEnvFile))
			rec := readRecord(t, e.envDir)
			assert.Nil(t, rec.Promoting)
			assert.NotContains(t, rec.Files, render.LiveEnvFile)
			drifted, err := render.CheckDrift(e.envDir)
			require.NoError(t, err)
			assert.Empty(t, drifted)
		})
	}
}

func TestPromoteVerifiesAnExistingKitDirAndNeverRewritesIt(t *testing.T) {
	e := newPromoEnv(t)
	c1, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
	require.NoError(t, e.live().Promote(c1))
	rec := readRecord(t, e.envDir)
	var kitDir string
	for p := range rec.Files {
		if strings.HasPrefix(p, render.LiveKitsDir+"/") {
			kitDir = filepath.Join(e.envDir, filepath.FromSlash(p))
		}
	}
	info, err := os.Stat(filepath.Join(kitDir, "julieta.yaml"))
	require.NoError(t, err)

	c2, _ := candidateFixture(t, e.envDir, 2, "env: two\n", e.bin, kitA())
	require.NoError(t, e.live().Promote(c2))
	again, err := os.Stat(filepath.Join(kitDir, "julieta.yaml"))
	require.NoError(t, err)
	assert.True(t, os.SameFile(info, again), "the kit dir is left as it was")

	require.NoError(t, os.WriteFile(filepath.Join(kitDir, "julieta.yaml"), []byte("changed\n"), 0o600))
	c3, _ := candidateFixture(t, e.envDir, 3, "env: three\n", e.bin, kitA())
	require.ErrorIs(t, e.live().Promote(c3), render.ErrKitDirDiffers)
}

// I15 (REQ-0498): a hand edit of sbxenv.yaml, of a kit file or of
// SHA256SUMS is drift, named by its path.
func TestCheckDriftNamesEachHandEditedFile(t *testing.T) {
	cases := map[string]func(t *testing.T, envDir string, rec render.Record) string{
		"sbxenv.yaml": func(t *testing.T, envDir string, _ render.Record) string {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(envDir, render.LiveEnvFile), []byte("env: widened\n"), 0o600))
			return render.LiveEnvFile
		},
		"a kit file": func(t *testing.T, envDir string, rec render.Record) string {
			t.Helper()
			for p := range rec.Files {
				if strings.HasPrefix(p, render.LiveKitsDir+"/") {
					require.NoError(t, os.WriteFile(filepath.Join(envDir, filepath.FromSlash(p), "julieta.yaml"), []byte("x\n"), 0o600))
					return p
				}
			}
			t.Fatal("no kit dir")
			return ""
		},
		"SHA256SUMS": func(t *testing.T, envDir string, _ render.Record) string {
			t.Helper()
			p := path.Join(render.BinDir, render.SumsFile)
			require.NoError(t, os.WriteFile(filepath.Join(envDir, filepath.FromSlash(p)), []byte("0  x\n"), 0o644))
			return p
		},
		"a deleted file": func(t *testing.T, envDir string, _ render.Record) string {
			t.Helper()
			require.NoError(t, os.Remove(filepath.Join(envDir, render.LiveEnvFile)))
			return render.LiveEnvFile
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			e := newPromoEnv(t)
			c, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
			require.NoError(t, e.live().Promote(c))
			want := edit(t, e.envDir, readRecord(t, e.envDir))

			drifted, err := render.CheckDrift(e.envDir)
			require.NoError(t, err)
			assert.Equal(t, []string{want}, drifted)
		})
	}
}

func TestCheckDriftWithNoLiveRecordIsClean(t *testing.T) {
	drifted, err := render.CheckDrift(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, drifted)
}

func TestCandidateFilesDigestFollowsTheContent(t *testing.T) {
	e := newPromoEnv(t)
	c, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
	d1, err := render.CandidateFilesDigest(c.Dir)
	require.NoError(t, err)
	d2, err := render.CandidateFilesDigest(c.Dir)
	require.NoError(t, err)
	assert.Equal(t, d1, d2)
	require.NoError(t, os.WriteFile(filepath.Join(c.Dir, render.CandidateEnvFile), []byte("env: two\n"), 0o600))
	d3, err := render.CandidateFilesDigest(c.Dir)
	require.NoError(t, err)
	assert.NotEqual(t, d1, d3)
}

func stepName(n int) string { return "killed after step " + string(rune('0'+n)) }

// A temporary kit dir a killed step 2 left half written is replaced,
// and an entry of .romeu/kits that is not a kit dir stays.
func TestPromoteReplacesAStaleTempKitDirAndKeepsWhatIsNotAKitDir(t *testing.T) {
	e := newPromoEnv(t)
	c, _ := candidateFixture(t, e.envDir, 1, "env: one\n", e.bin, kitA())
	rec, err := os.ReadFile(filepath.Join(c.Dir, render.CandidateRecord))
	require.NoError(t, err)
	r, err := render.DecodeRecord(rec)
	require.NoError(t, err)
	var kit string
	for p := range r.Files {
		if strings.HasPrefix(p, render.LiveKitsDir+"/") {
			kit = path.Base(p)
		}
	}
	kits := filepath.Join(e.envDir, filepath.FromSlash(render.LiveKitsDir))
	stale := filepath.Join(kits, ".tmp-"+kit)
	require.NoError(t, os.MkdirAll(stale, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(stale, "half"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(kits, "notes.txt"), []byte("mine\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(kits, "not-a-kit"), 0o700))

	require.NoError(t, e.live().Promote(c))

	assert.NoDirExists(t, stale)
	assert.NoFileExists(t, filepath.Join(kits, kit, "half"))
	assert.FileExists(t, filepath.Join(kits, "notes.txt"))
	assert.DirExists(t, filepath.Join(kits, "not-a-kit"))
	drifted, err := render.CheckDrift(e.envDir)
	require.NoError(t, err)
	assert.Empty(t, drifted)
}
