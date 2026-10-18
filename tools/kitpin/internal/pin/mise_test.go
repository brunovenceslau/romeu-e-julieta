// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package pin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/minisign"
	"github.com/brunovenceslau/romeu-e-julieta/tools/kitpin/pins"
)

const (
	miseSums = "mise/2026.10.3/SHASUMS256.txt"
	miseSig  = "mise/2026.10.3/SHASUMS256.txt.minisig"
	misePub  = "mise/minisign.pub"
	testPub  = "test/minisign.pub"
)

// miseRelease serves a checksum file and a signature as the release
// 2026.10.3, from the fixtures at the two paths.
func miseRelease(t *testing.T, sums, sig string) map[string]route {
	t.Helper()
	return map[string]route{
		"/mise/v2026.10.3/SHASUMS256.txt":         {body: fixture(t, sums)},
		"/mise/v2026.10.3/SHASUMS256.txt.minisig": {body: fixture(t, sig)},
	}
}

// TestMiseGoodSignature is the acceptance case: with mise's key and
// mise's own files, the version and both linux hashes are pinned, the
// two files are committed as downloaded, and the other entries stay.
func TestMiseGoodSignature(t *testing.T) {
	h := newHarness(t, miseRelease(t, miseSums, miseSig))
	h.repo.Write("kits/mise-minisign.pub", string(fixture(t, misePub)))
	h.repo.Write("kits/pins.yaml", startPins)
	require.Equal(t, exitOK, h.run(t, "mise", "2026.10.3"), h.stderr.String())
	pins := h.read(t, "kits/pins.yaml")
	assert.Contains(t, pins, `mise:
  version: 2026.10.3
  sha256:
    linux-amd64: 8d48bc510b7d844fad0bc7156c0855c2a1e7230c51e498a7f6b63b021f8b57c5
    linux-arm64: 357260e28904569a6e7124d33b65cef6d043846c07bb8f4906ddf22cc353d61d
`)
	assert.Contains(t, pins, startPins)
	assert.Equal(t, string(fixture(t, miseSums)), h.read(t, "kits/mise/SHASUMS256.txt"))
	assert.Equal(t, string(fixture(t, miseSig)), h.read(t, "kits/mise/SHASUMS256.txt.minisig"))
	assert.Contains(t, h.stdout.String(), "signed by key 64113EDF160FDEC2")
}

// TestMiseRefuses holds the files the command refuses: nothing is
// written, neither the pin nor the two files.
func TestMiseRefuses(t *testing.T) {
	cases := []struct {
		name, key, sums, sig, want string
	}{
		{"changed_byte", misePub, "mise/changed/SHASUMS256.txt", "mise/changed/SHASUMS256.txt.minisig", "verification failed"},
		{"signed_with_another_key", misePub, miseSums, "test/signed/SHASUMS256.txt.minisig", "made with the key"},
		{"signed_with_another_key_under_the_key_id", misePub, miseSums, "test/forged/SHASUMS256.txt.minisig", "verification failed"},
		{"legacy_mode", testPub, miseSums, "test/legacy/SHASUMS256.txt.minisig", "legacy mode"},
		{"changed_trusted_comment", testPub, miseSums, "test/comment/SHASUMS256.txt.minisig", "trusted comment's verification failed"},
		{"trusted_comment_names_another_file", testPub, miseSums, "test/wrongfile/SHASUMS256.txt.minisig", `does not name file:SHASUMS256.txt`},
		{"lacks_an_architecture", testPub, "test/noarch/SHASUMS256.txt", "test/noarch/SHASUMS256.txt.minisig", "SHASUMS256.txt lists no mise-v2026.10.3-linux-arm64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, miseRelease(t, c.sums, c.sig))
			h.repo.Write("kits/mise-minisign.pub", string(fixture(t, c.key)))
			h.repo.Write("kits/pins.yaml", startPins)
			assert.Equal(t, exitError, h.run(t, "mise", "2026.10.3"))
			assert.Contains(t, h.stderr.String(), c.want)
			assert.Equal(t, startPins, h.read(t, "kits/pins.yaml"))
			assert.False(t, h.exists("kits/mise/SHASUMS256.txt"))
			assert.False(t, h.exists("kits/mise/SHASUMS256.txt.minisig"))
		})
	}
}

// TestMiseGoodSignatureOfTheTestKey shows that the case that lacks an
// architecture fails on the architecture, not on the signature: the
// same key verifies mise's full file.
func TestMiseGoodSignatureOfTheTestKey(t *testing.T) {
	h := newHarness(t, miseRelease(t, miseSums, "test/signed/SHASUMS256.txt.minisig"))
	h.repo.Write("kits/mise-minisign.pub", string(fixture(t, testPub)))
	require.Equal(t, exitOK, h.run(t, "mise", "2026.10.3"), h.stderr.String())
}

// TestMiseMissingKey: without the committed key the command stops
// before it downloads anything.
func TestMiseMissingKey(t *testing.T) {
	h := newHarness(t, miseRelease(t, miseSums, miseSig))
	assert.Equal(t, exitError, h.run(t, "mise", "2026.10.3"))
	assert.Contains(t, h.stderr.String(), "kits/mise-minisign.pub is missing")
	assert.Equal(t, int32(0), h.requests.Load())
	assert.False(t, h.exists("kits/pins.yaml"))
}

func TestMiseRefusesAKeyThatIsNotOne(t *testing.T) {
	h := newHarness(t, miseRelease(t, miseSums, miseSig))
	h.repo.Write("kits/mise-minisign.pub", "untrusted comment: x\nnot a key\n")
	assert.Equal(t, exitError, h.run(t, "mise", "2026.10.3"))
	assert.Contains(t, h.stderr.String(), "kits/mise-minisign.pub: public key")
	assert.Equal(t, int32(0), h.requests.Load())
}

