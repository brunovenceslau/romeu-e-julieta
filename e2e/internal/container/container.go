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
// and a herdr or mise download that fails. Fail prints InfraPrefix for
// one, and the e2e step of tools/ci reads that line and ends the job
// with an exit status of its own. A manifest digest, or a download's
// sha256, that differs from its pin is a test failure, always: it is
// the one signal that a pinned artifact was substituted, and a security
// gate is hardened, not retried.
package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
)

// InfraPrefix starts the line Fail prints for an infrastructure error.
// The e2e step of tools/ci (tools/ci/internal/container) matches it
// after the file and line go test prints, and a test there holds its
// constant equal to this one.
const InfraPrefix = "container e2e: infrastructure error: "

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

// Fail ends the test on err, with the line of Report.
func Fail(tb testing.TB, err error) {
	tb.Helper()
	tb.Fatal(Report(err))
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
		return stdout.Bytes(), fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
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

// substituted are the texts with which docker reports content whose
// digest is not the one it asked for: a test failure, whatever else the
// message holds.
var substituted = []string{"unexpected commit digest", "digest mismatch", "manifest verification failed", "digest did not match"}

// transport are the texts with which docker reports a pull that failed
// on its way to the registry: a name that does not resolve, a timeout, a
// refused or reset connection, and an answer of 5xx or of 429, the
// registry's limit of pulls, none of which says anything of the content.
var transport = regexp.MustCompile(`(?i)no such host|server misbehaving|temporary failure in name resolution|i/o timeout|timeout|deadline exceeded|connection refused|connection reset|network is unreachable|: 5[0-9][0-9] [A-Za-z]|: 429 |toomanyrequests`)

// pullFailure classifies the error of a failed docker pull: an
// infrastructure error for a transport failure and a timeout, and a
// test failure for a digest that differs and for anything else, since a
// gate fails closed.
func pullFailure(err error) error {
	msg := err.Error()
	for _, s := range substituted {
		if strings.Contains(strings.ToLower(msg), s) {
			return fmt.Errorf("the pulled content is not the one the pin names: %w", err)
		}
	}
	if _, ok := errors.AsType[*procgroup.TimeoutError](err); ok || transport.MatchString(msg) {
		return &InfraError{Pull, err}
	}
	return fmt.Errorf("the pull of the workload's image failed: %w", err)
}

// PullImage pulls ref, which names its digest, and then checks that the
// image docker holds carries that digest.
func (c Config) PullImage(ctx context.Context, ref string) error {
	_, digest, ok := strings.Cut(ref, "@")
	if !ok || !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("the reference %q names no digest", ref)
	}
	if _, err := c.docker(ctx, procgroup.ToolStep, "pull", "--quiet", ref); err != nil {
		return pullFailure(err)
	}
	out, err := c.docker(ctx, procgroup.ToolProbe, "image", "inspect", "--format", "{{json .RepoDigests}}", ref)
	if err != nil {
		return fmt.Errorf("inspect the pulled image: %w", err)
	}
	var digests []string
	if err := json.Unmarshal(out, &digests); err != nil {
		return fmt.Errorf("the repository digests of the pulled image: %w", err)
	}
	for _, d := range digests {
		if strings.HasSuffix(d, "@"+digest) {
			return nil
		}
	}
	return fmt.Errorf("the pulled image carries the digests %v, not %s", digests, digest)
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

// Download returns the path of the program name of what (Herdr or
// Mise), read from url and kept in the cache as <sha256>/<name>, after
// it checks the file against want; the name stays the program's, since
// mise reads its own name to tell itself from a shim. A download that fails is an
// infrastructure error; a file whose sha256 is not want is a test
// failure, the downloaded one and the cached one alike.
func (c Config) Download(ctx context.Context, what, name, url, want string) (string, error) {
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
	if got, err := fileSHA256(path); err == nil {
		if got != want {
			return "", fmt.Errorf("%s: the cached file %s has the sha256 %s, not the pinned %s", what, path, got, want)
		}
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
	tmp, err := os.CreateTemp(dir, ".download-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
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

// fileSHA256 returns the sha256 of the file at path.
func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(b), nil
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

// HerdrBinary downloads the pinned herdr of the runner's architecture.
func (c Config) HerdrBinary(ctx context.Context) (string, error) {
	want, err := pick(c.Pins.Herdr.SHA256, c.GOARCH)
	if err != nil {
		return "", err
	}
	url := c.HerdrBase + "/v" + c.Pins.Herdr.Version + "/" + herdrAssets[c.GOARCH]
	return c.Download(ctx, Herdr, "herdr", url, want)
}

// MiseBinary downloads the pinned mise of the runner's architecture.
func (c Config) MiseBinary(ctx context.Context) (string, error) {
	want, err := pick(c.Pins.Mise.SHA256, c.GOARCH)
	if err != nil {
		return "", err
	}
	v := c.Pins.Mise.Version
	url := c.MiseBase + "/v" + v + "/mise-v" + v + "-linux-" + miseArches[c.GOARCH]
	return c.Download(ctx, Mise, "mise", url, want)
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
		return "", fmt.Errorf("build julieta: %w: %s", err, b)
	}
	return out, nil
}

// Mount is one directory of the host that a container sees read-only,
// at the same path.
type Mount string

// Run runs argv in a new container of the workload's image, with no
// network, the mounts read-only, and the program of argv[0] as the
// entrypoint, and returns its standard output.
func (c Config) Run(ctx context.Context, mounts []Mount, argv ...string) ([]byte, error) {
	args := []string{"run", "--rm", "--pull", "never", "--network", "none"}
	for _, m := range mounts {
		if strings.ContainsAny(string(m), ",=") {
			return nil, fmt.Errorf("a path holds \",\" or \"=\", which a bind mount cannot name: %s", m)
		}
		args = append(args, "--mount", "type=bind,source="+string(m)+",target="+string(m)+",readonly")
	}
	args = append(args, "--entrypoint", argv[0], c.Pins.Workload.Ref)
	return c.docker(ctx, procgroup.ToolStep, append(args, argv[1:]...)...)
}

// Workload is what the container cases share: the configuration, and
// the binaries the harness built and downloaded.
type Workload struct {
	Config
	// Julieta is the julieta binary, in Bin.
	Julieta string
	// Bin is the directory of Julieta, which the cases mount.
	Bin string
	// Herdr and Mise are the pinned binaries, in the cache.
	Herdr, Mise string
}

// Prepare meets every precondition of the container e2e, in order: the
// daemon, the image, herdr, mise, and julieta built into bin.
func Prepare(ctx context.Context, c Config, bin string) (*Workload, error) {
	if err := c.CheckDaemon(ctx); err != nil {
		return nil, err
	}
	if err := c.PullImage(ctx, c.Pins.Workload.Ref); err != nil {
		return nil, err
	}
	herdr, err := c.HerdrBinary(ctx)
	if err != nil {
		return nil, err
	}
	mise, err := c.MiseBinary(ctx)
	if err != nil {
		return nil, err
	}
	julieta, err := c.BuildJulieta(ctx, bin)
	if err != nil {
		return nil, err
	}
	return &Workload{Config: c, Julieta: julieta, Bin: bin, Herdr: herdr, Mise: mise}, nil
}

// ModuleRoot returns the nearest directory at or above dir that holds
// go.mod.
func ModuleRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod above the test's directory")
		}
		dir = parent
	}
}
