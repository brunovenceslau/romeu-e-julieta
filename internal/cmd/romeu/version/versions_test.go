// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package version

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/egress"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gate"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/render"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
)

// The digests of the fixtures.
var (
	workloadDigest = "sha256:" + strings.Repeat("e", 64)
	julietaDigest  = "0123456789ab" + strings.Repeat("1", 52)
	personalDigest = "abcdef012345" + strings.Repeat("2", 52)
)

// fixture is a machine for readVersions: its environment, with the
// settings and the state directory in a temporary dir, and the root of
// the trees.
type fixture struct {
	env      map[string]string
	settings string
	state    string
	root     string
}

// newFixture writes host settings whose root is a temporary dir, and
// makes the state directory with the records given; a nil record writes
// raw bytes that do not decode instead.
func newFixture(t *testing.T, records map[string]*state.Project) fixture {
	t.Helper()
	tmp := t.TempDir()
	f := fixture{root: filepath.Join(tmp, "Developer"), settings: filepath.Join(tmp, "s.yaml"), state: filepath.Join(tmp, "state")}
	f.env = map[string]string{"HOME": tmp, "XDG_STATE_HOME": f.state}
	data, err := spec.EncodeHostSettings(spec.HostSettings{
		Schema: "host-settings/v1",
		Root:   f.root,
		Config: spec.ConfigRepo{Project: "cfg", URL: "https://github.com/owner/cfg", Ref: "main"},
		Tools:  spec.Tools{Sbx: "/usr/bin/sbx", Git: "/usr/bin/git"},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(f.settings, data, 0o600))
	require.NoError(t, os.MkdirAll(f.root, 0o700))
	if records == nil {
		return f
	}
	s, err := state.Open(filepath.Join(f.state, "romeu"))
	require.NoError(t, err)
	require.NoError(t, s.Close())
	for name, p := range records {
		data := []byte("{")
		if p != nil {
			data, err = state.EncodeProject(p)
			require.NoError(t, err)
		}
		require.NoError(t, os.WriteFile(filepath.Join(f.state, "romeu", "projects", name+".json"), data, state.FileMode))
	}
	return f
}

// approved is a record of shop whose gate 2 approval names the fixture
// workload.
func approved(t *testing.T) *state.Project {
	t.Helper()
	set := gate.Set{
		Project:      "shop",
		Workload:     gate.Workload{Repository: "docker.io/docker/sbx-kit-claude", Digest: workloadDigest},
		RenderDigest: canon.Sum[canon.RenderKind]{2},
	}
	sum, err := gate.Digest(set)
	require.NoError(t, err)
	p := state.NewProject()
	p.Approval = &state.Approval{
		WideningDigest: sum, Set: set, RenderDigest: set.RenderDigest,
		SpecCommit: strings.Repeat("c", 40), CandidateID: canon.ULID{1},
		ApprovedAt: time.Date(2026, 10, 2, 9, 10, 0, 0, time.UTC),
	}
	return p
}

// liveRender writes <root>/shop-env/.romeu/render.json whose files hold
// sbxenv.yaml and the kit dirs of julieta and of the personal kit
// my-tools; edit changes the record before it is written.
func liveRender(t *testing.T, root string, edit func(*render.Record)) {
	t.Helper()
	env := filepath.Join(root, "shop-env")
	require.NoError(t, os.MkdirAll(filepath.Join(env, ".romeu"), 0o700))
	eg, err := render.EgressOf(egress.Result{})
	require.NoError(t, err)
	rec := render.Record{
		RomeuVersion: "v1.0.0",
		SpecSource:   render.SpecSource{Kind: render.SpecSourceOrigin, Ref: "refs/romeu/origin/cfg/main", Commit: strings.Repeat("c", 40)},
		Repos:        []render.RecordRepo{},
		Files: map[string]string{
			render.LiveEnvFile: strings.Repeat("3", 64),
			render.LiveKitsDir + "/julieta-" + julietaDigest[:12]:   julietaDigest,
			render.LiveKitsDir + "/my-tools-" + personalDigest[:12]: personalDigest,
		},
		Egress: eg,
	}
	if edit != nil {
		edit(&rec)
	}
	data, err := render.EncodeRecord(rec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(env, render.LiveRecord), data, 0o600))
}

// calls counts the calls of the tool seams.
type calls struct{ sbx, git int }

