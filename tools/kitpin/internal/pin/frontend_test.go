// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	indexType   = "application/vnd.oci.image.index.v1+json"
	indexDigest = "sha256:e6a397771e865625047cf84256a914c7974b3e749d858259b5bc9c653ddb6f0d"
	tagsPath    = "/v2/docker/sandbox-kit/tags/list"
	m8Path      = "/v2/docker/sandbox-kit/manifests/3.0.0-m.8"
)

// frontendRoutes serves the recorded tag list and the recorded index
// of 3.0.0-m.8, with the headers the registry sent.
func frontendRoutes(t *testing.T) map[string]route {
	t.Helper()
	return map[string]route{
		"/token": {body: []byte(`{"token":"test-token"}`)},
		tagsPath: {body: fixture(t, "frontend/tags.json")},
		m8Path: {body: fixture(t, "frontend/3.0.0-m.8.json"), headers: map[string]string{
			"Content-Type": indexType, "Docker-Content-Digest": indexDigest,
		}},
	}
}

// TestFrontendPinsTheHighestMilestone is the acceptance case.
func TestFrontendPinsTheHighestMilestone(t *testing.T) {
	h := newHarness(t, frontendRoutes(t))
	h.repo.Write("kits/pins.yaml", startPins)
	require.Equal(t, exitOK, h.run(t, "frontend"), h.stderr.String())
	assert.Equal(t, header()+`frontend:
  ref: docker/sandbox-kit:3.0.0-m.8@`+indexDigest+"\n"+startPins, h.read(t, "kits/pins.yaml"))
}

func TestFrontendFollowsTheLinkHeader(t *testing.T) {
	routes := frontendRoutes(t)
	routes[tagsPath] = route{
		body:    []byte(`{"name":"docker/sandbox-kit","tags":["3.0.0-m.2"]}`),
		headers: map[string]string{"Link": `</v2/docker/sandbox-kit/tags/page2?n=1000&last=3.0.0-m.2>; rel="next"`},
	}
	routes["/v2/docker/sandbox-kit/tags/page2"] = route{body: []byte(`{"name":"docker/sandbox-kit","tags":["3.0.0-m.10","3.0.0-m.9","latest"]}`)}
	routes["/v2/docker/sandbox-kit/manifests/3.0.0-m.10"] = routes[m8Path]
	h := newHarness(t, routes)
	require.Equal(t, exitOK, h.run(t, "frontend"), h.stderr.String())
	assert.Contains(t, h.read(t, "kits/pins.yaml"), "ref: docker/sandbox-kit:3.0.0-m.10@"+indexDigest)
}

func TestFrontendRefuses(t *testing.T) {
	cases := []struct {
		name  string
		patch func(map[string]route)
		want  string
	}{
		{"digest_header_differs", func(r map[string]route) {
			r[m8Path] = route{body: r[m8Path].body, headers: map[string]string{
				"Content-Type": indexType, "Docker-Content-Digest": "sha256:" + strings.Repeat("0", 64),
			}}
		}, "but its bytes hash to " + indexDigest},
		{"not_a_manifest", func(r map[string]route) {
			r[m8Path] = route{body: r[m8Path].body, headers: map[string]string{"Content-Type": "text/html"}}
		}, `has the type "text/html"`},
		{"no_milestone_tag", func(r map[string]route) {
			r[tagsPath] = route{body: []byte(`{"tags":["3","latest","3.0.0-m.x"]}`)}
		}, "has no tag 3.0.0-m.<N>"},
		{"single_manifest", func(r map[string]route) {
			r[m8Path] = route{body: r[m8Path].body, headers: map[string]string{"Content-Type": "application/vnd.oci.image.manifest.v1+json"}}
		}, "not an image index"},
		{"index_lacks_arm64", func(r map[string]route) {
			r[m8Path] = route{body: []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:` + strings.Repeat("1", 64) + `","size":480,"platform":{"architecture":"amd64","os":"linux"}}]}`),
				headers: map[string]string{"Content-Type": indexType}}
		}, "it lists no manifest for linux/arm64"},
		{"no_token", func(r map[string]route) {
			r["/token"] = route{body: []byte(`{}`)}
		}, "holds no token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			routes := frontendRoutes(t)
			c.patch(routes)
			h := newHarness(t, routes)
			h.repo.Write("kits/pins.yaml", startPins)
			assert.Equal(t, exitError, h.run(t, "frontend"))
			assert.Contains(t, h.stderr.String(), c.want)
			assert.Equal(t, startPins, h.read(t, "kits/pins.yaml"))
		})
	}
}

func TestNextLink(t *testing.T) {
	assert.Equal(t, "/v2/x/tags/list?last=a", nextLink(`</v2/x/tags/list?last=a>; rel="next"`))
	assert.Empty(t, nextLink(""))
	assert.Empty(t, nextLink(`<https://elsewhere.example/v2/x>; rel="next"`))
	assert.Empty(t, nextLink(`</v2/x/tags/list?last=a>; rel="prev"`))
}
