// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/canon"
)

// sampleSet is a widening set with an entry in every list.
func sampleSet() Set {
	return Set{
		Project: "shop",
		Repos: []Repo{
			{Dir: "shop-web", URL: "https://github.com/example/shop-web"},
			{Dir: "shop-api", URL: "https://github.com/example/shop-api", Primary: true, Ref: "main"},
		},
		Secrets: []Secret{
			{Name: "npm", Service: "npmjs.org", ArgvDigest: canon.Sum[canon.SecretArgvKind]{2}},
			{Name: "github", Service: "github.com", ArgvDigest: canon.Sum[canon.SecretArgvKind]{1}},
		},
		Workload: Workload{Repository: "docker.io/example/dev", Digest: "sha256:" + hex64('a'), Capabilities: Capabilities{"network": []any{"x.example"}}},
		Kits: []Kit{
			{ID: "julieta", Source: SourceProduct, ContentDigest: canon.Sum[canon.KitTreeKind]{3}},
			{ID: "tools", Source: SourcePersonal, ContentDigest: canon.Sum[canon.KitTreeKind]{4}, Args: map[string]string{"a": "b"}},
		},
		EgressGated:     []GatedDomain{{Domain: "z.example", Reason: ReasonSpecExtra}, {Domain: "github.com", Reason: ReasonUpload}},
		Ports:           []string{"9090:9090/tcp", "8080:8080/tcp"},
		Agent:           "claude",
		SalvageExcludes: []string{"node_modules", ".cache"},
		RenderDigest:    canon.Sum[canon.RenderKind]{5},
	}
}

// hex64 is 64 copies of the hexadecimal digit c.
func hex64(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestDigestSortsTheSortedListsAndKeepsTheKitOrder(t *testing.T) {
	t.Parallel()
	base, err := Digest(sampleSet())
	require.NoError(t, err)

	shuffled := sampleSet()
	shuffled.Repos[0], shuffled.Repos[1] = shuffled.Repos[1], shuffled.Repos[0]
	shuffled.Secrets[0], shuffled.Secrets[1] = shuffled.Secrets[1], shuffled.Secrets[0]
	shuffled.EgressGated[0], shuffled.EgressGated[1] = shuffled.EgressGated[1], shuffled.EgressGated[0]
	shuffled.Ports[0], shuffled.Ports[1] = shuffled.Ports[1], shuffled.Ports[0]
	shuffled.SalvageExcludes[0], shuffled.SalvageExcludes[1] = shuffled.SalvageExcludes[1], shuffled.SalvageExcludes[0]
	got, err := Digest(shuffled)
	require.NoError(t, err)
	assert.Equal(t, base, got, "the order of a sorted list is not part of the set")

	reordered := sampleSet()
	reordered.Kits[0], reordered.Kits[1] = reordered.Kits[1], reordered.Kits[0]
	got, err = Digest(reordered)
	require.NoError(t, err)
	assert.NotEqual(t, base, got, "the kits are ordered, so their order flips the digest")
}

func TestDigestTakesAnEmptyListAndNoListAlike(t *testing.T) {
	t.Parallel()
	empty := Set{Project: "shop", Repos: []Repo{}, Secrets: []Secret{}, Kits: []Kit{}, EgressGated: []GatedDomain{},
		Ports: []string{}, SalvageExcludes: []string{}, Workload: Workload{Capabilities: Capabilities{}}}
	none := Set{Project: "shop"}
	a, err := Digest(empty)
	require.NoError(t, err)
	b, err := Digest(none)
	require.NoError(t, err)
	assert.Equal(t, a, b)
}

func TestDigestDoesNotChangeItsInput(t *testing.T) {
	t.Parallel()
	s := sampleSet()
	_, err := Digest(s)
	require.NoError(t, err)
	assert.Equal(t, sampleSet(), s)
}

func TestDigestRefusesADuplicateKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*Set)
	}{
		{"two repos with one dir", func(s *Set) { s.Repos[1].Dir = s.Repos[0].Dir }},
		{"two secrets with one name", func(s *Set) { s.Secrets[1].Name = s.Secrets[0].Name }},
		{"two kits with one source and id", func(s *Set) { s.Kits[1] = s.Kits[0] }},
		{"one gated domain twice for one reason", func(s *Set) { s.EgressGated[1] = s.EgressGated[0] }},
		{"one port twice", func(s *Set) { s.Ports[1] = s.Ports[0] }},
		{"one exclude twice", func(s *Set) { s.SalvageExcludes[1] = s.SalvageExcludes[0] }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := sampleSet()
			tt.edit(&s)
			_, err := Digest(s)
			assert.Error(t, err)
		})
	}
}

func TestDigestRefusesAnUnknownKitSource(t *testing.T) {
	t.Parallel()
	s := sampleSet()
	s.Kits[0].Source = "vendor"
	_, err := Digest(s)
	assert.Error(t, err)
}

func TestKitTreeDigestIsTheSortedPathModeAndSHA256(t *testing.T) {
	t.Parallel()
	files := []File{
		{Path: "z.sh", Mode: ModeExecutable, Data: []byte("echo z\n")},
		{Path: "a.txt", Mode: ModeRegular, Data: []byte("a\n")},
	}
	got, err := KitTreeDigest(files)
	require.NoError(t, err)

	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	canonical := `[{"mode":"100644","path":"a.txt","sha256":"` + sum("a\n") + `"},{"mode":"100755","path":"z.sh","sha256":"` + sum("echo z\n") + `"}]`
	want := sha256.Sum256([]byte("romeu/kit-tree/v1\x00" + canonical))
	assert.Equal(t, want, [32]byte(got))
}

func TestKitTreeDigestRefusesWhatAKitCannotHold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files []File
	}{
		{"a symbolic link", []File{{Path: "l", Mode: "120000"}}},
		{"a directory", []File{{Path: "d", Mode: "040000"}}},
		{"an empty path", []File{{Path: "", Mode: ModeRegular}}},
		{"one path twice", []File{{Path: "a", Mode: ModeRegular}, {Path: "a", Mode: ModeExecutable}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := KitTreeDigest(tt.files)
			assert.Error(t, err)
		})
	}
}

func TestSecretArgvDigestIsTheDigestOfTheArgv(t *testing.T) {
	t.Parallel()
	got, err := SecretArgvDigest([]string{"/usr/bin/env", "-i", "gh", "auth", "token"})
	require.NoError(t, err)
	want := sha256.Sum256([]byte(`romeu/secret-argv/v1` + "\x00" + `["/usr/bin/env","-i","gh","auth","token"]`))
	assert.Equal(t, want, [32]byte(got))

	_, err = SecretArgvDigest(nil)
	assert.Error(t, err, "a secret has a command")
}
