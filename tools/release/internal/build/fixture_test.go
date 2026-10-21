// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// commitTime is the committer date of the first commit of a fixture:
// far from now, so a build date taken from the clock cannot match it.
var commitTime = time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)

// markerJulieta is a string only the fixture julieta holds, which the
// fixture romeu must carry after the build embeds it.
const markerJulieta = "julieta-fixture-marker-7f3a91"

// write writes the files, name to content, under dir.
func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

// commit stages everything under dir and commits it, unsigned, with the
// committer date at when. The repository is a throwaway fixture; the
// signing rule is of the real repository.
func commit(t *testing.T, dir, message string, when time.Time) string {
	t.Helper()
	ctx := t.Context()
	date := when.Format(time.RFC3339)
	env := append(gitEnv(), "GIT_COMMITTER_DATE="+date, "GIT_AUTHOR_DATE="+date,
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	for _, args := range [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-q", "--no-verify", "--allow-empty", "-m", message},
	} {
		_, err := output(ctx, procgroup.ToolGit, dir, env, args)
		require.NoError(t, err)
	}
	out, err := git(ctx, dir, "rev-parse", "HEAD")
	require.NoError(t, err)
	return string(out[:len(out)-1])
}

// initRepo makes a repository on main under a new directory.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := output(t.Context(), procgroup.ToolGit, dir, append(gitEnv(), "GIT_CONFIG_GLOBAL=/dev/null"),
		[]string{"git", "init", "-q", "-b", "main"})
	require.NoError(t, err)
	return dir
}

// fixtureModule is a module with the two commands of the repository, one
// dependency with a license by a replace directive, and the files a
// build reads. The fixture romeu embeds internal/embedded/julieta, which
// no file of the tree holds: only the overlay of a build does.
func fixtureModule(t *testing.T) string {
	t.Helper()
	return fixtureModuleWith(t, true)
}

// fixtureModuleWith is fixtureModule with or without the package that
// consumes the embedded julieta: without it, nothing reads
// internal/embedded/julieta, as in the repository today.
func fixtureModuleWith(t *testing.T, consumer bool) string {
	t.Helper()
	dep := t.TempDir()
	write(t, dep, map[string]string{
		"go.mod":  "module example.com/dep\n\ngo 1.27\n",
		"dep.go":  "package dep\n\n// Name is the dependency's name.\nconst Name = \"dep\"\n",
		"LICENSE": "The dep license: use it freely.\n",
	})
	root := initRepo(t)
	files := map[string]string{
		"go.mod":              "module example.com/fixture\n\ngo 1.27\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => " + dep + "\n",
		"VERSION":             "1.0.0\n",
		"COPYING":             "The fixture's own license.\n",
		"cmd/julieta/main.go": "package main\n\nimport _ \"example.com/dep\"\n\nvar version = \"devel\"\n\nfunc main() { println(version, \"" + markerJulieta + "\") }\n",
		"cmd/romeu/main.go": "package main\n\nimport (\n\t\"example.com/dep\"\n\t\"example.com/fixture/internal/embedded\"\n)\n\n" +
			"var version, commit, date = \"devel\", \"unknown\", \"unknown\"\n\n" +
			"func main() { println(dep.Name, version, commit, date, len(embedded.Julieta)) }\n",
		"internal/embedded/embedded.go": "// Package embedded holds the julieta binary.\npackage embedded\n\nimport _ \"embed\"\n\n// Julieta is the embedded binary.\n//\n//go:embed julieta\nvar Julieta []byte\n",
	}
	if !consumer {
		delete(files, "internal/embedded/embedded.go")
		files["cmd/romeu/main.go"] = "package main\n\nimport \"example.com/dep\"\n\nvar version, commit, date = \"devel\", \"unknown\", \"unknown\"\n\nfunc main() { println(dep.Name, version, commit, date) }\n"
	}
	write(t, root, files)
	commit(t, root, "fixture", commitTime)
	return root
}

