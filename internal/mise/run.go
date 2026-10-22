// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// passThrough are the variables of this process a mise run keeps, each
// only when set: where things are (the home, the search path, the
// temporary dir, the dirs go keeps its caches in) and how to reach the
// network (the proxy set and the certificates the sandbox's proxy signs
// with). Every other variable is dropped, so an XDG, MISE_ or GO
// variable of the caller reaches no run, and no token does: the sbx proxy
// authenticates the GitHub requests (decided on 2026-10-22). GOSUMDB
// and the variables that turn it off are not kept: the checksum database
// is the guard of a go: tool (the mise.toml of this repository).
var passThrough = []string{
	"PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL",
	"GOPATH", "GOMODCACHE", "GOCACHE",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "ALL_PROXY",
	"http_proxy", "https_proxy", "no_proxy", "all_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR",
}

// TrustedPathsEnv is the variable that names the configuration paths
// mise trusts (04 4.3, REQ-0426, REQ-0621).
const TrustedPathsEnv = "MISE_TRUSTED_CONFIG_PATHS"

// The paths no file can be at, which the global and the system
// configuration of mise are pointed at: /dev/null is not a directory, so
// nothing can be planted below it, and a path that does not exist is read
// as empty (measured with mise 2026.10.3, which then reads no conf.d of
// either).
const (
	noGlobalConfig = "/dev/null/julieta-global.toml"
	noSystemConfig = "/dev/null/julieta-system.toml"
)

// checkTrusted refuses a repo path MISE_TRUSTED_CONFIG_PATHS cannot
// hold as one path: a relative one, one with a colon, which mise reads as
// two paths, one with a control character, and an unclean one.
func checkTrusted(p string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p || strings.ContainsFunc(p, func(r rune) bool { return r == ':' || r < 0x20 || r == 0x7f }) {
		return fmt.Errorf("the repo path %q cannot be one path of %s: it is not absolute and clean, or holds a colon or a control character", p, TrustedPathsEnv)
	}
	return nil
}

