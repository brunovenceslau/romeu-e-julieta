// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package spec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/catalog"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/gitsafe"
	"github.com/brunovenceslau/romeu-e-julieta/internal/mise"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	projectspec "github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// validLock is testdata/valid.lock: gh (aqua:cli/cli) and go (core:go),
// both in the embedded catalog, their hosts produced by it.
var validLock = func() []byte {
	b, err := os.ReadFile(filepath.Join("testdata", "valid.lock"))
	if err != nil {
		panic(err)
	}
	return b
}()

// acmeLock is a lock of one tool the catalog lacks, aqua:acme/tool,
// whose artifacts are on a host the catalog does not produce.
const acmeLock = `lockfile_version = 3

[[tools."aqua:acme/tool"]]
version = "1.0.0"
backend = "aqua:acme/tool"
specifiers = ["1.0.0"]

[tools."aqua:acme/tool"."platforms.linux-arm64"]
checksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
url = "https://dl.acme.example/tool_linux_arm64.tar.gz"

[tools."aqua:acme/tool"."platforms.linux-x64"]
checksum = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
url = "https://dl.acme.example/tool_linux_amd64.tar.gz"
`

// fakeOrigins is a lockFetcher over a map of URL to lock; a URL it lacks
// has no lock, and the URL "fail" fails the fetch.
type fakeOrigins struct {
	locks map[string]string
	asked []string
}

func (f *fakeOrigins) fetch(_ context.Context, url, ref string) ([]byte, bool, error) {
	f.asked = append(f.asked, url+"@"+ref)
	if strings.HasSuffix(url, "/fail") {
		return nil, false, errors.New("the origin did not answer")
	}
	l, ok := f.locks[url]
	return []byte(l), ok, nil
}

// validate runs the command over the files with the fetcher of origins
// and returns the exit status, stdout and stderr.
func validate(t *testing.T, origins *fakeOrigins, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&stdout), Stderr: cli.NewWriter(&stderr)}
	err := run(t.Context(), env, args, origins.fetch)
	return cli.Report(env, err), stdout.String(), stderr.String()
}

// decodeShop decodes the spec body.
func decodeShop(t *testing.T, body string) projectspec.Project {
	t.Helper()
	p, err := projectspec.DecodeProject([]byte(body))
	require.NoError(t, err)
	return p
}

// embedded returns the embedded catalog.
func embedded(t *testing.T) catalog.Catalog {
	t.Helper()
	cat, err := catalog.Embedded()
	require.NoError(t, err)
	return cat
}

// TestSpecValidateCatalogKnownTools: with --catalog, the lock of each
// repo is read at its ref, the primary's at main and the secondary's at
// origin's default branch, and a lock whose tools the catalog has, on
// hosts it produces, passes with no note (REQ-0422, REQ-0639).
func TestSpecValidateCatalogKnownTools(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-api": string(validLock)}}

	code, stdout, stderr := validate(t, o, "projects/shop.yaml")

	require.Equal(t, 0, code, stderr)
	assert.Equal(t, notCheckedLine+"spec validate 1 files 0 findings\n", stdout)
	assert.Equal(t, []string{"https://github.com/example/shop-api@main", "https://github.com/example/shop-web@"}, o.asked)
}

// TestSpecValidateCatalogUnknownTool: a backend:tool neither the catalog
// nor egress.tools of the spec has is a finding, exit 6, the case sync
// refuses with catalog-unknown-tool (07 7.5, step 2); the host of its
// artifacts is the warning lock-host-gated, on stderr (step 3).
func TestSpecValidateCatalogUnknownTool(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-web": acmeLock}}

	code, stdout, stderr := validate(t, o, "projects/shop.yaml")

	assert.Equal(t, 6, code, stderr)
	assert.Contains(t, stderr, "RJ-328")
	assert.Contains(t, stdout, "finding catalog-unknown-tool projects/shop.yaml: repos[1]: aqua:acme/tool in the mise.lock of https://github.com/example/shop-web ")
	assert.Contains(t, stderr, "warning lock-host-gated: projects/shop.yaml: dl.acme.example is an artifact host ")
	assert.NotContains(t, stdout, "dl.acme.example")
	assert.Contains(t, stdout, "spec validate 1 files 1 findings\n")
}

