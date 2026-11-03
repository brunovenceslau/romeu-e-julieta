// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package upgrade

import (
	"encoding/base64"
	"encoding/json/v2"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
)

// requireID asserts that err is the error of the table row.
func requireID(t *testing.T, err error, row cli.Row) {
	t.Helper()
	require.Error(t, err)
	assert.Equal(t, row.ID, cli.Of(err).ID, "%v", err)
}

// TestPick holds REQ-1324 and REQ-1327 on the fixture of
// testdata/releases-handmade.json: with no tag, the newest release of
// /releases?per_page=10 that is not a draft, a prerelease included;
// --version reads /releases/tags/<tag> and refuses a tag that is no
// release, a draft and a tag of another form. Every request is a GET
// over HTTPS to the API host, with no Authorization header.
func TestPick(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, tag, want string
		path, query     string
		err             *cli.Row
	}{
		{name: "the newest that is not a draft, a prerelease included", want: "v1.0.0-alpha.2",
			path: "/repos/" + Repo + "/releases", query: "per_page=10"},
		{name: "a tag pins one", tag: "v1.0.0-alpha.1", want: "v1.0.0-alpha.1",
			path: "/repos/" + Repo + "/releases/tags/v1.0.0-alpha.1"},
		{name: "a tag that is no release", tag: "v9.9.9", err: &DownloadFailed,
			path: "/repos/" + Repo + "/releases/tags/v9.9.9"},
		{name: "a draft is no release to pin", tag: "v1.0.0-alpha.3", err: &DownloadFailed,
			path: "/repos/" + Repo + "/releases/tags/v1.0.0-alpha.3"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newReleaseServer(t)
			rel, err := srv.source().pick(t.Context(), tt.tag)
			if tt.err != nil {
				requireID(t, err, *tt.err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, rel.Tag)
			}
			assert.Equal(t, []request{{Method: http.MethodGet, Host: host(srv.api), Path: tt.path, Query: tt.query, TLS: true}}, srv.seen())
		})
	}
}

// TestPickRefusesWhatIsNoRelease: a list with no published release, and
// a release whose tag is not of the tag form, are download-failed.
func TestPickRefusesWhatIsNoRelease(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"drafts only":        `[{"tag_name":"v1.0.0","draft":true}]`,
		"empty":              `[]`,
		"a tag of no form":   `[{"tag_name":"latest","draft":false}]`,
		"a tag with a slash": `[{"tag_name":"v1.0.0/../x","draft":false}]`,
		"not json":           `<html>`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := newReleaseServer(t)
			srv.handle = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }
			_, err := srv.source().pick(t.Context(), "")
			requireID(t, err, DownloadFailed)
		})
	}
}

// TestGetFailures holds REQ-1332: a non-2xx answer, a timeout, a body
// over the cap and a redirect to a host that is neither of the two are
// download-failed, with the URL in the details.
func TestGetFailures(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		handle  func(srv *releaseServer) http.HandlerFunc
		arrange func(s *source)
		status  string
	}{
		{"a non-2xx answer", func(*releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "rate limited", http.StatusForbidden) }
		}, nil, "403 Forbidden"},
		{"a timeout", func(*releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-time.After(5 * time.Second):
				}
			}
		}, func(s *source) { s.client.Timeout = 50 * time.Millisecond }, ""},
		{"a body over the cap", func(*releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 65))) }
		}, func(s *source) { s.apiCap = 64 }, "200 OK"},
		{"a redirect to another host", func(*releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "https://example.com/elsewhere", http.StatusFound)
			}
		}, nil, ""},
		{"a redirect to plain http on the asset host", func(srv *releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "http://"+host(srv.asset)+"/download/x", http.StatusFound)
			}
		}, nil, ""},
		{"a redirect with a signed query to another host", func(*releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "https://user:pass@example.com/elsewhere?sig=secret#frag", http.StatusFound)
			}
		}, nil, ""},
		{"a signed redirect to the asset host that fails", func(srv *releaseServer) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, srv.asset.URL+"/hang?sig=secret", http.StatusFound)
			}
		}, func(s *source) { s.client.Timeout = 50 * time.Millisecond }, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newReleaseServer(t)
			srv.handle = tt.handle(srv)
			s := srv.source()
			if tt.arrange != nil {
				tt.arrange(s)
			}
			_, err := s.pick(t.Context(), "")
			requireID(t, err, DownloadFailed)
			details := cli.Of(err).Details
			assert.Contains(t, details, cli.Detail{Key: "url", Value: "https://" + host(srv.api) + "/repos/" + Repo + "/releases"},
				"the URL without its query")
			for _, leak := range []string{"secret", "per_page", "pass@", "frag"} {
				assert.NotContains(t, err.Error(), leak, "no query, user or fragment reaches the message")
				for _, d := range details {
					assert.NotContains(t, d.Value, leak, "no query, user or fragment reaches the details")
				}
			}
			if tt.status != "" {
				assert.Contains(t, details, cli.Detail{Key: "status", Value: tt.status})
			}
			for _, r := range srv.seen() {
				assert.Contains(t, []string{host(srv.api), host(srv.asset)}, r.Host, "no request leaves the two hosts")
			}
		})
	}
}

