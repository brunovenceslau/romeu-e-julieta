// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// releaseBase is the fixed prefix of the URL of mise's release
// (REQ-1237), and releaseRoots the roots that replace the system's for
// the download. Only the tests of this package change them, to serve a
// fake release locally; no flag or variable reaches either.
var (
	releaseBase  = "https://github.com/jdx/mise/releases/download/"
	releaseRoots *x509.CertPool
)

// downloadLimit is the limit 04 4.1 gives a command step of tools/ci,
// and downloadCap the largest file the download keeps.
const (
	downloadLimit = 15 * time.Minute
	downloadCap   = 256 << 20
)

// releaseURL is the URL of the bare binary of mise for the pin's
// runner.
func releaseURL(pin mise.Pin) string {
	return releaseBase + "v" + pin.Version + "/" + pin.Asset()
}

// downloadClient is the HTTP client of the download: the proxy
// variables apply, TLS verification stays on (SSL_CERT_FILE, where set,
// names the roots), a redirect is followed to https only, the body is
// not decompressed (so the hash covers the bytes the server sent), and
// the whole request, body included, stops at downloadLimit.
func downloadClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	tr.DisableCompression = true
	tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: releaseRoots}
	return &http.Client{
		Transport: tr,
		Timeout:   downloadLimit,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("a redirect to %s leaves https", req.URL.Redacted())
			}
			if len(via) >= 10 {
				return errors.New("more than 10 redirects")
			}
			return nil
		},
	}
}

// download writes the file at url to w, and returns its sha256 in
// lowercase hex, computed as it is written.
func download(ctx context.Context, url string, w io.Writer) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := downloadClient().Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	if resp.ContentLength > downloadCap {
		return "", fmt.Errorf("%s is larger than the cap of %d MiB", url, downloadCap>>20)
	}
	h := sha256.New()
	if _, err := copyCapped(io.MultiWriter(w, h), resp.Body, downloadCap); err != nil {
		return "", fmt.Errorf("%s: %w", url, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyCapped copies src to dst, and fails once src holds more than
// limit bytes.
func copyCapped(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err == nil && n > limit {
		err = fmt.Errorf("the file is larger than the cap of %d MiB", limit>>20)
	}
	return n, err
}

// install installs the pin's mise at its cache path (REQ-1239) and
// returns that path. The directories are created with mode 0700, and
// one that is a symbolic link, is not this user's or that others can
// write fails, as does a symbolic link or anything else that is not a
// regular file at the cache path. A download that an interrupted setup
// left beside that path is removed first. The download goes to a new file
// beside that path, created exclusively with mode 0600 and hashed as it
// is written; unless its sha256 is the pin's, it is removed, never made
// executable, and the error names both hashes. A match is made
// executable by its owner only and renamed into place.
func install(ctx context.Context, pin mise.Pin) (string, error) {
	dirs, err := mise.CacheDirs(pin.Version)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dirs[0]), 0o700); err != nil {
		return "", err
	}
	for _, dir := range dirs {
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", err
		}
		if err := mise.CheckDir(dir); err != nil {
			return "", err
		}
	}
	dest := filepath.Join(dirs[len(dirs)-1], "mise")
	if info, err := os.Lstat(dest); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is a symbolic link, where setup installs mise: remove it, then run setup again", dest)
	} else if err == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file, where setup installs mise: remove it, then run setup again", dest)
	}
	stale, err := filepath.Glob(filepath.Join(filepath.Dir(dest), ".mise-*.download"))
	if err != nil {
		return "", err
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return "", err
		}
	}
	f, err := os.CreateTemp(filepath.Dir(dest), ".mise-*.download")
	if err != nil {
		return "", err
	}
	placed := false
	defer func() {
		if !placed {
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()
	url := releaseURL(pin)
	sum, err := download(ctx, url, f)
	if err != nil {
		return "", err
	}
	if sum != pin.Sha256 {
		return "", fmt.Errorf("%s has sha256 %s, and the matrix of %s or %s pins %s for %s: the file is removed", url, sum, mise.PinnedWorkflow, mise.ScheduledWorkflow, pin.Sha256, pin.Label)
	}
	if err := f.Chmod(0o700); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(f.Name(), dest); err != nil {
		return "", err
	}
	placed = true
	return dest, nil
}
