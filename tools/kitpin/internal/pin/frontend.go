// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// frontendRepo is the repository of the kit frontend on Docker Hub.
const frontendRepo = "docker/sandbox-kit"

// milestone is the form of a milestone tag of the v3 frontend (06
// 6.1).
var milestone = regexp.MustCompile(`^3\.0\.0-m\.(0|[1-9][0-9]*)$`)

// indexTypes are the media types a frontend pin may name: an image
// index, in the OCI and the Docker forms. A single image manifest is
// refused, since it would serve one platform and the sandboxes run on
// both.
var indexTypes = []string{
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
}

// platforms are the platforms the pinned index must list.
var platforms = []string{"linux/amd64", "linux/arm64"}

// maxPages bounds the pages of the tag list a run follows.
const maxPages = 100

// pinFrontend pins the highest milestone tag of the frontend with the
// digest of its manifest, computed from the bytes the registry serves.
func pinFrontend(ctx context.Context, e Env, _ *writer, p *pins.Pins) (string, error) {
	token, err := pullToken(ctx, e)
	if err != nil {
		return "", err
	}
	auth := map[string]string{"Authorization": "Bearer " + token}
	tags, err := listTags(ctx, e, auth)
	if err != nil {
		return "", err
	}
	tag, err := highestMilestone(tags)
	if err != nil {
		return "", err
	}
	headers := map[string]string{"Accept": strings.Join(indexTypes, ", ")}
	for k, v := range auth {
		headers[k] = v
	}
	body, h, err := fetch(ctx, e, e.Registry, "/v2/"+frontendRepo+"/manifests/"+tag, headers)
	if err != nil {
		return "", err
	}
	media, _, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil || !slices.Contains(indexTypes, media) {
		return "", fmt.Errorf("the manifest of %s:%s has the type %q, not an image index", frontendRepo, tag, h.Get("Content-Type"))
	}
	if err := coversPlatforms(body); err != nil {
		return "", fmt.Errorf("the index of %s:%s: %w", frontendRepo, tag, err)
	}
	sum := sha256.Sum256(body)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if served := h.Get("Docker-Content-Digest"); served != "" && served != digest {
		return "", fmt.Errorf("the registry names the manifest of %s:%s %s, but its bytes hash to %s", frontendRepo, tag, served, digest)
	}
	p.Frontend = &pins.Frontend{Ref: frontendRepo + ":" + tag + "@" + digest}
	return "frontend: " + p.Frontend.Ref, nil
}

// pullToken returns an anonymous pull token for the frontend's
// repository.
func pullToken(ctx context.Context, e Env) (string, error) {
	q := url.Values{"service": {"registry.docker.io"}, "scope": {"repository:" + frontendRepo + ":pull"}}
	body, _, err := fetch(ctx, e, e.Auth, "?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	var t struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &t); err != nil {
		return "", fmt.Errorf("the token response: %w", err)
	}
	if t.Token == "" {
		return "", errors.New("the token response holds no token")
	}
	return t.Token, nil
}

// listTags returns every tag of the frontend's repository, following
// the Link header of each page.
func listTags(ctx context.Context, e Env, auth map[string]string) ([]string, error) {
	var tags []string
	next := "/v2/" + frontendRepo + "/tags/list?n=1000"
	for range maxPages {
		body, h, err := fetch(ctx, e, e.Registry, next, auth)
		if err != nil {
			return nil, err
		}
		var page struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("the tag list: %w", err)
		}
		tags = append(tags, page.Tags...)
		next = nextLink(h.Get("Link"))
		if next == "" {
			return tags, nil
		}
	}
	return nil, fmt.Errorf("the tag list has more than %d pages", maxPages)
}

// nextLink returns the path of the rel="next" link of a Link header,
// "" when there is none or it names another host.
func nextLink(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		target, params, ok := strings.Cut(strings.TrimSpace(part), ";")
		if !ok || !strings.Contains(strings.ReplaceAll(params, " ", ""), `rel="next"`) {
			continue
		}
		target = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(target), "<"), ">")
		if strings.HasPrefix(target, "/v2/") {
			return target
		}
	}
	return ""
}

// highestMilestone returns the milestone tag with the highest <N>.
func highestMilestone(tags []string) (string, error) {
	best, bestN := "", -1
	for _, t := range tags {
		m := milestone.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > bestN {
			best, bestN = t, n
		}
	}
	if best == "" {
		return "", fmt.Errorf("%s has no tag 3.0.0-m.<N>", frontendRepo)
	}
	return best, nil
}

// coversPlatforms returns an error unless the index lists a manifest
// for each of platforms.
func coversPlatforms(index []byte) error {
	var idx struct {
		Manifests []struct {
			Platform *struct {
				OS           string `json:"os"`
				Architecture string `json:"architecture"`
			} `json:"platform"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(index, &idx); err != nil {
		return err
	}
	var listed []string
	for _, m := range idx.Manifests {
		if m.Platform != nil {
			listed = append(listed, m.Platform.OS+"/"+m.Platform.Architecture)
		}
	}
	for _, want := range platforms {
		if !slices.Contains(listed, want) {
			return fmt.Errorf("it lists no manifest for %s", want)
		}
	}
	return nil
}
