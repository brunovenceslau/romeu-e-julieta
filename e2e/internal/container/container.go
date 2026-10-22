// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

// Package container is the harness of the container e2e (10 10.1):
// julieta in the workload's Debian base, the image pulled by the digest
// of kits/pins.yaml, with herdr and mise downloaded at their pins and
// checked against the sha256 the pin file holds (Q27). The tests are
// e2e/container_test.go, the harness entry, and the files of the julieta
// commands and of the kits beside it, under the tag e2e.
//
// A precondition the harness cannot meet is an infrastructure error
// (InfraError), never a skip (REQ-0798): the Docker daemon that does
// not answer, a pull that fails on transport (DNS, a timeout, a 5xx),
// and a herdr or mise download that fails. Fail names the test that met
// one in the private file InfraFileEnv names, which the e2e step of
// tools/ci makes and reads, and ends the job with an exit status of its
// own; the step reads no marker from the output, which a registry or a
// container could write. A cached download that fails its pin is
// removed and fetched once more, and the removal is a notice, a line in
// the private file NoticeFileEnv names, which the same step prints as
// an annotation: a channel no test output reaches. A manifest digest, or
// a download's sha256, that differs from its pin is a test failure, always: it is
// the one signal that a pinned artifact was substituted, and a security
// gate is hardened, not retried.
package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/google/renameio/v2"
)

// InfraPrefix starts the line Fail prints for an infrastructure error,
// for the person who reads the log; no program reads it.
const InfraPrefix = "container e2e: infrastructure error: "

// InfraFileEnv names the variable that gives the path of the file Fail
// names a test in, one per line, when the test met an infrastructure
// error. The e2e step of tools/ci (tools/ci/internal/container) makes
// the file, of mode 0600, and a test there holds its constant equal to
// this one. A run of go test with no such variable records nothing.
const InfraFileEnv = "ROMEU_E2E_INFRA_FILE"

// NoticeFileEnv names the variable that gives the path of the file a
// notice is appended to, one line for each, when the harness did
// something a person reading the run should see though no test failed:
// a cached download that failed its pin and was fetched again (Download;
// REQ-0798). The e2e step of tools/ci makes the file, of mode 0600,
// reads it after the run and prints each line as a warning of its own,
// as a GitHub Actions annotation on a runner; a test prints no line for
// it, so the classifier of that step, which counts the lines a test
// printed, never reads one, and no line a test prints can reach the
// annotation. A test there holds the constant equal to this one.
const NoticeFileEnv = "ROMEU_E2E_NOTICE_FILE"

// The preconditions of 10 10.1, as an InfraError names them.
const (
	Daemon = "the Docker daemon"
	Pull   = "the pull of the workload's image"
	Herdr  = "the herdr download"
	Mise   = "the mise download"
)

// InfraError is a precondition of the container e2e that could not be
// met: which one, and why.
type InfraError struct {
	Precondition string
	Err          error
}

func (e *InfraError) Error() string { return e.Precondition + ": " + e.Err.Error() }

// Unwrap returns the cause.
func (e *InfraError) Unwrap() error { return e.Err }

// Report returns the one line Fail prints for err: InfraPrefix and the
// error for an infrastructure error, and the error alone for any other.
// The line breaks of err become spaces, so no text an error carries
// (a registry's answer, say) starts a line of its own.
func Report(err error) string {
	line := strings.Join(strings.Fields(err.Error()), " ")
	if infra, ok := errors.AsType[*InfraError](err); ok && infra != nil {
		return InfraPrefix + line
	}
	return line
}

// Fail ends the test on err, with the line of Report, after it names
// the test in the file of InfraFileEnv when err is an infrastructure
// error. The step of tools/ci attributes an infrastructure error by test
// name, and counts the test as one only if it printed no line but the
// one Fail prints: a test that fails an assertion before it calls Fail
// with an InfraError reads as a test failure.
func Fail(tb testing.TB, err error) {
	tb.Helper()
	if _, ok := errors.AsType[*InfraError](err); ok {
		if rerr := recordInfra(tb.Name()); rerr != nil {
			tb.Fatalf("%s; and it could not be recorded as one: %v", Report(err), rerr)
		}
	}
	tb.Fatal(Report(err))
}

