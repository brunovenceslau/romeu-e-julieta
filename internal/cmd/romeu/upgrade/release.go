// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// Repo is the repository romeu upgrade reads its releases from: the one
// compiled into romeu, never read from a setting or a response
// (REQ-1327).
const Repo = "brunovenceslau/romeu-e-julieta"

// The injected values of 10 10.4 (T196): the host of the releases API,
// the host the asset reads are redirected to, and a certificate the
// client trusts in place of the system's roots. A release build sets
// none of them, so it reads api.github.com and the asset host GitHub
// serves release downloads from (the host 07 7.4 measured with mise
// 2026.10.3 through a proxy that logged each host an install of gh
// reaches), and trusts the system's roots. The e2e sets the three with -ldflags -X to serve
// a fixture release over TLS from the test (e2e/upgrade_test.go).
var (
	apiHost   = "api.github.com"
	assetHost = "release-assets.githubusercontent.com"
	// rootCA is the base64 of one DER certificate, or empty.
	rootCA = ""
)

// The limits of REQ-1327: one timeout per request and a cap per
// response.
const (
	requestTimeout = 30 * time.Second
	apiCap         = 8 << 20
	assetCap       = 256 << 20
)

// pickCount is the number of releases the pick reads, newest first
// (REQ-1327: /releases?per_page=10).
const pickCount = 10

// ChecksumsName is the checksum file of a release (REQ-0904).
const ChecksumsName = "checksums.txt"

// source reads the releases of Repo: GET requests over HTTPS to the API
// host, with no Authorization header, whose asset reads may be
// redirected to the asset host and nowhere else.
type source struct {
	client     *http.Client
	api, asset string // the two hosts, host[:port]
	userAgent  string // GitHub's API refuses a request with none
	// apiCap and assetCap are the caps of a response of the API and of
	// an asset; a test lowers them.
	apiCap, assetCap int64
}

// newSource returns the source of the injected values.
func newSource(version string) (*source, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, cli.Internal.New("the default HTTP transport is not an *http.Transport")
	}
	transport = transport.Clone()
	if rootCA != "" {
		der, err := base64.StdEncoding.DecodeString(rootCA)
		if err != nil {
			return nil, cli.Internal.Wrap(err, "the injected root certificate is not base64")
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, cli.Internal.Wrap(err, "the injected root certificate does not parse")
		}
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return newSourceWith(&http.Client{Transport: transport}, apiHost, assetHost, version), nil
}

// newSourceWith returns a source over client for the two hosts, with the
// timeout and the redirect rule of REQ-1327; a test gives it the client
// of its TLS server.
func newSourceWith(client *http.Client, api, asset, version string) *source {
	c := *client
	c.Timeout = requestTimeout
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("more than 5 redirects")
		}
		if req.URL.Scheme != "https" || (req.URL.Host != asset && req.URL.Host != api) {
			return fmt.Errorf("a redirect to %s, which is neither the API host nor the asset host", termsafe.Escape(req.URL.Redacted()))
		}
		return nil
	}
	return &source{client: &c, api: api, asset: asset, userAgent: "romeu/" + version, apiCap: apiCap, assetCap: assetCap}
}

// Release is the part of a GitHub release the pick reads.
type Release struct {
	Tag        string  `json:"tag_name"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset is one asset of a release: its id, which names its download on
// the API host, and its name.
type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// tagForm is the form of a release tag (10 10.2, Release and bootstrap):
// v<major>.<minor>.<patch>, and the same with a suffix for a prerelease;
// tools/release/internal/build holds the same pattern.
var tagForm = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$`)