// TestSpecValidateCatalogSpecTool: a tool egress.tools of the spec
// declares is no finding, and a lock host the catalog does not produce
// is the warning lock-host-gated (04 4.1, Warnings), which changes no
// exit code.
func TestSpecValidateCatalogSpecTool(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec + "  tools:\n    \"aqua:acme/tool\": [dl.acme.example]\n"})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-web": acmeLock}}

	code, stdout, stderr := validate(t, o, "projects/shop.yaml")

	require.Equal(t, 0, code, stderr)
	assert.Equal(t, notCheckedLine+"spec validate 1 files 0 findings\n", stdout)
	assert.Equal(t, "warning lock-host-gated: projects/shop.yaml: dl.acme.example is an artifact host of a mise.lock "+
		"that the embedded egress catalog does not produce; romeu sync gates it\n", stderr)
}

// TestSpecValidateCatalogUnparsedLock: a lock that does not parse with
// the pinned layout, or one gitsafe refuses to read, is the finding
// lock-unparsed, exit 6, so no unknown tool hides in it.
func TestSpecValidateCatalogUnparsedLock(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-api": "lockfile_version = 2\n"}}

	code, stdout, stderr := validate(t, o, "projects/shop.yaml")

	assert.Equal(t, 6, code, stderr)
	assert.Contains(t, stdout, "finding lock-unparsed projects/shop.yaml: repos[0]: the mise.lock of https://github.com/example/shop-api does not parse: ")

	refused := func(context.Context, string, string) ([]byte, bool, error) {
		return nil, false, fmt.Errorf("%w: %w: a link", gitsafe.ErrRefused, gitsafe.ErrFileRefused)
	}
	var out, errOut bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&errOut)}
	assert.Equal(t, 6, cli.Report(env, run(t.Context(), env, []string{"projects/shop.yaml"}, refused)))
	assert.Contains(t, out.String(), "finding lock-unparsed projects/shop.yaml: repos[0]: the mise.lock of https://github.com/example/shop-api at main is no regular file within the cap of 1048576 bytes: ")
}

// TestSpecValidateCatalogFetchFails: an origin that cannot be read stops
// the command with fetch-failed (RJ-360), exit 1, never a pass, and the
// details name the URL and the ref.
func TestSpecValidateCatalogFetchFails(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": strings.Replace(shopSpec, "shop-web", "fail", 1)})
	o := &fakeOrigins{}

	code, stdout, stderr := validate(t, o, "projects/shop.yaml")

	assert.Equal(t, 1, code, stderr)
	assert.Equal(t, "RJ-360", errorID(t, stderr), stderr)
	assert.Contains(t, stderr, "fix: decide: check the network and the origin URL, then run the command again\n")
	assert.Contains(t, stderr, "url: https://github.com/example/fail\n")
	assert.Contains(t, stderr, "could not read the mise.lock of https://github.com/example/fail at its default branch from origin")
	assert.Empty(t, stdout)
}

// TestSpecValidateCatalogSkipsWhatDoesNotDecode: --catalog reads the
// origins of the specs that decode only, and a finding of one spec does
// not stop the catalog check of another.
func TestSpecValidateCatalogSkipsWhatDoesNotDecode(t *testing.T) {
	blog := strings.NewReplacer("name: shop", "name: blog", "shop-api", "blog-api", "shop-web", "blog-web").Replace(shopSpec)
	configRepo(t, map[string]string{"shop.yaml": "name: [", "blog.yaml": blog})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/blog-web": acmeLock}}

	code, stdout, _ := validate(t, o, "projects/shop.yaml", "projects/blog.yaml")

	assert.Equal(t, 6, code)
	assert.Contains(t, stdout, "finding decode projects/shop.yaml: ")
	assert.Contains(t, stdout, "finding catalog-unknown-tool projects/blog.yaml: repos[1]: aqua:acme/tool ")
	assert.Equal(t, []string{"https://github.com/example/blog-api@main", "https://github.com/example/blog-web@"}, o.asked)
}