// recordInfra appends the name of a test to the file of InfraFileEnv,
// which must exist: the harness never makes it.
func recordInfra(name string) error {
	path := os.Getenv(InfraFileEnv)
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(name + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// notice appends msg to the file of NoticeFileEnv as one line, with its
// line breaks turned into spaces, which must exist: the harness never
// makes it. A run without the variable (go test by hand, with no step
// of tools/ci around it) records nothing, as recordInfra does: all
// output goes through the CLI writer (04 4.1), which a harness has not.
func notice(msg string) error {
	line := strings.Join(strings.Fields(msg), " ")
	path := os.Getenv(NoticeFileEnv)
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// CacheDir is the directory the harness keeps its downloads in, one
// subdirectory per platform. ci.yml restores and saves it with
// actions/cache, keyed by the sha256 of the fixture mise.lock
// (REQ-0797); the workflow grammar admits no expression that names a
// runner's directory, so the path is fixed. Every file in it is checked
// against its pin each time it is read, so a restored file is not
// trusted for having been cached.
const CacheDir = "/tmp/romeu-e-julieta-container-e2e"

// Config is how the harness reaches the Docker daemon, the release
// hosts and the pins. Tests of the harness replace a part of it.
type Config struct {
	// Root is the top of the repository.
	Root string
	// Pins are the entries of kits/pins.yaml the harness reads.
	Pins Pins
	// Env is the environment of the docker and go commands.
	Env []string
	// Client fetches the herdr and mise releases.
	Client *http.Client
	// HerdrBase and MiseBase are the URLs the releases are read below.
	HerdrBase, MiseBase string
	// CacheDir is where the downloads are kept (CacheDir).
	CacheDir string
	// GOARCH is the architecture of the runner, and of the binaries.
	GOARCH string
}

// Load returns the Config of a real run from the repository at root:
// the pins of kits/pins.yaml, this process's environment, and the
// release hosts of herdr and mise.
func Load(root, goarch string) (Config, error) {
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(PinsPath)))
	if err != nil {
		return Config{}, err
	}
	p, err := ParsePins(src)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Root:      root,
		Pins:      p,
		Env:       os.Environ(),
		Client:    &http.Client{Timeout: 5 * time.Minute},
		HerdrBase: "https://github.com/herdrdev/herdr/releases/download",
		MiseBase:  "https://github.com/jdx/mise/releases/download",
		CacheDir:  CacheDir,
		GOARCH:    goarch,
	}, nil
}