// seamsOf returns seams over the fixture's environment whose sbx, git
// and macOS answer with the values or errors given, counting the calls
// of sbx and git into c.
func (f fixture) seamsOf(c *calls, sbx, git, macOS string, sbxErr, gitErr, macOSErr error) seams {
	return seams{
		getenv: func(k string) string { return f.env[k] },
		sbx: func(_ context.Context, h *spec.HostSettings, home string) (string, error) {
			c.sbx++
			if h.Tools.Sbx != "/usr/bin/sbx" || home != f.env["HOME"] {
				return "", errors.New("the settings or HOME did not reach sbx")
			}
			return sbx, sbxErr
		},
		git: func(_ context.Context, h *spec.HostSettings, home string) (string, error) {
			c.git++
			if h.Tools.Git != "/usr/bin/git" || home != f.env["HOME"] {
				return "", errors.New("the settings or HOME did not reach git")
			}
			return git, gitErr
		},
		macOS: func() (string, error) { return macOS, macOSErr },
	}
}

// read runs readVersions over the fixture with the seams.
func (f fixture) read(t *testing.T, s seams) Versions {
	t.Helper()
	return readVersions(t.Context(), &cli.Env{SettingsPath: f.settings}, s)
}

// ids returns the read and the id of each error, in order.
func ids(errs []ReadError) []string {
	out := []string{}
	for _, e := range errs {
		out = append(out, e.Read+" "+e.ID+" "+e.Slug)
	}
	return out
}