// pick returns the release to install (REQ-1324): the one tag names, or
// with no tag the newest of the last pickCount that is not a draft, a
// prerelease included. A tag that is no release is download-failed with
// the status of the API (404); so is a list with no published release,
// and a release whose tag is not of the tag form.
func (s *source) pick(ctx context.Context, tag string) (Release, error) {
	path := "/repos/" + Repo + "/releases?per_page=" + strconv.Itoa(pickCount)
	if tag != "" {
		path = "/repos/" + Repo + "/releases/tags/" + url.PathEscape(tag)
	}
	body, err := s.get(ctx, "https://"+s.api+path, "application/vnd.github+json", s.apiCap)
	if err != nil {
		return Release{}, err
	}
	var rel Release
	if tag != "" {
		if err := json.Unmarshal(body, &rel); err != nil {
			return Release{}, DownloadFailed.Wrap(err, "the release does not decode", cli.Detail{Key: "url", Value: "https://" + s.api + path})
		}
		if rel.Draft {
			return Release{}, DownloadFailed.New("release "+termsafe.Escape(tag)+" is a draft", cli.Detail{Key: "url", Value: "https://" + s.api + path})
		}
	} else {
		var list []Release
		if err := json.Unmarshal(body, &list); err != nil {
			return Release{}, DownloadFailed.Wrap(err, "the list of releases does not decode", cli.Detail{Key: "url", Value: "https://" + s.api + path})
		}
		found := false
		for _, r := range list {
			if !r.Draft {
				rel, found = r, true
				break
			}
		}
		if !found {
			return Release{}, DownloadFailed.New("the repository has no published release", cli.Detail{Key: "url", Value: "https://" + s.api + path})
		}
	}
	if !tagForm.MatchString(rel.Tag) || (tag != "" && rel.Tag != tag) {
		return Release{}, DownloadFailed.New("the release names the tag "+termsafe.Escape(strconv.Quote(rel.Tag))+", which is not a release tag", cli.Detail{Key: "url", Value: "https://" + s.api + path})
	}
	return rel, nil
}

// ArchiveName returns the name of the darwin archive of goarch in the
// release of tag (tools/release/internal/build, REQ-0904).
func ArchiveName(tag, goarch string) string {
	return fmt.Sprintf("romeu_%s_darwin_%s.tar.gz", tag, goarch)
}

// assets returns the archive of runtime.GOARCH and checksums.txt of
// rel, and download-failed when either is missing (REQ-1332).
func (rel Release) assets() (archive, checksums Asset, err error) {
	want := ArchiveName(rel.Tag, runtime.GOARCH)
	for _, a := range rel.Assets {
		switch a.Name {
		case want:
			archive = a
		case ChecksumsName:
			checksums = a
		}
	}
	if archive.ID <= 0 || checksums.ID <= 0 {
		return Asset{}, Asset{}, DownloadFailed.New("release "+rel.Tag+" has no "+want+" or no "+ChecksumsName,
			cli.Detail{Key: "release", Value: rel.Tag})
	}
	return archive, checksums, nil
}

// download returns the content of the asset a, read on the API host and
// redirected to the asset host, up to the asset cap.
func (s *source) download(ctx context.Context, a Asset) ([]byte, error) {
	return s.get(ctx, "https://"+s.api+"/repos/"+Repo+"/releases/assets/"+strconv.FormatInt(a.ID, 10), "application/octet-stream", s.assetCap)
}

// get reads the body of a GET of u with the Accept header accept, and
// returns download-failed for a request that fails or times out, a
// status that is not 2xx and a body over limit, with the URL and the
// status in the details. The request carries no Authorization header.
func (s *source) get(ctx context.Context, u, accept string, limit int64) ([]byte, error) {
	where := cli.Detail{Key: "url", Value: u}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, DownloadFailed.Wrap(err, "the request does not build", where)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", s.userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, DownloadFailed.Wrap(err, "the read failed", where)
	}
	defer func() { _ = resp.Body.Close() }()
	status := cli.Detail{Key: "status", Value: resp.Status}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, DownloadFailed.New("the read answered "+termsafe.Escape(resp.Status), where, status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, DownloadFailed.Wrap(err, "the read failed", where, status)
	}
	if int64(len(body)) > limit {
		return nil, DownloadFailed.New(fmt.Sprintf("the body passed its cap of %d bytes", limit), where, status)
	}
	return body, nil
}