// docker runs one docker command and returns its standard output, or
// an error that holds what it printed to standard error.
func (c Config) docker(ctx context.Context, class procgroup.Class, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	err := procgroup.Run(ctx, procgroup.Command{
		Class:  class,
		Argv:   append([]string{"docker"}, args...),
		Env:    c.Env,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("docker %s: %w: %q", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// CheckDaemon asks the Docker daemon for its version: a daemon that does
// not answer is an infrastructure error.
func (c Config) CheckDaemon(ctx context.Context) error {
	out, err := c.docker(ctx, procgroup.ToolProbe, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return &InfraError{Daemon, err}
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return &InfraError{Daemon, errors.New("docker version printed no server version")}
	}
	return nil
}

// substituted are the words of a message about content, checked first:
// once every sha256 is masked out of the message (the pinned digest, and
// the blob URL of a layer fetch that failed on transport), any of them
// makes the failure a test failure, whatever else the message holds
// ("unexpected commit digest", "filesystem layer verification failed",
// "digest mismatch"), since the one signal that a pinned artifact was
// substituted must never be retried as infrastructure.
var substituted = []string{"digest", "verif", "mismatch"}

// anySHA256 is a sha256 as docker writes one in a reference or a URL.
var anySHA256 = regexp.MustCompile(`sha256:[0-9a-f]{64}`)

// transport are the messages with which docker reports a pull that
// failed on its way to the registry, each anchored on the text of the
// Go network error or of the registry's status around it: a name that
// does not resolve, a refused, unreachable or reset connection, a dial
// or handshake that timed out, and an answer of 5xx or of 429, the
// registry's limit of pulls. A timeout of the pull itself is the context
// or the procgroup timeout, checked apart.
var transport = []*regexp.Regexp{
	regexp.MustCompile(`dial tcp( [^ ]+)?: lookup [^ ]+( on [^ ]+)?: (no such host|server misbehaving|Temporary failure in name resolution)`),
	regexp.MustCompile(`dial tcp [^ ]+: connect: (connection refused|network is unreachable|no route to host)`),
	regexp.MustCompile(`dial tcp [^ ]+: i/o timeout`),
	regexp.MustCompile(`read tcp [^ ]+: read: connection reset by peer`),
	regexp.MustCompile(`net/http: TLS handshake timeout`),
	regexp.MustCompile(`net/http: request canceled while waiting for connection \(Client\.Timeout exceeded while awaiting headers\)`),
	regexp.MustCompile(`unexpected status from (HEAD|GET) request to [^ ]+: (5[0-9]{2}|429) [A-Z]`),
	regexp.MustCompile(`received unexpected HTTP status: (5[0-9]{2}|429) [A-Z]`),
	regexp.MustCompile(`toomanyrequests: `),
}

// pullFailure classifies the error of a failed docker pull of the image
// whose digest is pinned: a test failure for a message about content,
// checked first; an infrastructure error for a transport failure and a
// timeout; and a test failure for anything else, since a gate fails
// closed.
func pullFailure(err error, pinned string) error {
	msg := anySHA256.ReplaceAllString(strings.ReplaceAll(err.Error(), pinned, "<pinned>"), "<sha256>")
	lower := strings.ToLower(msg)
	for _, s := range substituted {
		if strings.Contains(lower, s) {
			return fmt.Errorf("the pulled content is not the one the pin names: %w", err)
		}
	}
	_, timedOut := errors.AsType[*procgroup.TimeoutError](err)
	if timedOut || errors.Is(err, context.DeadlineExceeded) || slices.ContainsFunc(transport, func(re *regexp.Regexp) bool { return re.MatchString(msg) }) {
		return &InfraError{Pull, err}
	}
	return fmt.Errorf("the pull of the workload's image failed: %w", err)
}

// PullTimeout bounds the pull of the workload's image, near 3 GB: below
// the test timeout of the e2e step (14 minutes, tools/ci/internal/
// container), so that a pull that hangs ends as the infrastructure
// error of a timeout and not as the panic of a test that ran too long.
const PullTimeout = 10 * time.Minute

// PullImage pulls ref, which names its digest, and then checks that the
// image docker holds carries that digest.
func (c Config) PullImage(ctx context.Context, ref string) error {
	_, digest, ok := strings.Cut(ref, "@")
	if !ok || !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("the reference %q names no digest", ref)
	}
	pullCtx, cancel := context.WithTimeout(ctx, PullTimeout)
	defer cancel()
	if _, err := c.docker(pullCtx, procgroup.ToolStep, "pull", "--quiet", ref); err != nil {
		return pullFailure(err, digest)
	}
	out, err := c.docker(ctx, procgroup.ToolProbe, "image", "inspect", "--format", "{{json .RepoDigests}}", ref)
	if err != nil {
		return fmt.Errorf("inspect the pulled image: %w", err)
	}
	var digests []string
	if err := json.Unmarshal(out, &digests); err != nil {
		return fmt.Errorf("the repository digests of the pulled image, %q: %w", out, err)
	}
	for _, d := range digests {
		if strings.HasSuffix(d, "@"+digest) {
			return nil
		}
	}
	return fmt.Errorf("the pulled image carries the digests %q, not %s", digests, digest)
}

// maxDownload bounds a release download: mise is the larger, near 135
// MB.
const maxDownload = 512 << 20

// platformDir returns the subdirectory of the cache of this platform,
// made if it is missing, after it checks that the cache is a directory
// of this user that no other user may write: a file another user could
// write would be read as the pinned one until its sha256 is checked,
// and the check would then fail the run.
func (c Config) platformDir() (string, error) {
	dir := filepath.Join(c.CacheDir, "linux-"+c.GOARCH)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	for _, d := range []string{c.CacheDir, dir} {
		info, err := os.Lstat(d)
		if err != nil {
			return "", err
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || !ok || int(st.Uid) != os.Getuid() || info.Mode().Perm()&0o022 != 0 {
			return "", fmt.Errorf("the cache %s is not a directory of this user that only it may write", d)
		}
	}
	return dir, nil
}

// Download installs the program name of what (Herdr or Mise) into dst,
// a directory only this user writes, and returns its path. The file is
// read from url when the cache does not hold it as <sha256>/<name>, and
// kept there; the copy into dst hashes the bytes it writes, read from
// the one open handle of the cached file, so the file the cases run is
// the one that was checked against want, whatever happens to the cache
// meanwhile. The name stays the program's, since mise reads its own name
// to tell itself from a shim. A download that fails is an infrastructure
// error; a file whose sha256 is not want is a test failure, the
// downloaded one and the cached one alike, except that a cached file
// that fails its check is deleted and fetched once more: a corrupt entry
// heals, and the fetched file must still match want. The removal is
// recorded as a notice (notice) that names the file and the pin, so a
// corrupted or tampered entry leaves a trace in the run (REQ-0798).
func (c Config) Download(ctx context.Context, what, name, url, want, dst string) (string, error) {
	cached, err := c.cache(ctx, what, name, url, want)
	if err != nil {
		return "", err
	}
	path, err := installVerified(what, cached, filepath.Join(dst, name), want)
	if _, corrupt := errors.AsType[*corruptError](err); !corrupt {
		return path, err
	}
	if rerr := os.Remove(cached); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return "", fmt.Errorf("%w; and it could not be removed: %w", err, rerr)
	}
	if nerr := notice(err.Error() + "; it was removed and fetched once more"); nerr != nil {
		return "", fmt.Errorf("%w; and the removal could not be recorded: %w", err, nerr)
	}
	if cached, err = c.cache(ctx, what, name, url, want); err != nil {
		return "", err
	}
	return installVerified(what, cached, filepath.Join(dst, name), want)
}

// corruptError is a cached file whose sha256 is not the pin.
type corruptError struct{ msg string }

func (e *corruptError) Error() string { return e.msg }

// installVerified copies src to dst, of mode 0755, hashing what it
// copies from the one handle it opened, and removes dst unless the
// sha256 of those bytes is want.
func installVerified(what, src, dst, want string) (path string, err error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(in, maxDownload+1)); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", &corruptError{fmt.Sprintf("%s: the cached file %s has the sha256 %s, not the pinned %s", what, src, got, want)}
	}
	return dst, nil
}

// cache returns the path of the cached file <sha256>/<name>, read from
// url when it is not there, checked against want and then written
// whole by a rename.
func (c Config) cache(ctx context.Context, what, name, url, want string) (string, error) {
	platform, err := c.platformDir()
	if err != nil {
		return "", err
	}
	if !isSHA256(want) {
		return "", fmt.Errorf("%s: the pin %q is not a sha256", what, want)
	}
	dir := filepath.Join(platform, want)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if _, err := os.Lstat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	body, err := c.fetch(ctx, url)
	if err != nil {
		return "", &InfraError{what, err}
	}
	if got := sha256Hex(body); got != want {
		return "", fmt.Errorf("%s: %s has the sha256 %s, not the pinned %s", what, url, got, want)
	}
	t, err := renameio.NewPendingFile(path,
		renameio.WithTempDir(dir), renameio.WithStaticPermissions(0o755))
	if err != nil {
		return "", err
	}
	defer func() { _ = t.Cleanup() }()
	if _, err := t.Write(body); err != nil {
		return "", err
	}
	return path, t.CloseAtomicallyReplace()
}

// fetch reads url whole, up to maxDownload.
func (c Config) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	if len(body) > maxDownload {
		return nil, fmt.Errorf("GET %s: more than %d bytes", url, maxDownload)
	}
	return body, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// pick returns the sha256 of a for the architecture goarch.
func pick(a Arches, goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return a.Amd64, nil
	case "arm64":
		return a.Arm64, nil
	}
	return "", fmt.Errorf("no pin for linux/%s", goarch)
}