// members reads an archive back: the headers in order and the content of
// each file.
func members(t *testing.T, data []byte) ([]*tar.Header, map[string][]byte) {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	var headers []*tar.Header
	content := map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		headers = append(headers, h)
		content[h.Name] = body
	}
	return headers, content
}

// tag makes the lightweight tag at HEAD.
func tag(t *testing.T, root, name string) {
	t.Helper()
	_, err := git(t.Context(), root, "-c", "tag.gpgsign=false", "tag", name)
	require.NoError(t, err)
}

// built is a build read into memory: what a test of the archives reads,
// kept after the directory the build wrote them to is gone.
type built struct {
	res      Result
	sums     []byte
	archives map[string][]byte // by file name
}

// readBuilt reads the files of res.
func readBuilt(res Result) (built, error) {
	b := built{res: res, archives: map[string][]byte{}}
	var err error
	if b.sums, err = os.ReadFile(res.Checksums); err != nil {
		return built{}, err
	}
	for _, path := range res.Archives {
		data, err := os.ReadFile(path)
		if err != nil {
			return built{}, err
		}
		b.archives[filepath.Base(path)] = data
	}
	return b, nil
}

// sharedBuilds are the two builds of one fixture commit that the tests
// of the archives read. The gzip of an archive is the slow part of a
// build under the race detector (some 4 s for each of the two archives of
// a build), so the package pays it for these two builds, not for each
// test that reads an archive.
//
// The paths in res.Archives and res.Checksums of each result point into
// the temporary directory of the first test, removed when it ends, so
// readers use only the in-memory bytes and the base names.
type sharedBuilds struct {
	head    string // the commit both builds are of
	dry     built  // a dry run, with a gate that must stay unasked
	release built  // a release build behind a gate that passes
	asked   int32  // the requests the gate of the dry run got
}

var (
	sharedOnce  sync.Once
	sharedValue sharedBuilds
	// errShared says that the first test to ask did not finish the
	// builds: a failed require ends that test inside the Once, and the
	// others must fail too, not read an empty value.
	errShared = errors.New("the shared builds did not finish in an earlier test")
)

// shared returns the two shared builds, made by the first test to ask.
// The builds run in that test: its context and its temporary files.
func shared(t *testing.T) sharedBuilds {
	t.Helper()
	sharedOnce.Do(func() { sharedValue, errShared = buildShared(t) })
	require.NoError(t, errShared)
	return sharedValue
}

// buildShared builds the fixture at v1.0.0 twice at once: as a dry run,
// and as a release behind a gate that passes. The dry run's gate counts
// the requests it gets, and answers none of them with success.
func buildShared(t *testing.T) (sharedBuilds, error) {
	t.Helper()
	root := fixtureModule(t)
	tag(t, root, "v1.0.0")
	head, err := git(t.Context(), root, "rev-parse", "HEAD")
	require.NoError(t, err)
	goBin := goBinOf(t)

	var asked atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked.Add(1)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	unasked := &FuzzGate{API: server.URL, Repo: "owner/romeu-e-julieta", Token: "token",
		Client: server.Client(), AllowHost: strings.TrimPrefix(server.URL, "https://")}

	opts := [2]Options{
		{Root: root, Version: "v1.0.0", Out: t.TempDir(), DryRun: true, GoBin: goBin, Gate: unasked},
		{Root: root, Version: "v1.0.0", Out: filepath.Join(t.TempDir(), "out"), GoBin: goBin, Gate: fuzzedGate(t, root)},
	}
	// The two builds run at once from one Root, which is safe because
	// Build never writes into Root: its staging directory comes from
	// os.MkdirTemp (build.go, Build) and its output from publish, both
	// outside Root.
	var results [2]built
	var errs [2]error
	var wg sync.WaitGroup
	for i := range opts {
		wg.Go(func() {
			res, err := Build(t.Context(), opts[i])
			if err == nil {
				results[i], err = readBuilt(res)
			}
			errs[i] = err
		})
	}
	wg.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		return sharedBuilds{}, err
	}
	return sharedBuilds{head: strings.TrimSpace(string(head)), dry: results[0], release: results[1], asked: asked.Load()}, nil
}
