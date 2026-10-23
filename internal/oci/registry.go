// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package oci

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

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

// Platform selects a manifest of an index.
type Platform struct {
	// OS is the operating system, such as "linux".
	OS string
	// Architecture is the CPU architecture in Go's names, such as
	// "amd64" and "arm64".
	Architecture string
	// Variant is the CPU variant, such as "v8"; when set, only an entry
	// with that variant matches, and when empty an entry of any variant
	// does, so two variants of the platform are ambiguous.
	Variant string
}

// String returns the platform as os/arch or os/arch/variant, the form
// the descriptor cache stores.
func (p Platform) String() string {
	if p.Variant == "" {
		return p.OS + "/" + p.Architecture
	}
	return p.OS + "/" + p.Architecture + "/" + p.Variant
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
// (REQ-0210), through go-containerregistry, whose requests all pass the
// guard of guard.go.
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
	// The reference is checked twice, by the form of the spec and by the
	// library's strict names, which also say where the registry is.
	digest, err := name.NewDigest(w.Repository+"@"+w.Digest, name.StrictValidation)
	if err != nil {
		return Fetched{}, fmt.Errorf("oci: %w", err)
	}
	where := digest.Context().RegistryStr()
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	g := newGuard(c.transport, where)
	g.testAddr = c.testAddr
	r := &registry{host: where, ref: digest, guard: g, opts: []remote.Option{
		// Anonymous only. Not remote.WithAuthFromKeychain, and never the
		// library's default keychain, which reads the Docker config and
		// runs credential helpers (a test holds the rule, REQ-0210).
		remote.WithAuth(authn.Anonymous),
		remote.WithTransport(g),
		remote.WithContext(ctx),
		// One attempt: a retry of a refusal would be a second request to
		// a registry that said no, and the caller can rerun the command.
		remote.WithRetryBackoff(remote.Backoff{Steps: 1}),
		remote.WithRetryStatusCodes(),
		remote.WithJobs(1),
	}}
	f, err := r.fetch(w, p)
	if err != nil {
		return Fetched{}, r.classify(err)
	}
	return f, nil
}

// hostOf returns the host of a host or host:port.
func hostOf(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return hostport
}

// registry is one fetch from one repository.
type registry struct {
	host string // the registry, as the library names it, for messages
	ref  name.Digest
	// guard is the guard of the fetch, which knows the token endpoints.
	guard *guard
	opts  []remote.Option
}

// shapeError is the error of a document or a guard: upstream-shape. It
// is kept apart from a transport error until classify.
type shapeError struct{ reason string }

func (e *shapeError) Error() string { return e.reason }

// shape returns a shapeError.
func shape(format string, args ...any) error {
	return &shapeError{reason: fmt.Sprintf(format, args...)}
}

// classify gives err its row: a shape or a guard is upstream-shape, a
// transport failure or a status is registry-unreachable, and a
// descriptor refused by the grammar stays an error of ErrInvalid. The
// text of a registry's error body is never repeated.
func (r *registry) classify(err error) error {
	detail := cli.Detail{Key: "registry", Value: r.host}
	// upstream-shape names its source in the detail "upstream" (04 4.4).
	source := cli.Detail{Key: "upstream", Value: r.host}
	if se, ok := errors.AsType[*shapeError](err); ok {
		return cli.UpstreamShape.New("registry "+r.host+": "+se.reason, source)
	}
	for _, guard := range []error{ErrInsecure, ErrRedirect, ErrTooLarge, ErrDigestMismatch, ErrForeignRealm, ErrPrivateAddress} {
		if errors.Is(err, guard) {
			return cli.UpstreamShape.Wrap(err, "registry "+r.host+": a guard of the client refused the response", source)
		}
	}
	if errors.Is(err, ErrInvalid) {
		return fmt.Errorf("registry %s: %w", r.host, err)
	}
	if te, ok := errors.AsType[*transport.Error](err); ok {
		if te.StatusCode == http.StatusUnauthorized {
			return RegistryUnreachable.Wrap(ErrCredentials, "registry "+r.host+": it refuses an anonymous read of the repository", detail)
		}
		who := "it"
		if te.Request != nil && r.guard.isRealm(te.Request.URL) {
			who = "its token service"
		}
		return RegistryUnreachable.New(fmt.Sprintf("registry %s: %s answered %d", r.host, who, te.StatusCode), detail)
	}
	return RegistryUnreachable.Wrap(err, "registry "+r.host+" did not answer", detail)
}