// herdrAssets and miseArches name each architecture as the releases do.
var (
	herdrAssets = map[string]string{"amd64": "herdr-linux-x86_64", "arm64": "herdr-linux-aarch64"}
	miseArches  = map[string]string{"amd64": "x64", "arm64": "arm64"}
)

// HerdrBinary installs the pinned herdr of the runner's architecture
// into dst.
func (c Config) HerdrBinary(ctx context.Context, dst string) (string, error) {
	want, err := pick(c.Pins.Herdr.SHA256, c.GOARCH)
	if err != nil {
		return "", err
	}
	url := c.HerdrBase + "/v" + c.Pins.Herdr.Version + "/" + herdrAssets[c.GOARCH]
	return c.Download(ctx, Herdr, "herdr", url, want, dst)
}

// MiseBinary installs the pinned mise of the runner's architecture into
// dst.
func (c Config) MiseBinary(ctx context.Context, dst string) (string, error) {
	want, err := pick(c.Pins.Mise.SHA256, c.GOARCH)
	if err != nil {
		return "", err
	}
	v := c.Pins.Mise.Version
	url := c.MiseBase + "/v" + v + "/mise-v" + v + "-linux-" + miseArches[c.GOARCH]
	return c.Download(ctx, Mise, "mise", url, want, dst)
}

