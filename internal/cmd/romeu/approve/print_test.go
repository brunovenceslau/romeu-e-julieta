// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build unix

package approve

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli"
	"github.com/brunovenceslau/romeu-e-julieta/internal/cli/testsupport"
	"github.com/brunovenceslau/romeu-e-julieta/internal/state"
	"github.com/brunovenceslau/romeu-e-julieta/internal/termsafe"
)

// text returns Text for rec and live, with no project.
func text(t *testing.T, rec *state.Toolchain, live Live) string {
	t.Helper()
	s, err := Text(rec, live, nil)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(s, Question), "the question comes last: %q", s)
	return s
}

// TestTextOneLine: with julieta and the catalog unchanged the prompt is
// one line, a dev build's included (REQ-0065), and it names the tested
// window (REQ-0933).
//
//romeu:invariant I7 test
func TestTextOneLine(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)
	live.Romeu.Version = "v0.0.0-dev+aaaa"
	rec := recordOf(t, live)
	live.Romeu = state.Binary{Version: "v0.0.0-dev+bbbb", SHA256: state.SHA256{0xbb}}

	got := text(t, rec, live)
	assert.NotContains(t, got, "\n")
	assert.Equal(t, "gate 1: sbx 0.49.2 (newest recorded 0.49.2; tested window 0.49.2), romeu v0.0.0-dev+aaaa -> v0.0.0-dev+bbbb, "+
		"sha256 010000000000 -> bb0000000000; julieta and the catalog are unchanged; "+Question, got)
}

// TestTextSbx: the prompt prints the live sbx version beside the
// recorded one and the newest recorded, says when it is outside the
// tested window, and marks a lower one downgrade (01 1.4, I7).
//
//romeu:invariant I7 test
func TestTextSbx(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, recorded, live, want string
	}{
		{"a newer sbx", "0.49.2", "0.50.0", "sbx 0.49.2 -> 0.50.0 (newest recorded 0.49.2; tested window 0.49.2; 0.50.0 is outside it, untested)"},
		{"a downgrade", "0.50.0", "0.49.2", "sbx 0.50.0 -> 0.49.2 downgrade (newest recorded 0.49.2; tested window 0.49.2)"},
		{"the same sbx", "0.49.2", "0.49.2", "sbx 0.49.2 (newest recorded 0.49.2; tested window 0.49.2)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			live := sampleLive(t)
			live.Sbx = tt.recorded
			rec := recordOf(t, live)
			live.Sbx = tt.live
			assert.Contains(t, text(t, rec, live), "gate 1: "+tt.want+", romeu ")
		})
	}
}

// TestTextDowngrade: downgrade beside each version lower than the
// recorded one, julieta's protocol included (I7, 05 5.2 case 6).
//
//romeu:invariant I7 test
func TestTextDowngrade(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)
	live.Romeu.Version, live.Julieta.Version, live.Julieta.Protocol = "v1.1.0", "v1.1.0", 2
	rec := recordOf(t, live)
	live.Romeu.Version, live.Julieta.Version, live.Julieta.Protocol = "v1.0.0", "v1.0.0", 1

	got := text(t, rec, live)
	assert.Contains(t, got, "\n  romeu v1.1.0 -> v1.0.0 downgrade, sha256 010000000000\n")
	assert.Contains(t, got, "\n  julieta v1.1.0 -> v1.0.0 downgrade, protocol 2 -> 1 downgrade, sha256 020000000000\n")

	live.Romeu.Version = "v1.2.0"
	assert.Contains(t, text(t, rec, live), "romeu v1.1.0 -> v1.2.0, sha256", "a newer version is no downgrade")
}

// TestTextJulieta: a julieta change prints the version change on its own
// line, and the catalog as unchanged.
func TestTextJulieta(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)
	rec := recordOf(t, live)
	live.Julieta.Version, live.Julieta.SHA256 = "v1.1.0", state.SHA256{0xcc}

	got := text(t, rec, live)
	assert.Equal(t, "gate 1: the toolchain of this machine\n"+
		"  sbx 0.49.2 (newest recorded 0.49.2; tested window 0.49.2)\n"+
		"  romeu v1.0.0, sha256 010000000000\n"+
		"  julieta v1.0.0 -> v1.1.0, protocol 1, sha256 020000000000 -> cc0000000000\n"+
		"  catalog "+live.CatalogDigest.String()[:12]+", unchanged\n"+Question, got)
}

