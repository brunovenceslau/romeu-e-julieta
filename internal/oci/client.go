// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The caps of the client (REQ-0515): every response is read through a
// limit, so a registry cannot make the client hold more than this.
const (
	// MaxManifestSize is the most bytes of an index or a manifest: the 4
	// MiB the distribution spec says a registry supports (a manifest
	// push of at least that size), which no real image comes near.
	MaxManifestSize = 4 << 20
	// MaxTokenSize is the most bytes of a token response.
	MaxTokenSize = 64 << 10
	// maxRedirects is the most redirects a request follows.
	maxRedirects = 5
	// requestTimeout bounds one request, the body read included.
	requestTimeout = 60 * time.Second
)

// The refusals of the guards of the client. They are wrapped with the
// registry's name, and FetchWorkload gives them the row upstream-shape.
var (
	// ErrInsecure is a URL that is not https, or that holds credentials.
	ErrInsecure = errors.New("oci: not an https URL")
	// ErrRedirect is a redirect to a URL that is not https, or one too
	// many.
	ErrRedirect = errors.New("oci: redirect refused")
	// ErrTooLarge is a response over its cap.
	ErrTooLarge = errors.New("oci: response over its cap")
	// ErrDigestMismatch is content whose sha256 is not the digest it was
	// asked for by.
	ErrDigestMismatch = errors.New("oci: content does not match its digest")
	// ErrCredentials is a registry that asks for credentials.
	ErrCredentials = errors.New("oci: the registry asks for credentials")
)

// Client is the guarded client of the registries (REQ-0515, I28). It
// reads anonymously: it holds no credential, reads no Docker config and
// runs no credential helper, whatever DOCKER_CONFIG or the home
// directory hold (REQ-0210); it speaks HTTPS only and follows only HTTPS
// redirects; it verifies the digest of every hop; and it caps every
// response. A Client is safe for concurrent use.
type Client struct {
	http *http.Client
}

// NewClient returns a Client with the guards on. The proxy of the
// environment is used, as for any HTTPS client, and the server's
// certificate is verified against the system roots.
func NewClient() *Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Client{http: &http.Client{
		Transport:     tr,
		Timeout:       requestTimeout,
		CheckRedirect: checkRedirect,
	}}
}

// checkRedirect lets a request follow https redirects, a few of them
// (REQ-0515).
func checkRedirect(req *http.Request, via []*http.Request) error {
	switch {
	case len(via) >= maxRedirects:
		return fmt.Errorf("%w: more than %d redirects", ErrRedirect, maxRedirects)
	case req.URL.Scheme != "https":
		return fmt.Errorf("%w: to a URL that is not https", ErrRedirect)
	case req.URL.User != nil:
		return fmt.Errorf("%w: to a URL with credentials", ErrRedirect)
	}
	return nil
}

// reply is a response read through its cap.
type reply struct {
	status int
	header http.Header
	body   []byte
}

// get makes one GET to rawURL, which must be https, and reads at most
// limit bytes of the body. It adds no header of its own: the caller's
// header is all the request carries besides Go's defaults, and the
// caller sets Authorization only on a registry request, with a token
// (never on the request for the token).
func (c *Client) get(ctx context.Context, rawURL string, header http.Header, limit int64) (*reply, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("%w: %s", ErrInsecure, quote(rawURL))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInsecure, quote(rawURL))
	}
	maps.Copy(req.Header, header)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("%w: the response says %d bytes, over the cap of %d", ErrTooLarge, resp.ContentLength, limit)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: the body passes the cap of %d", ErrTooLarge, limit)
	}
	return &reply{status: resp.StatusCode, header: resp.Header, body: body}, nil
}

// verifyDigest returns nil when body is the content that digest, a
// "sha256:<64 hex>", names (REQ-0515). A digest of another algorithm is
// refused: the product pins sha256 only (03 3.1, REQ-0189).
func verifyDigest(digest string, body []byte) error {
	hexSum, ok := strings.CutPrefix(digest, "sha256:")
	if !ok || !digestForm.MatchString(digest) {
		return fmt.Errorf("%w: %s is not sha256:<64 hex>", ErrDigestMismatch, quote(digest))
	}
	want, err := hex.DecodeString(hexSum)
	if err != nil {
		return fmt.Errorf("%w: %s is not hex", ErrDigestMismatch, quote(digest))
	}
	got := sha256.Sum256(body)
	if subtle.ConstantTimeCompare(got[:], want) != 1 {
		return fmt.Errorf("%w: the content is sha256:%s, and %s was asked for", ErrDigestMismatch, hex.EncodeToString(got[:]), digest)
	}
	return nil
}