// The documents of a registry are outputs of a tool the product does not
// own: decoded tolerantly, an unknown member skipped, and checked
// strictly, a member that is read being required and typed, a duplicate
// member name refused by the decoder (REQ-0170, ADR 0041). A pointer
// field is one that is required.
type indexDoc struct {
	SchemaVersion *int              `json:"schemaVersion"`
	MediaType     *string           `json:"mediaType"`
	Manifests     []entryDoc        `json:"manifests"`
	Annotations   map[string]string `json:"annotations"`
}

type entryDoc struct {
	MediaType *string      `json:"mediaType"`
	Digest    *string      `json:"digest"`
	Platform  *platformDoc `json:"platform"`
}

type platformDoc struct {
	OS           *string `json:"os"`
	Architecture *string `json:"architecture"`
	Variant      *string `json:"variant"`
}

type manifestDoc struct {
	SchemaVersion *int              `json:"schemaVersion"`
	MediaType     *string           `json:"mediaType"`
	Annotations   map[string]string `json:"annotations"`
}

// fetch walks the hops.
func (r *registry) fetch(w spec.Workload, p Platform) (Fetched, error) {
	body, ctype, err := r.manifest(w.Digest)
	if err != nil {
		return Fetched{}, err
	}
	digest := w.Digest
	switch ctype {
	case mediaOCIIndex, mediaDockerList:
		digest, body, ctype, err = r.platformManifest(body, ctype, p)
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
func (r *registry) platformManifest(body []byte, ctype string, p Platform) (digest string, out []byte, outType string, err error) {
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
	if _, ok := ix.Annotations[DescriptorAnnotation]; ok {
		return "", nil, "", shape("the index carries the annotation %s, which only a platform manifest does", DescriptorAnnotation)
	}
	var chosen *entryDoc
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
		if p.Variant != "" && (e.Platform.Variant == nil || *e.Platform.Variant != p.Variant) {
			continue
		}
		if *e.MediaType != mediaOCIManifest && *e.MediaType != mediaDockerManifest {
			continue
		}
		// The first match is what the image spec says to use, but an
		// index that lists two for one platform is an index whose
		// reading the product would be choosing: it fails closed.
		if chosen != nil {
			return "", nil, "", shape("the index lists more than one manifest for %s/%s", quote(p.OS), quote(p.Architecture))
		}
		chosen = &ix.Manifests[i]
	}
	if chosen == nil {
		return "", nil, "", shape("the index lists no manifest for %s/%s", quote(p.OS), quote(p.Architecture))
	}
	body, got, err := r.manifest(*chosen.Digest)
	if err != nil {
		return "", nil, "", err
	}
	return *chosen.Digest, body, got, nil
}

// sameType refuses a manifest with no mediaType member, and one whose
// member names a type other than the Content-Type it was served as
// (distribution spec: a client should reject a manifest whose mediaType
// does not match), so the header alone never picks what is read.
func sameType(member *string, ctype string) error {
	if member == nil {
		return shape("the document has no mediaType member")
	}
	if *member != ctype {
		return shape("the mediaType of the document is %s and it was served as %s", quote(*member), quote(ctype))
	}
	return nil
}

// manifest reads the manifest digest names, through the library, which
// verifies it again by that digest, and returns its body and its media
// type.
func (r *registry) manifest(digest string) ([]byte, string, error) {
	// The digest of an index entry is the registry's text and becomes
	// part of a URL: only the one form the product pins gets through.
	if !digestForm.MatchString(digest) {
		return nil, "", shape("a digest of the document is not sha256:<64 hex>: %s", quote(digest))
	}
	d, err := remote.Get(r.ref.Context().Digest(digest), r.opts...)
	if err != nil {
		return nil, "", err
	}
	ctype, _, err := mime.ParseMediaType(string(d.MediaType))
	if err != nil {
		return nil, "", shape("the response has no usable Content-Type")
	}
	return d.Manifest, ctype, nil
}
