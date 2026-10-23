// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

// Package gitsafe is the one runner of git in both binaries (05 5.1):
// romeu runs every git subprocess in host mode, julieta every one in
// sandbox mode. Each call gets a fresh environment and the flags of
// its mode, and runs through internal/procgroup under the git class of
// the timeout table (04 4.1, Starting values).
//
// The package exports verbs, not a general runner: a fetch takes an
// explicit URL that CheckURL accepts and refspecs whose destinations
// are allowlisted for the mode (I5); a clone takes such a URL and a
// new directory; CreateRef is create-only (I6);
// BundleCreate writes a bundle of named refs, and BundleOf a bundle of
// name and object id pairs from a temporary repository, so no ref is
// written into the user's; nothing builds a push (I18). Unbundle,
// verify, fsck, the tree walk, the history reads, the reads of a
// working tree, the list of refs, the reflog, the global and system
// hooks paths, the reachability of a set of commits and the common git
// dir are the verbs of the files unbundle*.go, verify*.go, fsck*.go,
// walk*.go, history*.go, status*.go, refs_list*.go, reflog*.go,
// global*.go, reach*.go and commondir*.go; BundleOf is in bundle_of.go.
package gitsafe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Mode is how gitsafe hardens a git call: Host for every git
// subprocess romeu starts, Sandbox for every git call julieta makes.
// The zero value names no mode, and New refuses it.
type Mode int

// The two modes of 05 5.1.
const (
	modeNone Mode = iota
	Host          // romeu's git: fresh environment, no global or system config, the full flag list
	Sandbox       // julieta's git: fresh environment with the network pass-through, the sandbox's own global config, three flags
)

// String returns the name of the mode.
func (m Mode) String() string {
	switch m {
	case Host:
		return "host"
	case Sandbox:
		return "sandbox"
	}
	return "mode-" + strconv.Itoa(int(m))
}

// setting is one -c flag: a config key and the value gitsafe gives it.
type setting struct {
	key, value string
}

// hostSettings is the flag list of host mode, in the order of 05 5.1.
// It is the one list of the keys gitsafe neutralizes: NeutralizedKeys
// reads it, and so do doctor's host-git-config check and the I3 plant
// list, through NeutralizedKeys.
var hostSettings = []setting{
	{"core.hooksPath", "/dev/null"},
	{"core.fsmonitor", "false"},
	{"submodule.recurse", "false"},
	{"protocol.allow", "never"},
	{"protocol.https.allow", "always"},
	{"protocol.git.allow", "always"},
	{"core.sshCommand", "false"},
	{"core.gitProxy", ""},
	{"core.askPass", ""},
	{"core.alternateRefsCommand", ""},
	{"diff.external", ""},
	{"core.pager", "cat"},
	{"credential.helper", ""},
	{"fetch.prune", "false"},
	{"transfer.fsckObjects", "true"},
	{"transfer.bundleURI", "false"},
	{"fetch.bundleURI", ""},
	{"gc.auto", "0"},
	{"maintenance.auto", "false"},
	{"core.symlinks", "false"},
	{"advice.detachedHead", "false"},
}

// sandboxSettings is the flag list of sandbox mode (05 5.1). The rest
// of julieta's git config is the sandbox's own.
var sandboxSettings = []setting{
	{"core.hooksPath", "/dev/null"},
	{"core.fsmonitor", "false"},
	{"submodule.recurse", "false"},
	{"fetch.bundleURI", ""},
}

// networkEnv is the network pass-through list of sandbox mode (05
// 5.1): the variables julieta copies from its own environment when they
// are set and not empty, so its git reaches origin through the
// sandbox's proxy and, with a TLS library that reads SSL_CERT_FILE,
// trusts the CA bundle it names. It holds no GIT_* variable, and host mode
// passes none of it.
var networkEnv = []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy", "SSL_CERT_FILE"}

// NetworkEnv returns the network pass-through list of sandbox mode, in
// the order the variables reach git. Each call returns a new slice.
func NetworkEnv() []string {
	return slices.Clone(networkEnv)
}

// NeutralizedKeys returns the git config keys host mode sets on every
// call, in the order of 05 5.1. Each call returns a new slice.
func NeutralizedKeys() []string {
	keys := make([]string, 0, len(hostSettings))
	for _, s := range hostSettings {
		keys = append(keys, s.key)
	}
	return keys
}

// GitHost is one gitHosts entry of host settings (03 3.4).
type GitHost struct {
	// Host is the host of the repository URLs, with a port when the
	// URLs carry one.
	Host string
	// CAFile is the absolute path of a CA bundle that git trusts for
	// this host only; empty means the system's.
	CAFile string
}

