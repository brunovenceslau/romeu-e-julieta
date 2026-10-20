// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

// herdrRelease is what pinHerdr reads of a release of the GitHub API.
type herdrRelease struct {
	TagName   string `json:"tag_name"`
	Immutable *bool  `json:"immutable"`
	Assets    []struct {
		Name   string  `json:"name"`
		Digest *string `json:"digest"`
	} `json:"assets"`
}

// The assets of a herdr release that the pin names, by architecture.
const (
	herdrAmd64 = "herdr-linux-x86_64"
	herdrArm64 = "herdr-linux-aarch64"
)

// pinHerdr takes each hash from the release API's digest field and
// refuses a release that is not marked immutable (06 6.4).
func pinHerdr(ctx context.Context, e Env, p *pins.Pins, version string) (string, error) {
	if !pins.Version.MatchString(version) {
		return "", fmt.Errorf("the version %q is not three numbers, as 0.9.5", version)
	}
	body, _, err := fetch(ctx, e, e.GitHub, "/repos/herdrdev/herdr/releases/tags/v"+version, map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	})
	if err != nil {
		return "", err
	}
	var rel herdrRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", fmt.Errorf("the release v%s: %w", version, err)
	}
	if rel.TagName != "v"+version {
		return "", fmt.Errorf("the API answered with the release %q, not v%s", rel.TagName, version)
	}
	switch {
	case rel.Immutable == nil:
		return "", fmt.Errorf("the release v%s has no field immutable, so it is not marked immutable", version)
	case !*rel.Immutable:
		return "", fmt.Errorf("the release v%s is not marked immutable (immutable is false)", version)
	}
	sums := map[string]string{}
	for _, a := range rel.Assets {
		if a.Name != herdrAmd64 && a.Name != herdrArm64 {
			continue
		}
		if _, dup := sums[a.Name]; dup {
			return "", fmt.Errorf("the release v%s has two assets %s", version, a.Name)
		}
		if a.Digest == nil {
			return "", fmt.Errorf("the release v%s: assets[%s].digest is missing", version, a.Name)
		}
		sum, ok := strings.CutPrefix(*a.Digest, "sha256:")
		if !ok || !pins.SHA256(sum) {
			return "", fmt.Errorf("the release v%s: assets[%s].digest %q is not sha256:<64 hex>", version, a.Name, *a.Digest)
		}
		sums[a.Name] = sum
	}
	for _, name := range []string{herdrAmd64, herdrArm64} {
		if _, ok := sums[name]; !ok {
			return "", fmt.Errorf("the release v%s has no asset %s", version, name)
		}
	}
	p.Herdr = &pins.Release{Version: version, SHA256: pins.Arches{Amd64: sums[herdrAmd64], Arm64: sums[herdrArm64]}}
	return fmt.Sprintf("herdr %s: immutable release; linux-amd64 %s, linux-arm64 %s", version, sums[herdrAmd64], sums[herdrArm64]), nil
}