func TestMiseMissingRelease(t *testing.T) {
	h := newHarness(t, nil)
	h.repo.Write("kits/mise-minisign.pub", string(fixture(t, misePub)))
	assert.Equal(t, exitError, h.run(t, "mise", "2026.10.3"))
	assert.Contains(t, h.stderr.String(), "404")
	assert.False(t, h.exists("kits/pins.yaml"))
}

func TestMiseRefusesAVersionThatIsNotOne(t *testing.T) {
	cases := []struct{ name, version string }{
		{"latest", "latest"},
		{"v_prefix", "v2026.10.3"},
		{"two_parts", "2026.10"},
		{"dot_dot", "../2026.10.3"},
		{"slash_suffix", "2026.10.3/x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t, nil)
			h.repo.Write("kits/mise-minisign.pub", string(fixture(t, misePub)))
			assert.Equal(t, exitError, h.run(t, "mise", c.version))
			assert.Contains(t, h.stderr.String(), "is not three numbers")
			assert.Equal(t, int32(0), h.requests.Load())
		})
	}
}

func TestMiseHashesRefuses(t *testing.T) {
	sum := "8d48bc510b7d844fad0bc7156c0855c2a1e7230c51e498a7f6b63b021f8b57c5"
	cases := []struct{ name, sums, want string }{
		{"listed_twice", sum + "  ./mise-v1.2.3-linux-x64\n" + sum + "  ./mise-v1.2.3-linux-x64\n" + sum + "  ./mise-v1.2.3-linux-arm64\n", "lists mise-v1.2.3-linux-x64 twice"},
		{"not_a_hash", "XYZ  ./mise-v1.2.3-linux-x64\n" + sum + "  ./mise-v1.2.3-linux-arm64\n", `"XYZ"`},
		{"lacks_amd64", sum + "  ./mise-v1.2.3-linux-arm64\n" + sum + "  ./mise-v1.2.3-linux-x64-musl\n", "lists no mise-v1.2.3-linux-x64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := miseHashes([]byte(c.sums), "1.2.3")
			require.ErrorContains(t, err, c.want)
		})
	}
}

// TestMiseRefusesALowerVersion: the pin only moves forward, checked
// before any request; the pinned version itself may be read again.
func TestMiseRefusesALowerVersion(t *testing.T) {
	pinned := "mise:\n  version: 2026.10.4\n  sha256:\n    linux-amd64: " + strings.Repeat("a", 64) + "\n    linux-arm64: " + strings.Repeat("b", 64) + "\n"
	h := newHarness(t, miseRelease(t, miseSums, miseSig))
	h.repo.Write("kits/mise-minisign.pub", string(fixture(t, misePub)))
	h.repo.Write("kits/pins.yaml", pinned)
	assert.Equal(t, exitError, h.run(t, "mise", "2026.10.3"))
	assert.Contains(t, h.stderr.String(), "the version 2026.10.3 is lower than the pinned 2026.10.4")
	assert.Equal(t, int32(0), h.requests.Load())
	assert.Equal(t, pinned, h.read(t, "kits/pins.yaml"))

	h.repo.Write("kits/pins.yaml", strings.Replace(pinned, "2026.10.4", "2026.10.3", 1))
	require.Equal(t, exitOK, h.run(t, "mise", "2026.10.3"), h.stderr.String())
}

func TestOlder(t *testing.T) {
	assert.True(t, older("2026.10.3", "2026.10.4"))
	assert.True(t, older("2026.9.30", "2026.10.0"))
	assert.False(t, older("2026.10.3", "2026.10.3"))
	assert.False(t, older("2026.10.10", "2026.10.9"))
}

// TestMiseLeavesNoTemporaryFile: each file is written through a
// temporary file that the rename removes.
func TestMiseLeavesNoTemporaryFile(t *testing.T) {
	h := newHarness(t, miseRelease(t, miseSums, miseSig))
	h.repo.Write("kits/mise-minisign.pub", string(fixture(t, misePub)))
	require.Equal(t, exitOK, h.run(t, "mise", "2026.10.3"), h.stderr.String())
	for _, dir := range []string{"kits", "kits/mise"} {
		entries, err := os.ReadDir(filepath.Join(h.repo.Dir, dir))
		require.NoError(t, err)
		for _, e := range entries {
			assert.NotContains(t, e.Name(), ".kitpin-")
		}
	}
}

// TestCommittedMiseFilesVerify reads the files of the repository
// itself: kits/mise/SHASUMS256.txt verifies against its signature with
// kits/mise-minisign.pub, names SHASUMS256.txt in its trusted comment,
// and holds the two hashes kits/pins.yaml pins for mise.
func TestCommittedMiseFilesVerify(t *testing.T) {
	top := filepath.Join("..", "..", "..", "..")
	read := func(path string) []byte {
		b, err := os.ReadFile(filepath.Join(top, filepath.FromSlash(path)))
		require.NoError(t, err)
		return b
	}
	key, err := minisign.ParsePublicKey(read(miseKeyPath))
	require.NoError(t, err)
	sums := read(miseSumsPath)
	trusted, err := minisign.Verify(key, sums, read(miseSigPath))
	require.NoError(t, err)
	assert.Contains(t, strings.Split(trusted, "\t"), "file:SHASUMS256.txt")
	p, err := pins.Parse(read(pins.Path))
	require.NoError(t, err)
	require.NotNil(t, p.Mise)
	amd64, arm64, err := miseHashes(sums, p.Mise.Version)
	require.NoError(t, err)
	assert.Equal(t, pins.Arches{Amd64: amd64, Arm64: arm64}, p.Mise.SHA256)
}