// TestReadVersions: each member comes from its read, and a read that
// fails is listed by its error id while the others go on.
func TestReadVersions(t *testing.T) {
	pins := []KitPin{{Kit: "julieta", Digest: julietaDigest}, {Kit: "my-tools", Digest: personalDigest}}
	floor := &gitsafe.FloorError{Found: "banana"}
	shape := cli.UpstreamShape.New("sbx version printed no version")
	tests := []struct {
		name     string
		records  map[string]*state.Project
		setup    func(t *testing.T, f *fixture)
		sbxErr   error
		gitErr   error
		macOSErr error
		calls    calls
		want     Versions
	}{
		{
			name: "every member read",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				*f = newFixture(t, map[string]*state.Project{"shop": approved(t)})
				liveRender(t, f.root, nil)
			},
			calls: calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Errors: []ReadError{},
				Projects: []ProjectVersions{{Name: "shop", Workload: workloadDigest, Kits: pins, Errors: []ReadError{}}}},
		},
		{
			name:  "no host state: no project, and no failed read",
			calls: calls{1, 1},
			want:  Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Projects: []ProjectVersions{}, Errors: []ReadError{}},
		},
		{
			name:    "a record never approved and a project never promoted",
			records: map[string]*state.Project{"shop": state.NewProject()},
			calls:   calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Errors: []ReadError{},
				Projects: []ProjectVersions{{Name: "shop", Errors: []ReadError{}}}},
		},
		{
			name: "an interrupted promotion: the record is the candidate's, so no pins",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				*f = newFixture(t, map[string]*state.Project{"shop": approved(t)})
				liveRender(t, f.root, func(r *render.Record) {
					r.Promoting = &render.Promoting{CandidateID: canon.ULID{3}, FilesDigest: canon.Sum[canon.CandidateFilesKind]{4}, PreviousFiles: map[string]string{}}
				})
			},
			calls: calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Errors: []ReadError{},
				Projects: []ProjectVersions{{Name: "shop", Workload: workloadDigest, Errors: []ReadError{}}}},
		},
		{
			name: "a render with no kit: an empty list, not an absent one",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				*f = newFixture(t, map[string]*state.Project{"shop": state.NewProject()})
				liveRender(t, f.root, func(r *render.Record) { r.Files = map[string]string{render.LiveEnvFile: strings.Repeat("3", 64)} })
			},
			calls: calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Errors: []ReadError{},
				Projects: []ProjectVersions{{Name: "shop", Kits: []KitPin{}, Errors: []ReadError{}}}},
		},
		{
			name:     "sbx, git and macOS that do not read, each by its id",
			sbxErr:   shape,
			gitErr:   floor,
			macOSErr: cli.Internal.New("the macOS version is not one"),
			calls:    calls{1, 1},
			want: Versions{Projects: []ProjectVersions{}, Errors: []ReadError{
				{ID: "RJ-348", Slug: "internal", Read: readMacOS, Message: "the macOS version is not one"},
				{ID: shape.ID, Slug: shape.Slug, Read: readSbx, Message: shape.Error()},
				{ID: "RJ-344", Slug: "git-floor", Read: readGit, Message: floor.Error()},
			}},
		},
		{
			name:   "an error with no id is internal",
			gitErr: errors.New("boom"),
			calls:  calls{1, 1},
			want: Versions{Sbx: "0.49.2", MacOS: "15.1", Projects: []ProjectVersions{}, Errors: []ReadError{
				{ID: "RJ-348", Slug: "internal", Read: readGit, Message: "boom"},
			}},
		},
		{
			name: "no host settings: no sbx, no git, and the projects still read",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				*f = newFixture(t, map[string]*state.Project{"shop": approved(t)})
				require.NoError(t, os.Remove(f.settings))
			},
			want: Versions{MacOS: "15.1",
				Projects: []ProjectVersions{{Name: "shop", Workload: workloadDigest, Errors: []ReadError{}}},
				Errors:   []ReadError{{ID: "RJ-349", Slug: "usage", Read: readSettings}}},
		},
		{
			name: "a HOME the drivers refuse: neither sbx nor git runs",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				f.env["HOME"] = "relative/home"
			},
			want: Versions{MacOS: "15.1", Projects: []ProjectVersions{},
				Errors: []ReadError{{ID: "RJ-349", Slug: "usage", Read: readHome}}},
		},
		{
			name: "a state directory that is a file",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				require.NoError(t, os.MkdirAll(f.state, 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(f.state, "romeu"), nil, 0o600))
			},
			calls: calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Projects: []ProjectVersions{},
				Errors: []ReadError{{ID: "RJ-348", Slug: "internal", Read: readState}}},
		},
		{
			name:    "an unreadable record and an unreadable render",
			records: map[string]*state.Project{"shop": nil},
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(f.root, "shop-env", ".romeu"), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(f.root, "shop-env", render.LiveRecord), []byte("{"), 0o600))
			},
			calls: calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Errors: []ReadError{},
				Projects: []ProjectVersions{{Name: "shop", Errors: []ReadError{
					{ID: state.RecordUnreadable.ID, Slug: state.RecordUnreadable.Slug, Read: readRecord},
					{ID: state.RecordUnreadable.ID, Slug: state.RecordUnreadable.Slug, Read: readRender},
				}}}},
		},
		{
			name: "a kit dir whose name does not begin its digest is a damaged render",
			setup: func(t *testing.T, f *fixture) {
				t.Helper()
				*f = newFixture(t, map[string]*state.Project{"shop": state.NewProject()})
				liveRender(t, f.root, func(r *render.Record) {
					r.Files = map[string]string{render.LiveKitsDir + "/julieta-ffffffffffff": julietaDigest}
				})
			},
			calls: calls{1, 1},
			want: Versions{Sbx: "0.49.2", Git: "2.53.0", MacOS: "15.1", Errors: []ReadError{},
				Projects: []ProjectVersions{{Name: "shop", Errors: []ReadError{
					{ID: state.RecordUnreadable.ID, Slug: state.RecordUnreadable.Slug, Read: readRender},
				}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, tt.records)
			if tt.setup != nil {
				tt.setup(t, &f)
			}
			var c calls
			got := f.read(t, f.seamsOf(&c, "0.49.2", "2.53.0", "15.1", tt.sbxErr, tt.gitErr, tt.macOSErr))
			assert.Equal(t, tt.calls, c)
			// A message that is the system's is held by its read and id alone.
			assert.Equal(t, ids(tt.want.Errors), ids(got.Errors))
			for i := range got.Projects {
				if i < len(tt.want.Projects) {
					assert.Equal(t, ids(tt.want.Projects[i].Errors), ids(got.Projects[i].Errors))
					got.Projects[i].Errors, tt.want.Projects[i].Errors = nil, nil
				}
			}
			if len(tt.want.Errors) > 0 && tt.want.Errors[0].Message != "" {
				assert.Equal(t, tt.want.Errors, got.Errors)
			}
			got.Errors, tt.want.Errors = nil, nil
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestReadVersionsEscapesAHostileRender: a render.json whose spec
// source kind holds code points that Go's %q keeps (a Hangul filler,
// the braille blank, a combining grapheme joiner) and 3000 more bytes
// reaches the document escaped (I29) and cut, as the record's check
// quotes the kind in its error.
func TestReadVersionsEscapesAHostileRender(t *testing.T) {
	t.Parallel()
	f := newFixture(t, map[string]*state.Project{"shop": state.NewProject()})
	liveRender(t, f.root, nil)
	path := filepath.Join(f.root, "shop-env", render.LiveRecord)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	hostile := "\u3164\u2800\u034f" + strings.Repeat("D", 3000)
	edited := strings.Replace(string(data), `"kind": "origin"`, `"kind": "`+hostile+`"`, 1)
	require.NotEqual(t, string(data), edited, "the fixture names its spec source kind")
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o600))
	var c calls
	got := f.read(t, f.seamsOf(&c, "0.49.2", "2.53.0", "15.1", nil, nil, nil))
	require.Len(t, got.Projects, 1)
	require.Len(t, got.Projects[0].Errors, 1)
	msg := got.Projects[0].Errors[0].Message
	for _, raw := range []string{"\u3164", "\u2800", "\u034f"} {
		assert.NotContains(t, msg, raw, "%q", raw)
	}
	assert.Contains(t, msg, `\u3164\u2800\u034f`, "each code point of I29 is shown as its escape")
	assert.True(t, strings.HasSuffix(msg, "..."), "the message is cut")
	assert.LessOrEqual(t, len(msg), 2*maxMessage)
}

// TestReadVersionsWritesNothing: the versions are read without a lock
// and leave the state directory and the tree as they were.
func TestReadVersionsWritesNothing(t *testing.T) {
	t.Parallel()
	f := newFixture(t, map[string]*state.Project{"shop": approved(t)})
	liveRender(t, f.root, nil)
	before := tree(t, filepath.Dir(f.root))
	var c calls
	f.read(t, f.seamsOf(&c, "0.49.2", "2.53.0", "15.1", nil, nil, nil))
	assert.Equal(t, before, tree(t, filepath.Dir(f.root)))
}

// tree returns the path, mode and size of every entry below dir.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	got := map[string]string{}
	require.NoError(t, filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		got[p] = info.Mode().String() + " " + info.ModTime().String()
		return nil
	}))
	return got
}