// TestSpecValidateCatalogEgressCounts: the domains --catalog finds enter
// the manifest whose size is measured.
func TestSpecValidateCatalogEgressCounts(t *testing.T) {
	p := decodeShop(t, shopSpec)
	cat := embedded(t)
	eg := baseEgress(cat, p)
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-web": acmeLock, "https://github.com/example/shop-api": string(validLock)}}

	_, _, err := checkCatalog(t.Context(), o.fetch, cat, "projects/shop.yaml", p, eg)

	require.NoError(t, err)
	assert.True(t, eg["dl.acme.example"], "a lock host")
	assert.True(t, eg["proxy.golang.org"], "a domain of core:go in the catalog")
}

// TestSpecValidateCatalogReadsOrigin runs the fetcher of julieta, a
// shallow fetch through gitsafe, against an origin served over git://
// on the loopback, and the check over what it reads: the lock of the
// branch named, or of the default branch, is read from the commit of
// origin; a branch without one has none; a lock that is a symbolic
// link, or one over the cap of mise, is the finding lock-unparsed, not
// a failure of the network; a branch origin lacks is fetch-failed
// (RJ-360). No temporary directory is left.
func TestSpecValidateCatalogReadsOrigin(t *testing.T) {
	base := t.TempDir()
	work := t.TempDir()
	git(t, work, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(work, "mise.lock"), validLock, 0o600))
	git(t, work, "add", "mise.lock")
	git(t, work, "commit", "-q", "-m", "lock")
	git(t, work, "checkout", "-q", "-b", "bare")
	git(t, work, "rm", "-q", "mise.lock")
	git(t, work, "commit", "-q", "-m", "no lock")
	git(t, work, "checkout", "-q", "-b", "link", "main")
	git(t, work, "rm", "-q", "mise.lock")
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(work, "mise.lock")))
	git(t, work, "add", "mise.lock")
	git(t, work, "commit", "-q", "-m", "a link")
	git(t, work, "checkout", "-q", "-b", "big", "main")
	require.NoError(t, os.WriteFile(filepath.Join(work, "mise.lock"), bytes.Repeat([]byte("#"), mise.MaxFileSize+1), 0o600))
	git(t, work, "add", "mise.lock")
	git(t, work, "commit", "-q", "-m", "big")
	git(t, work, "checkout", "-q", "main")
	git(t, base, "clone", "-q", "--bare", work, "api.git")
	url := serveGit(t, base) + "api.git"
	g, err := gitsafe.New(gitsafe.Config{Mode: gitsafe.Sandbox, Git: gitPath(t), Home: t.TempDir()})
	require.NoError(t, err)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	fetch := gitFetcher(g)
	cat := embedded(t)
	check := func(ref string) ([]finding, error) {
		p := projectspec.Project{Name: "shop", Repos: []projectspec.Repo{{URL: url, Ref: ref, Dir: "api", Primary: true}}}
		found, _, err := checkCatalog(t.Context(), fetch, cat, "projects/shop.yaml", p, egressSet{})
		return found, err
	}

	for _, ref := range []string{"", "main"} {
		data, found, err := fetch(t.Context(), url, ref)
		require.NoError(t, err, ref)
		assert.True(t, found, ref)
		assert.Equal(t, validLock, data, ref)
		got, err := check(ref)
		require.NoError(t, err, ref)
		assert.Empty(t, got, ref)
	}
	_, found, err := fetch(t.Context(), url, "bare")
	require.NoError(t, err)
	assert.False(t, found)
	for _, ref := range []string{"link", "big"} {
		got, err := check(ref)
		require.NoError(t, err, ref)
		require.Len(t, got, 1, ref)
		assert.Equal(t, "lock-unparsed", got[0].rule, ref)
		assert.Equal(t, "repos[0]", got[0].path, ref)
	}
	_, err = check("gone")
	assert.Equal(t, "RJ-360", cli.Of(err).ID, "a branch origin lacks: %v", err)

	left, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, left, "no julieta-spec-* directory is left")
}