// TestAssetsAndDownload: the archive of runtime.GOARCH and checksums.txt
// are read on the API host by their ids and redirected to the asset
// host, both over HTTPS with no Authorization header; a release without
// either is download-failed, and so is an asset over its cap.
func TestAssetsAndDownload(t *testing.T) {
	t.Parallel()
	srv := newReleaseServer(t)
	s := srv.source()
	rel, err := s.pick(t.Context(), "")
	require.NoError(t, err)
	archive, sums, err := rel.assets()
	require.NoError(t, err)
	assert.Equal(t, ArchiveName("v1.0.0-alpha.2", runtime.GOARCH), archive.Name)
	assert.Equal(t, ChecksumsName, sums.Name)

	got, err := s.download(t.Context(), archive)
	require.NoError(t, err)
	assert.Equal(t, srv.files[archive.Name], got)
	assert.Equal(t, []request{
		{Method: http.MethodGet, Host: host(srv.api), Path: "/repos/" + Repo + "/releases", Query: "per_page=10", TLS: true},
		{Method: http.MethodGet, Host: host(srv.api), Path: "/repos/" + Repo + "/releases/assets/" + strconv.FormatInt(archive.ID, 10), TLS: true},
		{Method: http.MethodGet, Host: host(srv.asset), Path: "/download/" + archive.Name, TLS: true},
	}, srv.seen())

	s.assetCap = 8
	_, err = s.download(t.Context(), archive)
	requireID(t, err, DownloadFailed)

	got, err = s.download(t.Context(), sums)
	require.NoError(t, err, "checksums.txt has a cap of its own")
	s.checksumsCap = int64(len(got)) - 1
	_, err = s.download(t.Context(), sums)
	requireID(t, err, DownloadFailed)
	require.ErrorContains(t, err, "the body passed its cap of "+strconv.Itoa(len(got)-1)+" bytes")

	for name, assets := range map[string][]Asset{
		"no archive of this arch": {sums, {ID: 9, Name: ArchiveName(rel.Tag, "other")}},
		"no checksums.txt":        {archive},
		"an archive of id 0":      {{Name: archive.Name}, sums},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Release{Tag: rel.Tag, Assets: assets}.assets()
			requireID(t, err, DownloadFailed)
		})
	}
}