// TestTextCatalogDiff: the catalog diff names the domains added and
// removed per backend:tool and the upload flags changed, each with the
// projects it affects (01 1.4, REQ-0061; 07 7.6, REQ-0638).
func TestTextCatalogDiff(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)
	rec := recordOf(t, live)
	next := DecodeCatalog(fixtureCatalog(t, "schema: catalog/v1\n"+
		"domains:\n  dl.example: {upload: true}\n  new.example: {upload: true}\n  node.example: {upload: false}\n"+
		"tools:\n  core:go: [dl.example, new.example]\n  aqua:jqlang/jq: [node.example]\n  core:node: [node.example]\n"))
	live.Catalog = next
	sum, err := canon.Digest(canon.Catalog, next)
	require.NoError(t, err)
	live.CatalogDigest = sum
	projects := map[string][]string{
		"shop": {"git.example", "dl.example"},
		"web":  {"git.example", "up.example"},
		"blog": {"git.example"},
	}

	got, err := Text(rec, live, projects)
	require.NoError(t, err)
	_, diff, ok := strings.Cut(got, "\n  catalog "+rec.CatalogDigest.String()[:12]+" -> "+sum.String()[:12]+"\n")
	require.True(t, ok, got)
	assert.Equal(t, ""+
		"    tool aqua:jqlang/jq: + node.example; projects: blog, shop, web\n"+
		"    tool core:go: + new.example (upload), - up.example; projects: shop, web\n"+
		"    tool core:node (new): + node.example; projects: blog, shop, web\n"+
		"    upload flag of dl.example: false -> true; projects: shop\n"+
		Question, diff)
}

// TestTextFirstAcknowledgement: with no record every member is new, the
// catalog is listed whole, and the meta and base sets affect every
// project.
func TestTextFirstAcknowledgement(t *testing.T) {
	t.Parallel()
	got, err := Text(nil, sampleLive(t), map[string][]string{"shop": nil})
	require.NoError(t, err)
	assert.Contains(t, got, "gate 1: the toolchain of this machine\n"+
		"  sbx 0.49.2 (none recorded) (newest recorded 0.49.2; tested window 0.49.2)\n"+
		"  romeu v1.0.0 (none recorded), sha256 010000000000 (none recorded)\n"+
		"  julieta v1.0.0 (none recorded), protocol 1 (none recorded), sha256 020000000000 (none recorded)\n")
	assert.Contains(t, got, "    meta: + mise.example; projects: shop\n")
	assert.Contains(t, got, "    base: + git.example (upload); projects: shop\n")
	assert.Contains(t, got, "    tool aqua:jqlang/jq (new): no domain; projects: shop\n")
	assert.Contains(t, got, "    tool core:go (new): + dl.example, + up.example (upload); projects: shop\n")
}

// TestTextAffectsAProjectByAnAddedDomain: a project whose applied
// egress holds only a domain the tool gains is affected too.
func TestTextAffectsAProjectByAnAddedDomain(t *testing.T) {
	t.Parallel()
	old := DecodeCatalog(fixtureCatalog(t, sampleFamily))
	next := DecodeCatalog(fixtureCatalog(t, "schema: catalog/v1\n"+
		"domains:\n  dl.example: {upload: false}\n  up.example: {upload: true}\n  new.example: {upload: false}\n"+
		"tools:\n  core:go: [dl.example, up.example, new.example]\n  aqua:jqlang/jq: []\n"))
	got := catalogDiff(old, next, map[string][]string{"fresh": {"new.example"}, "other": {"x.example"}})
	assert.Equal(t, []string{"tool core:go: + new.example; projects: fresh"}, got)
}

// TestTextUnreadableCatalog: a record whose stored catalog does not
// decode is record-unreadable (RJ-334), never a bare error.
func TestTextUnreadableCatalog(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)
	rec := recordOf(t, live)
	rec.Catalog = state.Catalog{"tools": "not a map"}
	live.CatalogDigest = canon.Sum[canon.CatalogKind]{9}
	_, err := Text(rec, live, nil)
	testsupport.RequireID(t, "RJ-334", err)
}

// TestTextEscapes: a control character in a version, a catalog domain or
// a project name comes out escaped (I29), from Text itself and through
// the CLI writer, which leaves Text's output as it is.
//
//romeu:invariant I29 test
func TestTextEscapes(t *testing.T) {
	t.Parallel()
	live := sampleLive(t)
	rec := recordOf(t, live)
	live.Romeu.Version = "v1.1.0\x1b[2J\nforged line"
	live.Julieta.Version = "v1.1.0\a"
	live.Catalog.Tools["core:go"] = []string{"dl.example", "up.example", "bad\x1b]0;x\x07.example"}
	live.CatalogDigest = canon.Sum[canon.CatalogKind]{9}
	projects := map[string][]string{"shop\x1b[31m": {"dl.example"}}

	got, err := Text(rec, live, projects)
	require.NoError(t, err)
	for _, r := range strings.TrimSuffix(got, Question) {
		if r != '\n' {
			require.False(t, r < 0x20 || r == 0x7f, "a raw control %q in %q", r, got)
		}
	}
	assert.NotContains(t, got, "\nforged line", "a line feed in a value does not start a line")
	assert.Contains(t, got, termsafe.Escape("shop\x1b[31m"))

	var through strings.Builder
	_, err = cli.NewWriter(&through).Write([]byte(got))
	require.NoError(t, err)
	assert.Equal(t, got, through.String())
}