// TestSpecValidateCatalogHostlessURL: an artifact URL of a lock with no
// host julieta can read is the warning lock-host-gated, not dropped.
func TestSpecValidateCatalogHostlessURL(t *testing.T) {
	lock := strings.Replace(acmeLock, "https://dl.acme.example/tool_linux_arm64.tar.gz", "https:///tool_linux_arm64.tar.gz", 1)
	configRepo(t, map[string]string{"shop.yaml": shopSpec + "  tools:\n    \"aqua:acme/tool\": [dl.acme.example]\n"})
	o := &fakeOrigins{locks: map[string]string{"https://github.com/example/shop-web": lock}}

	code, _, stderr := validate(t, o, "projects/shop.yaml")

	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "warning lock-host-gated: projects/shop.yaml: the mise.lock of https://github.com/example/shop-web records an artifact URL "+
		"for aqua:acme/tool, platform linux-arm64, with no host julieta can read; ")
	assert.Contains(t, stderr, "warning lock-host-gated: projects/shop.yaml: dl.acme.example is an artifact host ")
}

// gitPath is the git of the tests, the first one julieta looks at.
func gitPath(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"/usr/bin/git", "/bin/git"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	require.FailNow(t, "no git at /usr/bin/git or /bin/git")
	return ""
}

// fixtureEnv is the environment of the plain git that builds fixtures:
// no system or global config, and a fixed identity and date.
func fixtureEnv(home string) []string {
	return []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "PATH=/usr/bin:/bin", "HOME=" + home, "LANG=C",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	}
}

// git runs git without julieta, to build fixtures.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	var stderr bytes.Buffer
	argv := append([]string{gitPath(t), "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)
	err := procgroup.Run(t.Context(), procgroup.Command{Class: procgroup.JulietaGit, Argv: argv, Dir: dir,
		Env: fixtureEnv(t.TempDir()), Stderr: &stderr})
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), stderr.String())
}

// serveGit serves the repositories below base over git:// on
// 127.0.0.1, one git daemon --inetd per connection, and returns the
// URL prefix of base, ending in a slash.
func serveGit(t *testing.T, base string) string {
	t.Helper()
	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	argv := []string{gitPath(t), "daemon", "--inetd", "--export-all", "--base-path=" + base}
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = conn.Close() }()
				// A failed session shows as a failed fetch in the test.
				_ = procgroup.Run(ctx, procgroup.Command{Class: procgroup.JulietaGit, Argv: argv,
					Env: fixtureEnv(base), Stdin: conn, Stdout: conn})
			})
		}
	})
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		wg.Wait()
	})
	return "git://" + ln.Addr().String() + "/"
}

// TestSpecValidateCatalogDeadline: a repo whose read runs past its
// deadline stops the command with fetch-failed (RJ-360), never a pass.
func TestSpecValidateCatalogDeadline(t *testing.T) {
	configRepo(t, map[string]string{"shop.yaml": shopSpec})
	saved := repoTimeout
	repoTimeout = 10 * time.Millisecond
	t.Cleanup(func() { repoTimeout = saved })
	slow := func(ctx context.Context, _, _ string) ([]byte, bool, error) {
		<-ctx.Done()
		return nil, false, ctx.Err()
	}
	var out, errOut bytes.Buffer
	env := &cli.Env{Binary: "julieta", Stdout: cli.NewWriter(&out), Stderr: cli.NewWriter(&errOut)}

	err := run(t.Context(), env, []string{"projects/shop.yaml"}, slow)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, "RJ-360", cli.Of(err).ID)
	assert.Empty(t, out.String())
}
