// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/spec"
)

// DescriptorAnnotation is the annotation of a kit image's manifest that
// carries its descriptor as compact JSON, in the kit schema's words:
// "the vnd.docker.sandbox.kit.descriptor manifest annotation".
const DescriptorAnnotation = "vnd.docker.sandbox.kit.descriptor"

// The media types of the documents the client reads: the OCI image
// index and manifest (image-spec, image-index.md and manifest.md), and
// the Docker schema 2 manifest list and manifest registries still serve
// for images built by Docker.
const (
	mediaOCIIndex       = "application/vnd.oci.image.index.v1+json"
	mediaOCIManifest    = "application/vnd.oci.image.manifest.v1+json"
	mediaDockerList     = "application/vnd.docker.distribution.manifest.list.v2+json"
	mediaDockerManifest = "application/vnd.docker.distribution.manifest.v2+json"
)

// accept is the Accept header of a manifest request: the distribution
// spec's "client SHOULD include an Accept header indicating which
// manifest content types it supports".
var accept = strings.Join([]string{mediaOCIIndex, mediaDockerList, mediaOCIManifest, mediaDockerManifest}, ", ")

// Platform selects a manifest of an index.
type Platform struct {
	// OS is the operating system, such as "linux".
	OS string
	// Architecture is the CPU architecture in Go's names, such as
	// "amd64" and "arm64".
	Architecture string
}

// Fetched is a workload's descriptor, read and verified.
type Fetched struct {
	// Workload is the reference that was read.
	Workload spec.Workload
	// ManifestDigest is the digest of the manifest the descriptor was
	// read from: the workload's own when it is a single manifest, the
	// platform manifest's when it is an index.
	ManifestDigest string
	// Kit is the parsed descriptor.
	Kit Kit
}

// FetchWorkload reads the descriptor of the workload ref, a
// <registry>/<repository>@sha256:<64 hex> (the form of internal/spec,
// REQ-0189), for the platform p. The digest is verified at every hop
// (REQ-0515): the index by the digest of ref, the platform manifest by
// the digest the index lists, and the annotation with it, being a
// member of that manifest. The registry is read anonymously over HTTPS
// (REQ-0210).
//
// A registry that does not answer, or answers with a status that is not
// a manifest, is registry-unreachable (RJ-322). A document that does not
// decode, lacks a field read here, is of an unknown shape, or a response
// that fails a guard of the client, is upstream-shape (RJ-310), naming
// the registry. A descriptor the grammar refuses is an error that wraps
// ErrInvalid.
func (c *Client) FetchWorkload(ctx context.Context, ref string, p Platform) (Fetched, error) {
	w, err := spec.ParseWorkload(ref)
	if err != nil {
		return Fetched{}, fmt.Errorf("oci: %w", err)
	}
	host, repo, _ := strings.Cut(w.Repository, "/")
	r := &registry{c: c, host: host}
	r.endpoint, r.repo = endpoint(host, repo)
	f, err := r.fetch(ctx, w, p)
	if err != nil {
		return Fetched{}, r.classify(err)
	}
	return f, nil
}

// endpoint returns where requests for the repository repo of the
// registry host go, and the repository's name there: Docker Hub is
// served from registry-1.docker.io, and its official images live under
// library/.
func endpoint(host, repo string) (string, string) {
	if host != "docker.io" && host != "index.docker.io" {
		return host, repo
	}
	if !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	return "registry-1.docker.io", repo
}

// registry is one registry and repository, and the token a request to
// it got.
type registry struct {
	c        *Client
	host     string // the name in the reference, for messages
	endpoint string // where requests go
	repo     string
	token    string
}

// shapeError is the error of a document or a guard: upstream-shape. It
// is kept apart from a transport error until classify.
type shapeError struct{ reason string }

func (e *shapeError) Error() string { return e.reason }

// shape returns a shapeError.
func shape(format string, args ...any) error {
	return &shapeError{reason: fmt.Sprintf(format, args...)}
}

// reachError is the error of a registry that did not answer a manifest.
type reachError struct{ error }

// classify gives err its row: a shape or a guard is upstream-shape, a
// transport failure or a status is registry-unreachable, and a
// descriptor refused by the grammar stays an error of ErrInvalid.
func (r *registry) classify(err error) error {
	detail := cli.Detail{Key: "registry", Value: r.host}
	// upstream-shape names its source in the detail "upstream" (04 4.4).
	source := cli.Detail{Key: "upstream", Value: r.host}
	if se, ok := errors.AsType[*shapeError](err); ok {
		return cli.UpstreamShape.New("registry "+r.host+": "+se.reason, source)
	}
	for _, guard := range []error{ErrInsecure, ErrRedirect, ErrTooLarge, ErrDigestMismatch} {
		if errors.Is(err, guard) {
			return cli.UpstreamShape.Wrap(err, "registry "+r.host+": a guard of the client refused the response", source)
		}
	}
	if errors.Is(err, ErrInvalid) {
		return fmt.Errorf("registry %s: %w", r.host, err)
	}
	if rc, ok := errors.AsType[*reachError](err); ok {
		return RegistryUnreachable.New("registry "+r.host+": "+rc.Error(), detail)
	}
	return RegistryUnreachable.Wrap(err, "registry "+r.host+" did not answer", detail)
}

