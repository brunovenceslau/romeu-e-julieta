// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package romeu

import (
	"bytes"
	"context"
	"encoding/pem"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// LogicalHost is the host of the origins' URLs, the one the lines of a
// journey show (https://github.com/<owner>/<config-repo>). The rules of
// 03 3.1 refuse the address and the port of the test server in a URL, so
// a test never reaches the real host: the git of the Env rewrites it to
// the server (installGit).
const LogicalHost = "github.com"

// Origins serves git repositories over TLS, the origins of 10 10.1:
// git http-backend (net/http/cgi) behind an httptest TLS server. They
// are the remote of a config repo and, for later tasks, of the repos of
// a project. The standard library does the serving; the repositories
// are made with the machine's git.
type Origins struct {
	srv  *httptest.Server
	root string // the directory of the bare repositories
	work string // the working copies Push commits in
	ca   string // the server's certificate as a CA bundle
	git  string // the machine's git, absolute
}

// NewOrigins starts the server and stops it with the test.
func NewOrigins(tb testing.TB) *Origins {
	tb.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		tb.Fatal(err)
	}
	if git, err = filepath.Abs(git); err != nil {
		tb.Fatal(err)
	}
	tmp := tb.TempDir()
	o := &Origins{git: git, root: filepath.Join(tmp, "bare"), work: filepath.Join(tmp, "work")}
	for _, d := range []string{o.root, o.work} {
		if err := os.Mkdir(d, 0o700); err != nil {
			tb.Fatal(err)
		}
	}
	out, err := exec.Command(git, "--exec-path").Output()
	if err != nil {
		tb.Fatal(err)
	}
	backend := &cgi.Handler{
		Path:       filepath.Join(strings.TrimSpace(string(out)), "git-http-backend"),
		Root:       "/",
		Env:        []string{"GIT_PROJECT_ROOT=" + o.root, "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"},
		InheritEnv: []string{"PATH"},
	}
	o.srv = httptest.NewTLSServer(backend)
	tb.Cleanup(o.srv.Close)
	o.ca = filepath.Join(tmp, "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: o.srv.Certificate().Raw})
	if err := os.WriteFile(o.ca, pemBytes, 0o600); err != nil {
		tb.Fatal(err)
	}
	return o
}

// URL returns the URL of the repository owner/repo as a project spec
// names it: https://github.com/owner/repo, valid by 03 3.1.
func (o *Origins) URL(ownerRepo string) string {
	return "https://" + LogicalHost + "/" + ownerRepo
}

// CAFile returns the path of the server's CA bundle.
func (o *Origins) CAFile() string { return o.ca }

// Push commits files (path to content) on the branch of the repository
// owner/repo, creating the repository, bare and served, on the first
// call, and pushes the commit to it. It is how a test makes a config
// repo, and how a later task replays an agent's commit on an origin.
func (o *Origins) Push(tb testing.TB, ownerRepo, branch string, files map[string]string) {
	tb.Helper()
	bare := filepath.Join(o.root, filepath.FromSlash(ownerRepo))
	work := filepath.Join(o.work, filepath.FromSlash(ownerRepo))
	if _, err := os.Stat(bare); err != nil {
		for _, d := range []string{bare, work} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				tb.Fatal(err)
			}
		}
		o.Git(tb, bare, "init", "-q", "--bare", "-b", "main")
		o.Git(tb, work, "init", "-q", "-b", branch)
		o.Git(tb, work, "remote", "add", "origin", bare)
	}
	for name, content := range files {
		p := filepath.Join(work, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			tb.Fatal(err)
		}
	}
	o.Git(tb, work, "add", "-A")
	o.Git(tb, work, "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "fixture")
	o.Git(tb, work, "push", "-q", "origin", "HEAD:refs/heads/"+branch)
}

// Git runs the machine's git in dir with the arguments, under an
// environment that reads no configuration and names a fixed author, and
// returns its output; a failure fails the test. It is the test's own
// git: romeu never runs through it.
func (o *Origins) Git(tb testing.TB, dir string, args ...string) string {
	tb.Helper()
	ctx, cancel := context.WithTimeout(tb.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, o.git, args...)
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=/usr/bin:/bin", "HOME=" + o.work, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
	}
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		tb.Fatalf("git %v in %s: %v\n%s", args, dir, err, stderr.String())
	}
	return out.String()
}

// installGit writes the git of the Env into bin and returns its path.
// With origins it is a script that runs the machine's git with two
// settings of its own before the caller's: the URLs of LogicalHost
// rewritten to the TLS server, and the server's CA trusted. Host
// settings name an absolute tools.git (03 3.4), so this is the one
// place a test reaches its origins without a gitsafe override (there is
// none) and without a port in a URL, which the rules of 03 3.1 refuse.
// Without origins it is a link to the machine's git.
func installGit(tb testing.TB, bin string, o *Origins) string {
	tb.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		tb.Fatal(err)
	}
	if git, err = filepath.Abs(git); err != nil {
		tb.Fatal(err)
	}
	path := filepath.Join(bin, "git")
	if o == nil {
		if err := os.Symlink(git, path); err != nil {
			tb.Fatal(err)
		}
		return path
	}
	for _, s := range []string{git, o.srv.URL, o.ca} {
		if strings.ContainsAny(s, "'\n") {
			tb.Fatalf("%q cannot be quoted in the git script", s)
		}
	}
	script := "#!/bin/sh\nexec '" + git + "' -c 'url." + o.srv.URL + "/.insteadOf=https://" + LogicalHost + "/' -c 'http.sslCAInfo=" + o.ca + "' \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		tb.Fatal(err)
	}
	return path
}