// Config is what a Runner needs from host settings or from julieta.
type Config struct {
	Mode Mode
	// Git is the absolute path of git: tools.git of host settings, or
	// julieta's own lookup in the sandbox.
	Git string
	// Home is the HOME git runs with: the operator's on the host, the
	// workload user's in the sandbox.
	Home string
	// CredentialHelper is tools.gitCredentialHelper: a helper name or
	// an absolute path (CheckCredentialHelper), the one helper host mode
	// passes, and only for CredentialHost. Host mode only.
	CredentialHelper string
	// CredentialHost is the host of config.url, with its port when the
	// URL has one: the one host CredentialHelper answers, as
	// credential.https://<host>.helper (05 5.1, REQ-0464). It is set
	// exactly when CredentialHelper is.
	CredentialHost string
	// GitHosts are the gitHosts of host settings. Host mode only.
	GitHosts []GitHost
}

// Runner runs git hardened for its mode. Its methods are safe for
// concurrent use.
type Runner struct {
	mode     Mode
	git      string
	env      []string
	settings []setting
	hosts    []GitHost
	class    procgroup.Class
	// start runs a command; tests replace it to record what would run.
	start func(context.Context, procgroup.Command) error
}

// ErrConfig is the error of a Config that gitsafe cannot run hardened.
var ErrConfig = errors.New("gitsafe: invalid config")

// ErrRefused is the error of a URL, ref, refspec, object id or path
// that gitsafe refuses before git runs.
var ErrRefused = errors.New("gitsafe: refused")

// hostPattern is a host name or IPv4 address with an optional port.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]{1,5})?$`)

// noLazyFetch stops git from fetching a missing object from a promisor
// remote the repository's own config names, which would bypass CheckURL
// and the gitHosts allowlist. Git reads it from 2.45.0, below the floor.
const noLazyFetch = "GIT_NO_LAZY_FETCH=1"

// New returns a runner for the config, or ErrConfig.
func New(c Config) (*Runner, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	r := &Runner{mode: c.Mode, git: c.Git, start: procgroup.Run}
	switch c.Mode {
	case Host:
		r.class = procgroup.HostGitFetch
		r.env = []string{
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_TERMINAL_PROMPT=0",
			"GIT_ATTR_NOSYSTEM=1",
			noLazyFetch,
			"PATH=/usr/bin:/bin",
			"HOME=" + c.Home,
			"LANG=C",
		}
		r.settings = slices.Clone(hostSettings)
		if c.CredentialHelper != "" {
			// After the empty value, which resets the multi-valued key:
			// this helper and no other, for the config repo's host only
			// (03 3.4, REQ-0464).
			i := slices.IndexFunc(r.settings, func(s setting) bool { return s.key == "credential.helper" })
			r.settings = slices.Insert(r.settings, i+1, helperFor(c.CredentialHost, c.CredentialHelper))
		}
		r.hosts = slices.Clone(c.GitHosts)
	case Sandbox:
		r.class = procgroup.JulietaGit
		r.env = []string{
			"GIT_TERMINAL_PROMPT=0",
			noLazyFetch,
			"PATH=/usr/bin:/bin",
			"HOME=" + c.Home,
			"LANG=C",
		}
		for _, k := range networkEnv {
			if v := os.Getenv(k); v != "" {
				r.env = append(r.env, k+"="+v)
			}
		}
		r.settings = slices.Clone(sandboxSettings)
	}
	return r, nil
}

// check returns ErrConfig, wrapped with the reason, for a config New
// refuses.
func (c Config) check() error {
	switch {
	case c.Mode != Host && c.Mode != Sandbox:
		return fmt.Errorf("%w: mode %s", ErrConfig, c.Mode)
	case !filepath.IsAbs(c.Git):
		return fmt.Errorf("%w: git %q is not an absolute path", ErrConfig, c.Git)
	case !filepath.IsAbs(c.Home):
		return fmt.Errorf("%w: home %q is not an absolute path", ErrConfig, c.Home)
	case c.Mode == Sandbox && (c.CredentialHelper != "" || len(c.GitHosts) > 0):
		return fmt.Errorf("%w: sandbox mode takes no host credentials", ErrConfig)
	case c.CredentialHelper != "" && CheckCredentialHelperFile(c.CredentialHelper) != nil:
		return fmt.Errorf("%w: %w", ErrConfig, CheckCredentialHelperFile(c.CredentialHelper))
	case (c.CredentialHelper != "") != (c.CredentialHost != ""):
		return fmt.Errorf("%w: a credential helper and its host go together", ErrConfig)
	case c.CredentialHost != "" && !hostPattern.MatchString(c.CredentialHost):
		return fmt.Errorf("%w: credential host %q", ErrConfig, c.CredentialHost)
	}
	for _, h := range c.GitHosts {
		if !hostPattern.MatchString(h.Host) {
			return fmt.Errorf("%w: git host %q", ErrConfig, h.Host)
		}
		if h.CAFile != "" && (!filepath.IsAbs(h.CAFile) || strings.ContainsFunc(h.CAFile, isControl)) {
			return fmt.Errorf("%w: caFile %q of %s is not an absolute path", ErrConfig, h.CAFile, h.Host)
		}
	}
	return nil
}