// Environ returns the whole environment of a mise run, as "KEY=value"
// (04 4.3, REQ-0426, as amended on 2026-10-22): what passThrough keeps of
// the caller's, looked up with lookup; the data, cache and state dirs of
// mise below home, not wherever the caller says; the global and the
// system configuration pointed at paths that cannot hold a file; then
// MISE_LOCKED=1, MISE_DISABLE_UPDATE_WARNING=1,
// MISE_USE_VERSIONS_HOST_TRACK=false, MISE_AUTO_INSTALL=false,
// MISE_YES=1 and MISE_PARANOID=1; and MISE_TRUSTED_CONFIG_PATHS set to
// trusted, the repo paths of the manifest, joined by ":". A path that
// cannot be one is an error (checkTrusted).
//
// MISE_PARANOID=1 is in the list because, measured with mise 2026.10.3,
// MISE_YES=1 makes mise trust every configuration file it reads, so a
// mise.toml outside MISE_TRUSTED_CONFIG_PATHS was read and the command of
// its exec() template ran, for mise env and for mise install alike; with
// MISE_PARANOID=1 beside MISE_YES=1 the same file is refused as not
// trusted, and a path of MISE_TRUSTED_CONFIG_PATHS still is trusted. It
// also turns on the verification of attestations, which contacts
// tuf-repo-cdn.sigstore.dev (07 7.4). What it does not do: mise reads
// the configuration of a trusted repo as the repo's own code, so an
// exec() template, a task or a hook of that mise.toml runs by design
// (REQ-0621; the residual of 05 5.4). TestJulietaInstallRefusesUntrustedConfig
// of the container e2e measures the refusal.
func Environ(lookup func(string) (string, bool), home string, trusted []string) ([]string, error) {
	for _, p := range trusted {
		if err := checkTrusted(p); err != nil {
			return nil, err
		}
	}
	var env []string
	for _, k := range passThrough {
		if v, ok := lookup(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return append(env,
		"MISE_DATA_DIR="+filepath.Join(home, ".local", "share", "mise"),
		"MISE_CACHE_DIR="+filepath.Join(home, ".cache", "mise"),
		"MISE_STATE_DIR="+filepath.Join(home, ".local", "state", "mise"),
		"MISE_GLOBAL_CONFIG_FILE="+noGlobalConfig,
		"MISE_SYSTEM_CONFIG_FILE="+noSystemConfig,
		"MISE_LOCKED=1",
		"MISE_DISABLE_UPDATE_WARNING=1",
		"MISE_USE_VERSIONS_HOST_TRACK=false",
		"MISE_AUTO_INSTALL=false",
		"MISE_YES=1",
		"MISE_PARANOID=1",
		TrustedPathsEnv+"="+strings.Join(trusted, ":"),
	), nil
}

// Driver runs mise. Every run goes through internal/procgroup under the
// class Mise of the timeout table of 04 4.1, so a mise that hangs is
// killed with its group and its run is the error subprocess-timeout
// (REQ-0300); a test sets the limit through the seam of that package
// (procgroup.WithTestTimeout).
//
// The program is "mise" looked up in the PATH of this process. The kit
// installs it at a known place, but the manifest names none, so julieta
// cannot resolve it by a fixed path as it does git (05 5.1): a PATH the
// agent sets can choose the mise, as it can choose any program the
// agent's own shell starts (an open question of the task, the audit of
// 2026-10-22).
type Driver struct {
	// Env is the environment of every run: Environ's.
	Env []string
	// Program is the program run.
	Program string
	// start is procgroup.Run.
	start func(context.Context, procgroup.Command) error
}

// New returns a driver for mise, whose environment is Environ(lookup,
// home, trusted); it fails on a trusted path Environ refuses.
func New(lookup func(string) (string, bool), home string, trusted []string) (*Driver, error) {
	env, err := Environ(lookup, home, trusted)
	if err != nil {
		return nil, err
	}
	return &Driver{Env: env, Program: "mise", start: procgroup.Run}, nil
}

// capture keeps the first headLimit bytes and the last tailLimit bytes
// of what a run writes, so the details of an error hold the start of
// the output (04 4.1, Error ids, item 7) and the scan for what went
// wrong reads the end, where mise says it. It never fails a write, so
// mise never blocks on a full pipe.
type capture struct {
	head, tail []byte
}

// The bounds of capture.
const (
	headLimit = 4 << 10
	tailLimit = 64 << 10
)

// Write keeps what fits and reports all of p written.
func (c *capture) Write(p []byte) (int, error) {
	n := len(p)
	if room := headLimit - len(c.head); room > 0 {
		k := min(room, len(p))
		c.head = append(c.head, p[:k]...)
		p = p[k:]
	}
	if len(p) > 0 {
		c.tail = append(c.tail, p...)
		if len(c.tail) > tailLimit {
			c.tail = slices.Clone(c.tail[len(c.tail)-tailLimit:])
		}
	}
	return n, nil
}

// all returns what was kept, the head and then the tail: the whole
// output when it is short.
func (c *capture) all() string { return string(c.head) + string(c.tail) }

// Target is one run of mise in a repo: where it runs and what the
// errors of the run need to say.
type Target struct {
	// Dir is the repo's path, the working directory of mise.
	Dir string
	// Repo is the repo's dir in the manifest, for the messages.
	Repo string
	// Sandbox is the name of the sandbox, the project's name, for the
	// sbx command an install-blocked error names.
	Sandbox string
	// Egress is the egress set of the manifest: install-blocked says
	// whether the blocked host is in it.
	Egress []string
}

// Install runs "mise install" in the repo, locked, with the
// environment of 04 4.3 (REQ-0403). A mise that hangs is
// subprocess-timeout (RJ-323), which names the tool it was installing
// when it can tell; a download the egress policy may have blocked is
// install-blocked (RJ-341); any other failure is mise-failed (RJ-351),
// whose details hold the escaped argv, the status and the first 4 KiB of
// mise's output (04 4.1, Error ids, item 7).
func (d *Driver) Install(ctx context.Context, t Target) error {
	out, err := d.run(ctx, t.Dir, "install")
	if err == nil {
		return nil
	}
	if timeout, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return timeoutError(timeout, "install", t, out)
	}
	if b, ok := Blocked(out.all()); ok {
		return InstallBlockedError(b, t)
	}
	return failure(err, d.argv("install"), "install", t, out)
}

// Lock runs "mise lock --platform <platforms>" in the repo (REQ-0405),
// which rewrites mise.lock. The errors are Install's, but for the one
// of a blocked download, which is not told apart here.
func (d *Driver) Lock(ctx context.Context, t Target, platforms []string) error {
	args := []string{"lock", "--platform", strings.Join(platforms, ",")}
	out, err := d.run(ctx, t.Dir, args...)
	if err == nil {
		return nil
	}
	if timeout, ok := errors.AsType[*procgroup.TimeoutError](err); ok {
		return timeoutError(timeout, "lock", t, out)
	}
	return failure(err, d.argv(args...), "lock", t, out)
}

// argv returns the argv of a run, for the details of an error.
func (d *Driver) argv(args ...string) []string { return append([]string{d.Program}, args...) }

// run runs mise with args in dir, its output captured.
func (d *Driver) run(ctx context.Context, dir string, args ...string) (*capture, error) {
	out := &capture{}
	err := d.start(ctx, procgroup.Command{
		Class:  procgroup.Mise,
		Argv:   d.argv(args...),
		Dir:    dir,
		Env:    d.Env,
		Stdout: out,
		Stderr: out,
	})
	return out, err
}

// failure is the error mise-failed for a mise that did not succeed and
// is neither a timeout nor a blocked download. The message names the
// verb and the repo; the argv, the status and the output are details
// (04 4.1, Error ids, item 7).
func failure(err error, argv []string, verb string, t Target, out *capture) error {
	status := err.Error()
	if exit, ok := errors.AsType[*procgroup.ExitError](err); ok {
		status = "exited " + fmt.Sprint(exit.Code)
		if exit.Signal != 0 {
			status = "ended by signal " + fmt.Sprint(int(exit.Signal))
		}
	}
	details := []cli.Detail{
		{Key: "argv", Value: termsafe.Escape(strings.Join(argv, " "))},
		{Key: "status", Value: termsafe.Escape(status)},
	}
	if head := strings.TrimSpace(string(out.head)); head != "" {
		details = append(details, cli.Detail{Key: "stderr", Value: termsafe.Escape(head)})
	}
	return MiseFailed.Wrap(err, fmt.Sprintf("mise %s failed in %s", verb, termsafe.Escape(t.Repo)), details...)
}

// progress matches the name@version of a tool in a line of mise's
// progress output, "mise ✓ jq@1.7.1  2.0s" or "mise ✗ http:foo@1.0".
var progress = regexp.MustCompile(`^mise\s+(?:(\S+)\s+)?([^\s@]\S*@\S+)`)

// installing returns the tool mise was working on when it was killed:
// the last one of its progress lines that is not marked done, "" when
// the output names none.
func installing(output string) string {
	var tool string
	for line := range strings.Lines(output) {
		if m := progress.FindStringSubmatch(line); m != nil && m[1] != "✓" {
			tool = m[2]
		}
	}
	return tool
}

// timeoutError returns subprocess-timeout for a mise that ran past its
// limit, naming the tool (04 4.3), the class and its limit (REQ-0301).
func timeoutError(e *procgroup.TimeoutError, verb string, t Target, out *capture) error {
	tool := installing(out.all())
	named := "mise"
	if tool != "" {
		named = tool
	}
	details := []cli.Detail{
		{Key: "tool", Value: termsafe.Escape(named)},
		{Key: "repo", Value: termsafe.Escape(t.Repo)},
		{Key: "class", Value: fmt.Sprintf("%s, limit %s", e.Class, e.Limit)},
	}
	return cli.SubprocessTimeout.Wrap(e, fmt.Sprintf("mise %s of %s ran past its limit while working on %s", verb, termsafe.Escape(t.Repo), termsafe.Escape(named)), details...)
}

// Block is what the output of a failed install says about a refused
// download: the tool, the host when mise names it, and whether the form
// was a tunnel a proxy refused rather than an HTTP 403.
type Block struct {
	Tool   string
	Host   string
	Tunnel bool
}

// The two forms mise 2026.10.3 prints for a refused download, measured: a
// server that answers 403, "Failed to install http:foo@1.0: HTTP status
// client error (403 Forbidden) for url (http://127.0.0.1:18403/foo)", and
// a proxy that answers the CONNECT with 403, "Failed to install <tool>:
// error sending request: client error (Connect): tunnel error:
// unsuccessful", whose host is only in the retry warnings before it:
// "HTTP GET https://api.github.com/... attempt 3 failed".
var (
	blockedForm = regexp.MustCompile(`Failed to install (\S+): HTTP status client error \(403[^)]*\) for url \(([^)\s]+)\)`)
	tunnelForm  = regexp.MustCompile(`Failed to install (\S+): error sending request: client error \(Connect\): tunnel error: unsuccessful`)
	retryForm   = regexp.MustCompile(`HTTP GET (\S+) attempt \d+ failed`)
)

// Blocked reports whether the output of a failed mise install says a
// download was refused, by HTTP 403 or by a proxy that refused the
// tunnel, and returns what it names. A 403 is not proof of the egress
// policy, and the caller says so (InstallBlockedError); the host is
// lower-cased and may be empty, and it is not yet a valid domain
// (spec.ValidDomain).
func Blocked(output string) (Block, bool) {
	if m := blockedForm.FindStringSubmatch(output); m != nil {
		host := hostOf(m[2])
		if host == "" {
			return Block{}, false
		}
		return Block{Tool: trimVersion(m[1]), Host: host}, true
	}
	m := tunnelForm.FindStringSubmatch(output)
	if m == nil {
		return Block{}, false
	}
	b := Block{Tool: trimVersion(m[1]), Tunnel: true}
	for _, r := range retryForm.FindAllStringSubmatch(output, -1) {
		if host := hostOf(r[1]); host != "" {
			b.Host = host
		}
	}
	return b, true
}

// hostOf returns the lower-cased host of a URL as mise prints it, "" when
// it names none. A URL the parser refuses, which a repo's own text can
// be, is cut by hand after "://", so the host is still shown, and spec.ValidDomain
// then keeps it out of any command.
func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return strings.ToLower(u.Hostname())
	}
	_, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return ""
	}
	rest, _, _ = strings.Cut(rest, "/")
	if _, after, ok := strings.Cut(rest, "@"); ok {
		rest = after
	}
	host, _, _ := strings.Cut(rest, ":")
	return strings.ToLower(host)
}