// BuildJulieta builds julieta for linux on the runner's architecture,
// with cgo off, into dir, and returns its path.
func (c Config) BuildJulieta(ctx context.Context, dir string) (string, error) {
	out := filepath.Join(dir, "julieta")
	b, err := procgroup.CombinedOutput(ctx, procgroup.Command{
		Class: procgroup.ToolStep,
		Argv:  []string{"go", "build", "-o", out, "./cmd/julieta"},
		Dir:   c.Root,
		Env:   append(append([]string{}, c.Env...), "GOOS=linux", "GOARCH="+c.GOARCH, "CGO_ENABLED=0"),
	})
	if err != nil {
		return "", fmt.Errorf("build julieta: %w: %q", err, b)
	}
	return out, nil
}

// Mount is one directory of the host that a container sees read-only,
// at the same path.
type Mount string

// RunUser is the user a container of the cases runs as: nobody, not
// the image's own user, who is in the sudo group of the image.
const RunUser = "65534:65534"

// Run runs argv in a new container of the workload's image and returns
// its standard output. The container has no network, no capability, no
// way to gain privileges, a bound on its processes and its memory, the
// mounts read-only, the user RunUser, and the program of argv[0] as the
// entrypoint.
func (c Config) Run(ctx context.Context, mounts []Mount, argv ...string) ([]byte, error) {
	args, err := runArgs(c.Pins.Workload.Ref, mounts, argv)
	if err != nil {
		return nil, err
	}
	return c.docker(ctx, procgroup.ToolStep, args...)
}

// runArgs returns the arguments of the docker run of Run.
func runArgs(image string, mounts []Mount, argv []string) ([]string, error) {
	if len(argv) == 0 {
		return nil, errors.New("no program to run")
	}
	args := []string{"run", "--rm", "--pull=never", "--network=none",
		"--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=256", "--memory=1g",
		"--user=" + RunUser}
	for _, m := range mounts {
		if strings.ContainsAny(string(m), ",=") {
			return nil, fmt.Errorf("a path holds \",\" or \"=\", which a bind mount cannot name: %s", m)
		}
		args = append(args, "--mount", "type=bind,source="+string(m)+",target="+string(m)+",readonly")
	}
	args = append(args, "--entrypoint", argv[0], image)
	return append(args, argv[1:]...), nil
}

// Workload is what the container cases share: the configuration, and
// the binaries the harness built and installed.
type Workload struct {
	Config
	// Bin is the directory of the binaries, which the cases mount: a
	// directory only this user writes.
	Bin string
	// Julieta is the julieta binary, and Herdr and Mise the pinned ones,
	// each in Bin.
	Julieta, Herdr, Mise string
}

// Prepare meets every precondition of the container e2e, in order: the
// daemon, the image, herdr and mise installed into bin, and julieta
// built there. bin must be a directory only this user writes; Prepare
// makes it readable by the container's user.
func Prepare(ctx context.Context, c Config, bin string) (*Workload, error) {
	if err := os.Chmod(bin, 0o755); err != nil {
		return nil, err
	}
	if err := c.CheckDaemon(ctx); err != nil {
		return nil, err
	}
	if err := c.PullImage(ctx, c.Pins.Workload.Ref); err != nil {
		return nil, err
	}
	herdr, err := c.HerdrBinary(ctx, bin)
	if err != nil {
		return nil, err
	}
	mise, err := c.MiseBinary(ctx, bin)
	if err != nil {
		return nil, err
	}
	julieta, err := c.BuildJulieta(ctx, bin)
	if err != nil {
		return nil, err
	}
	return &Workload{Config: c, Julieta: julieta, Bin: bin, Herdr: herdr, Mise: mise}, nil
}