// The documents of a registry are outputs of a tool the product does not
// own: decoded tolerantly, an unknown member skipped, and checked
// strictly, a member that is read being required and typed, a duplicate
// member name refused by the decoder (REQ-0170, ADR 0041). A pointer
// field is one that is required.
type indexDoc struct {
	SchemaVersion *int       `json:"schemaVersion"`
	MediaType     *string    `json:"mediaType"`
	Manifests     []entryDoc `json:"manifests"`
}

type entryDoc struct {
	MediaType *string      `json:"mediaType"`
	Digest    *string      `json:"digest"`
	Platform  *platformDoc `json:"platform"`
}

type platformDoc struct {
	OS           *string `json:"os"`
	Architecture *string `json:"architecture"`
}

type manifestDoc struct {
	SchemaVersion *int              `json:"schemaVersion"`
	MediaType     *string           `json:"mediaType"`
	Annotations   map[string]string `json:"annotations"`
}

// fetch walks the hops.
func (r *registry) fetch(ctx context.Context, w spec.Workload, p Platform) (Fetched, error) {
	body, ctype, err := r.manifest(ctx, w.Digest)
	if err != nil {
		return Fetched{}, err
	}
	digest := w.Digest
	switch ctype {
	case mediaOCIIndex, mediaDockerList:
		digest, body, ctype, err = r.platformManifest(ctx, body, ctype, p)
		if err != nil {
			return Fetched{}, err
		}
	}
	if ctype != mediaOCIManifest && ctype != mediaDockerManifest {
		return Fetched{}, shape("the manifest is of the type %s, which this release does not read", quote(ctype))
	}
	var m manifestDoc
	if err := jsonv2.Unmarshal(body, &m); err != nil {
		return Fetched{}, shape("the manifest does not decode as JSON with typed members")
	}
	if m.SchemaVersion == nil || *m.SchemaVersion != 2 {
		return Fetched{}, shape("the manifest has no schemaVersion 2")
	}
	if err := sameType(m.MediaType, ctype); err != nil {
		return Fetched{}, err
	}
	ann, ok := m.Annotations[DescriptorAnnotation]
	if !ok || ann == "" {
		return Fetched{}, shape("the manifest has no annotation %s", DescriptorAnnotation)
	}
	kit, err := ParseAnnotation([]byte(ann))
	if err != nil {
		return Fetched{}, err
	}
	c := kit.Capabilities
	switch {
	case c.Kind != "workload":
		return Fetched{}, invalid("the descriptor of %s is not a workload", quote(w.Repository))
	case c.Build != "" || c.Dockerfile != "":
		return Fetched{}, invalid("the descriptor of a workload carries a recipe, which the set does not read")
	case len(c.Kits) > 0:
		return Fetched{}, invalid("the descriptor of a workload lists kits, whose capabilities the set does not hold")
	}
	return Fetched{Workload: w, ManifestDigest: digest, Kit: kit}, nil
}

// platformManifest picks the manifest of the index for p and reads it.
func (r *registry) platformManifest(ctx context.Context, body []byte, ctype string, p Platform) (digest string, out []byte, outType string, err error) {
	var ix indexDoc
	if err := jsonv2.Unmarshal(body, &ix); err != nil {
		return "", nil, "", shape("the index does not decode as JSON with typed members")
	}
	if ix.SchemaVersion == nil || *ix.SchemaVersion != 2 {
		return "", nil, "", shape("the index has no schemaVersion 2")
	}
	if err := sameType(ix.MediaType, ctype); err != nil {
		return "", nil, "", err
	}
	if ix.Manifests == nil {
		return "", nil, "", shape("the index has no manifests list")
	}
	for i, e := range ix.Manifests {
		if e.MediaType == nil || e.Digest == nil {
			return "", nil, "", shape("manifests[%d] of the index has no mediaType or no digest", i)
		}
		// An entry without a platform (an attestation, say) is not a
		// candidate; one that has it must have both of its members.
		if e.Platform == nil {
			continue
		}
		if e.Platform.OS == nil || e.Platform.Architecture == nil {
			return "", nil, "", shape("manifests[%d] of the index has a platform without os or architecture", i)
		}
		if *e.Platform.OS != p.OS || *e.Platform.Architecture != p.Architecture {
			continue
		}
		if *e.MediaType != mediaOCIManifest && *e.MediaType != mediaDockerManifest {
			continue
		}
		body, got, err := r.manifest(ctx, *e.Digest)
		if err != nil {
			return "", nil, "", err
		}
		return *e.Digest, body, got, nil
	}
	return "", nil, "", shape("the index lists no manifest for %s/%s", quote(p.OS), quote(p.Architecture))
}