// trimVersion returns the tool of "tool@version".
func trimVersion(tool string) string {
	if i := strings.LastIndex(tool, "@"); i > 0 {
		return tool[:i]
	}
	return tool
}

// InstallBlockedError returns install-blocked (RJ-341) for a refused
// download (04 4.3, REQ-0404; 07 7.5, REQ-0637). The message says what
// mise saw, since an HTTP 403 or a refused tunnel is not proof that the
// egress policy did it; the details name the tool, the host, the sbx
// command that approves the host once for this sandbox, that the
// approval is lost at recreate, the durable fix and whether the host is
// in the egress set of the manifest. A host that is no plain domain is
// shown and no command is built from it.
func InstallBlockedError(b Block, t Target) *cli.Error {
	tool := termsafe.Escape(b.Tool)
	host := b.Host
	shown := termsafe.Escape(host)
	saw := "mise got HTTP 403 from " + shown
	if b.Tunnel {
		saw = "a proxy refused the tunnel to " + shown
		if host == "" {
			saw = "a proxy refused the tunnel"
		}
	}
	e := InstallBlocked.New(fmt.Sprintf("%s while installing %s; if that is the egress policy, the host needs approval", saw, tool),
		cli.Detail{Key: "tool", Value: tool},
		cli.Detail{Key: "host", Value: shown},
	)
	switch {
	case host == "":
		e.Details = append(e.Details, cli.Detail{Key: "approve once", Value: "not offered: mise named no host, so read the sbx approval queue"})
	case spec.ValidDomain(host) != nil:
		e.Details = append(e.Details, cli.Detail{Key: "approve once", Value: "not offered: the host is not a plain domain name"})
	default:
		set := "is not in the egress set of the manifest, so romeu never applied it"
		if slices.Contains(t.Egress, host) {
			set = "is in the egress set of the manifest, so romeu applied it and the refusal may come from elsewhere"
		}
		e.Details = append(e.Details,
			cli.Detail{Key: "egress set", Value: "the host " + set},
			cli.Detail{Key: "approve once", Value: "on the host, sbx policy allow network --sandbox " + termsafe.Escape(t.Sandbox) + " " + host},
		)
	}
	return withTail(e, shown)
}

// withTail adds the details every install-blocked error holds: that the
// approval is lost at recreate and the durable fix.
func withTail(e *cli.Error, host string) *cli.Error {
	name := "the host"
	if host != "" {
		name = host
	}
	e.Details = append(e.Details,
		cli.Detail{Key: "approval lifetime", Value: "this approval is lost when the sandbox is recreated"},
		cli.Detail{Key: "durable fix", Value: "add " + name + " to egress.extra of the project spec, or the tool to egress.tools, then run romeu sync and approve gate 2; egress follows the ref of the spec, so a tool added on a branch installs once it merges"},
	)
	return e
}