// TestNewSource holds the injected values of 10 10.4: the source reads
// the API host and the asset host it was built with, and trusts the
// injected certificate in place of the system's roots; an injected
// certificate that is not base64 DER is the error internal. It replaces
// the injected values, so it does not run in parallel.
func TestNewSource(t *testing.T) {
	srv := newReleaseServer(t)
	oldAPI, oldAsset, oldCA := apiHost, assetHost, rootCA
	t.Cleanup(func() { apiHost, assetHost, rootCA = oldAPI, oldAsset, oldCA })
	apiHost, assetHost = host(srv.api), host(srv.asset)

	rootCA = ""
	s, err := newSource("v1.0.0-alpha.1")
	require.NoError(t, err)
	_, err = s.pick(t.Context(), "")
	requireID(t, err, DownloadFailed)
	require.ErrorContains(t, err, "certificate", "the system's roots do not hold the test's certificate")

	rootCA = base64.StdEncoding.EncodeToString(srv.api.Certificate().Raw)
	s, err = newSource("v1.0.0-alpha.1")
	require.NoError(t, err)
	assert.Equal(t, requestTimeout, s.client.Timeout)
	assert.Equal(t, 30*time.Second, requestTimeout, "REQ-1327: a 30 s timeout per request")
	assert.EqualValues(t, 8<<20, s.apiCap, "REQ-1327: 8 MiB for the API")
	assert.EqualValues(t, 1<<20, s.checksumsCap, "REQ-1327: 1 MiB for checksums.txt")
	assert.EqualValues(t, 256<<20, s.assetCap, "REQ-1327: 256 MiB for an asset")
	rel, err := s.pick(t.Context(), "")
	require.NoError(t, err)
	archive, _, err := rel.assets()
	require.NoError(t, err)
	_, err = s.download(t.Context(), archive)
	require.NoError(t, err, "the redirect to the asset host is followed")

	for _, bad := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("not a certificate"))} {
		rootCA = bad
		_, err := newSource("v1.0.0-alpha.1")
		requireID(t, err, cli.Internal)
	}
}

// TestRecordedShapeDecodes holds the decode of the pick against the
// shape GitHub serves (10 10.1, recorded): testdata/releases-recorded.json
// is /releases?per_page=10 of goreleaser/goreleaser as the API answered
// it, five nightly prereleases then five releases, each with
// checksums.txt and darwin archives. Release reads its tags, its draft
// and prerelease flags, and the ids and names of its assets, and the
// pick over it takes the newest, a prerelease.
func TestRecordedShapeDecodes(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "releases-recorded.json"))
	require.NoError(t, err)
	var list []Release
	require.NoError(t, json.Unmarshal(data, &list))
	require.Len(t, list, 10)
	var tags []string
	var prereleases []bool
	for _, r := range list {
		tags = append(tags, r.Tag)
		prereleases = append(prereleases, r.Prerelease)
		assert.False(t, r.Draft, r.Tag)
		assert.True(t, tagForm.MatchString(r.Tag), r.Tag)
		names := map[string]int64{}
		for _, a := range r.Assets {
			assert.Positive(t, a.ID, "%s: %s", r.Tag, a.Name)
			names[a.Name] = a.ID
		}
		assert.Len(t, r.Assets, 55, r.Tag)
		assert.Contains(t, names, ChecksumsName, r.Tag)
		assert.Contains(t, names, "goreleaser_Darwin_arm64.tar.gz", r.Tag)
		assert.Contains(t, names, "goreleaser_Darwin_x86_64.tar.gz", r.Tag)
	}
	assert.Equal(t, []string{
		"v2.20.0-3d5c3a2e-nightly", "v2.20.0-538a99ed-nightly", "v2.20.0-2e260e4a-nightly", "v2.20.0-57b095c9-nightly", "v2.20.0-0162ee4b-nightly",
		"v2.19.0", "v2.18.3", "v2.18.2", "v2.18.1", "v2.18.0",
	}, tags)
	assert.Equal(t, []bool{true, true, true, true, true, false, false, false, false, false}, prereleases)
	assert.Equal(t, Asset{ID: 700022707, Name: ChecksumsName}, list[0].Assets[0])

	srv := newReleaseServer(t)
	srv.handle = func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) }
	rel, err := srv.source().pick(t.Context(), "")
	require.NoError(t, err)
	assert.Equal(t, "v2.20.0-3d5c3a2e-nightly", rel.Tag, "the newest, a prerelease")
	assert.True(t, rel.Prerelease)
}