// TestKitPins: the kit dirs of render.json, and nothing else of its
// files, give the pins; a key under the kits dir that render would not
// write is refused, never guessed at.
func TestKitPins(t *testing.T) {
	t.Parallel()
	kits := render.LiveKitsDir + "/"
	tests := []struct {
		name  string
		files map[string]string
		want  []KitPin
	}{
		{name: "no kit", files: map[string]string{"sbxenv.yaml": julietaDigest}, want: []KitPin{}},
		{name: "a hyphen in the id", files: map[string]string{kits + "julieta-claude-" + julietaDigest[:12]: julietaDigest},
			want: []KitPin{{Kit: "julieta-claude", Digest: julietaDigest}}},
		{name: "in path order", files: map[string]string{kits + "b-" + personalDigest[:12]: personalDigest, kits + "a-" + julietaDigest[:12]: julietaDigest},
			want: []KitPin{{Kit: "a", Digest: julietaDigest}, {Kit: "b", Digest: personalDigest}}},
		{name: "a name that does not begin its digest", files: map[string]string{kits + "julieta-" + personalDigest[:12]: julietaDigest}},
		{name: "no digest in the name", files: map[string]string{kits + "julieta": julietaDigest}},
		{name: "an uppercase id", files: map[string]string{kits + "Julieta-" + julietaDigest[:12]: julietaDigest}},
		{name: "a path below a kit dir", files: map[string]string{kits + "julieta-" + julietaDigest[:12] + "/x": julietaDigest}},
		{name: "a digest that is not one", files: map[string]string{kits + "julieta-" + julietaDigest[:12]: julietaDigest[:12]}},
		{name: "an id of 64 bytes", files: map[string]string{kits + strings.Repeat("a", 64) + "-" + julietaDigest[:12]: julietaDigest}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := kitPins(tt.files)
			if tt.want == nil {
				require.ErrorIs(t, err, errKitDir)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseMacOS: the product version of macOS is major.minor or
// major.minor.patch, and any other value is the error internal, quoted
// escaped and cut.
func TestParseMacOS(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"15.1", "14.6.1", "26.0", "10.13.4"} {
		got, err := parseMacOS(ok)
		require.NoError(t, err, ok)
		assert.Equal(t, ok, got)
	}
	for _, bad := range []string{"", "15", "v15.1", "15.1.1.1", "15.01", "15.1-beta", "15.1+a", "15.1 (a)", "15.x", "15.1\n", "\x1b[2J15.1", strings.Repeat("1.", 40) + "1"} {
		got, err := parseMacOS(bad)
		require.Error(t, err, "%q", bad)
		assert.Empty(t, got)
		assert.Equal(t, "RJ-348", cli.Of(err).ID, "%q", bad)
		assert.NotContains(t, err.Error(), "\x1b", "%q", bad)
		assert.LessOrEqual(t, len(err.Error()), 200, "%q", bad)
	}
}

// TestMessage: a long message is cut on a code point's first byte, so
// the cut leaves no broken byte to escape as \x.
func TestMessage(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "short", message("short"))
	long := "a" + strings.Repeat("é", maxMessage)
	got := message(long)
	assert.NotContains(t, got, `\x`)
	assert.Equal(t, "a"+strings.Repeat("é", (maxMessage-1)/2)+"...", got)
}