// checkRepo returns ErrRefused for a dir that is not the root of a
// repository: an absolute path that holds .git, or a bare repository's
// HEAD and objects. git given any other directory walks up to the
// repository that encloses it and acts there.
func checkRepo(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("%w: repository %q is not an absolute path", ErrRefused, dir)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	head, headErr := os.Lstat(filepath.Join(dir, "HEAD"))
	objects, objErr := os.Lstat(filepath.Join(dir, "objects"))
	if headErr == nil && objErr == nil && head.Mode().IsRegular() && objects.IsDir() {
		return nil
	}
	return fmt.Errorf("%w: %q is not the root of a repository", ErrRefused, dir)
}

// isControl reports whether r is an ASCII control character.
func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}

// refusedSubcommands are the git subcommands the runner never starts:
// romeu pushes no salvage commit, and julieta signs and pushes none
// (I18).
var refusedSubcommands = []string{"push", "send-pack"}

// run runs git in dir with the flags of the mode, the extra flags
// before the subcommand, and args, which start with the subcommand.
// stdout receives the standard output; nil discards it. The error of a
// git that fails carries the start of its standard error, escaped with
// termsafe.
//
//romeu:invariant I3 guard
//romeu:invariant I18 guard
func (r *Runner) run(ctx context.Context, dir string, stdout io.Writer, args ...string) error {
	return r.runWith(ctx, dir, nil, stdout, args...)
}

// runWith is run with extra settings after those of the mode.
func (r *Runner) runWith(ctx context.Context, dir string, extra []setting, stdout io.Writer, args ...string) error {
	return r.runInput(ctx, dir, extra, nil, nil, stdout, args...)
}

// runInput is runWith with env appended to the environment of the mode
// and stdin as git's standard input; nil means an empty one.
func (r *Runner) runInput(ctx context.Context, dir string, extra []setting, env []string, stdin io.Reader, stdout io.Writer, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: no git subcommand", ErrRefused)
	}
	if slices.Contains(refusedSubcommands, args[0]) {
		return fmt.Errorf("%w: git subcommand %q", ErrRefused, args[0])
	}
	argv := make([]string, 0, 1+2*(len(r.settings)+len(extra))+len(args))
	argv = append(argv, r.git)
	for _, s := range slices.Concat(r.settings, extra) {
		argv = append(argv, "-c", s.key+"="+s.value)
	}
	argv = append(argv, args...)
	stderr := limitedBuffer{limit: stderrLimit}
	err := r.start(ctx, procgroup.Command{
		Class:  r.class,
		Argv:   argv,
		Dir:    dir,
		Env:    slices.Concat(r.env, env),
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: &stderr,
	})
	if err != nil {
		// A server writes part of git's standard error (remote: lines),
		// so it reaches the error only escaped (I29).
		msg := termsafe.Escape(strings.TrimSpace(stderr.String()))
		if msg == "" {
			return fmt.Errorf("gitsafe: git %s: %w", args[0], err)
		}
		return fmt.Errorf("gitsafe: git %s: %w: %s", args[0], err, msg)
	}
	return nil
}

// output runs git as run does and returns its standard output, of at
// most outputLimit bytes.
func (r *Runner) output(ctx context.Context, dir string, extra []setting, args ...string) (string, error) {
	out := limitedBuffer{limit: outputLimit}
	if err := r.runWith(ctx, dir, extra, &out, args...); err != nil {
		return "", err
	}
	if out.truncated {
		return "", fmt.Errorf("gitsafe: git %s: output longer than %d bytes", args[0], outputLimit)
	}
	return out.String(), nil
}

// The bounds of what gitsafe keeps of git's output: the start of the
// standard error for the error message, and the whole standard output
// of the calls that read it, which are one-line answers.
const (
	stderrLimit = 4 << 10
	outputLimit = 64 << 10
)

// limitedBuffer keeps the first limit bytes written to it and drops
// the rest, without failing the writer, so git never blocks on a full
// pipe.
type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// Write keeps what fits under the limit and reports all of p written.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.buf.Len()
	if len(p) > room {
		b.truncated = true
		b.buf.Write(p[:max(room, 0)])
		return len(p), nil
	}
	b.buf.Write(p)
	return len(p), nil
}

// String returns what was kept.
func (b *limitedBuffer) String() string {
	return b.buf.String()
}