// sameType refuses a manifest whose mediaType member names a type other
// than the Content-Type it was served as (distribution spec: a client
// should reject a manifest whose mediaType does not match).
func sameType(member *string, ctype string) error {
	if member != nil && *member != ctype {
		return shape("the mediaType of the document is %s and it was served as %s", quote(*member), quote(ctype))
	}
	return nil
}

// manifest reads the manifest digest names, with the token flow of a
// registry, and returns its body, verified, and its media type.
func (r *registry) manifest(ctx context.Context, digest string) ([]byte, string, error) {
	// The digest of an index entry is the registry's text and becomes
	// part of a URL: only the one form the product pins gets through.
	if !digestForm.MatchString(digest) {
		return nil, "", shape("a digest of the document is not sha256:<64 hex>: %s", quote(digest))
	}
	u := "https://" + r.endpoint + "/v2/" + r.repo + "/manifests/" + digest
	for attempt := 0; ; attempt++ {
		h := http.Header{"Accept": {accept}}
		if r.token != "" {
			h.Set("Authorization", "Bearer "+r.token)
		}
		resp, err := r.c.get(ctx, u, h, MaxManifestSize)
		if err != nil {
			return nil, "", err
		}
		switch {
		case resp.status == http.StatusUnauthorized && attempt == 0:
			if err := r.authenticate(ctx, resp.header.Get("Www-Authenticate")); err != nil {
				return nil, "", err
			}
			continue
		case resp.status == http.StatusUnauthorized:
			return nil, "", &reachError{fmt.Errorf("%w: it refuses an anonymous read of the repository", ErrCredentials)}
		case resp.status != http.StatusOK:
			return nil, "", &reachError{fmt.Errorf("it answered %d to the request for %s", resp.status, digest)}
		}
		if err := verifyDigest(digest, resp.body); err != nil {
			return nil, "", err
		}
		if h := resp.header.Get("Docker-Content-Digest"); h != "" && h != digest {
			return nil, "", fmt.Errorf("%w: the header Docker-Content-Digest is %s", ErrDigestMismatch, quote(h))
		}
		ctype, _, err := mime.ParseMediaType(resp.header.Get("Content-Type"))
		if err != nil {
			return nil, "", shape("the response has no usable Content-Type")
		}
		return resp.body, ctype, nil
	}
}

// authenticate answers a 401 as the token-authentication specification
// of distribution describes: the challenge names a realm, a service and
// a scope; the token is asked for anonymously, with no Authorization
// header, and presented to the registry after (REQ-0210).
func (r *registry) authenticate(ctx context.Context, challenge string) error {
	scheme, params := parseChallenge(challenge)
	switch {
	case strings.EqualFold(scheme, "basic"):
		return &reachError{fmt.Errorf("%w: it asks for a user name and a password", ErrCredentials)}
	case !strings.EqualFold(scheme, "bearer") || params["realm"] == "":
		return shape("the 401 has no Bearer challenge with a realm")
	}
	realm, err := url.Parse(params["realm"])
	if err != nil {
		return shape("the realm of the challenge is not a URL")
	}
	q := realm.Query()
	if s := params["service"]; s != "" {
		q.Set("service", s)
	}
	if s := params["scope"]; s != "" {
		q.Set("scope", s)
	}
	realm.RawQuery = q.Encode()
	resp, err := r.c.get(ctx, realm.String(), http.Header{}, MaxTokenSize)
	if err != nil {
		return err
	}
	if resp.status != http.StatusOK {
		return &reachError{fmt.Errorf("its token service answered %d", resp.status)}
	}
	var t struct {
		Token       *string `json:"token"`
		AccessToken *string `json:"access_token"`
	}
	if err := jsonv2.Unmarshal(resp.body, &t); err != nil {
		return shape("the token response does not decode as JSON with typed members")
	}
	switch {
	case t.Token != nil && *t.Token != "":
		r.token = *t.Token
	case t.AccessToken != nil && *t.AccessToken != "":
		r.token = *t.AccessToken
	default:
		return shape("the token response has neither token nor access_token")
	}
	if strings.ContainsAny(r.token, "\r\n") {
		r.token = ""
		return shape("the token holds a line break")
	}
	return nil
}

// parseChallenge reads a WWW-Authenticate value: a scheme and its
// key="value" parameters. A value it cannot read gives an empty scheme.
func parseChallenge(h string) (scheme string, params map[string]string) {
	params = map[string]string{}
	scheme, rest, _ := strings.Cut(strings.TrimSpace(h), " ")
	for rest = strings.TrimSpace(rest); rest != ""; {
		key, after, ok := strings.Cut(rest, "=")
		if !ok {
			return "", nil
		}
		key = strings.ToLower(strings.TrimSpace(key))
		after = strings.TrimSpace(after)
		var val string
		if strings.HasPrefix(after, `"`) {
			end := strings.Index(after[1:], `"`)
			if end < 0 {
				return "", nil
			}
			val, after = after[1:1+end], after[2+end:]
		} else {
			val, after, _ = strings.Cut(after, ",")
		}
		params[key] = val
		rest = strings.TrimLeft(after, ", ")
	}
	return scheme, params
}
