// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package mise

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// pinnedMise returns the mise that the measurements of this package
// start: the one every mise command of tools/ci starts for this
// repository (Path), at the version ci.yml pins (CheckVersion), so a
// measurement is of that version whatever mise comes first on the
// search path.
func pinnedMise(t *testing.T) string {
	t.Helper()
	root := citest.ModuleRoot(t)
	require.NoError(t, CheckVersion(t.Context(), root), "the measurements are of the mise ci.yml pins")
	path, err := Path(root)
	require.NoError(t, err)
	return path
}

// writeWorkflow writes src as the pinned workflow of a new tree, and
// returns the tree.
func writeWorkflow(t *testing.T, src string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(PinnedWorkflow))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	return root
}

// TestReadPin reads the version and the sha256 of each platform's
// runner from the matrix (the Runners list of 10 10.2), and refuses a
// version or a sha256 of another shape, a platform with no runner and a
// runner the matrix does not list.
func TestReadPin(t *testing.T) {
	shas := map[string]string{
		"ubuntu-26.04":     strings.Repeat("1", 64),
		"ubuntu-26.04-arm": strings.Repeat("2", 64),
		"macos-26":         strings.Repeat("3", 64),
		"macos-26-intel":   strings.Repeat("a", 64),
	}
	workflow := citest.MiseWorkflow(t, "2026.10.3")
	for label, sha := range shas {
		workflow = strings.Replace(workflow, "- os: "+label+"\n            mise_sha256: "+citest.FakeMiseSha256, "- os: "+label+"\n            mise_sha256: "+sha, 1)
	}
	root := writeWorkflow(t, workflow)
	tests := []struct{ goos, goarch, label string }{
		{"linux", "amd64", "ubuntu-26.04"},
		{"linux", "arm64", "ubuntu-26.04-arm"},
		{"darwin", "arm64", "macos-26"},
		{"darwin", "amd64", "macos-26-intel"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"/"+tt.goarch, func(t *testing.T) {
			pin, err := readPin(root, tt.goos, tt.goarch)
			require.NoError(t, err)
			assert.Equal(t, Pin{Version: "2026.10.3", Sha256: shas[tt.label], Label: tt.label}, pin)
		})
	}
	t.Run("this platform", func(t *testing.T) {
		pin, err := ReadPin(root)
		require.NoError(t, err)
		assert.Equal(t, shas[pin.Label], pin.Sha256)
	})

	for _, platform := range [][2]string{{"windows", "amd64"}, {"linux", "386"}, {"freebsd", "arm64"}} {
		_, err := readPin(root, platform[0], platform[1])
		require.ErrorContains(t, err, "no runner of .github/workflows/ci.yml or .github/workflows/x64.yml is "+platform[0]+"/"+platform[1])
	}

	malformed := []struct{ name, workflow, want string }{
		{"a version of two numbers", citest.MiseWorkflow(t, `"2026.10"`), `.github/workflows/ci.yml pins the version of mise "2026.10", which is not three numbers joined by dots`},
		{"a version with a v", citest.MiseWorkflow(t, "v2026.10.3"), `"v2026.10.3", which is not three numbers`},
		{"a version with a suffix", citest.MiseWorkflow(t, "2026.10.3-rc1"), `"2026.10.3-rc1", which is not three numbers`},
		{"a version with a path", citest.MiseWorkflow(t, "2026.10.3/../x"), "which is not three numbers"},
		{"a sha256 in upper case", citest.PinnedMiseWorkflow(t, "2026.10.3", strings.Repeat("A", 64)), "is not 64 lowercase hex digits"},
		{"a short sha256", citest.PinnedMiseWorkflow(t, "2026.10.3", strings.Repeat("a", 63)), "is not 64 lowercase hex digits"},
		{"no sha256 for the runner", strings.Replace(citest.MiseWorkflow(t, "2026.10.3"), "- os: ubuntu-26.04\n", "- os: ubuntu-26.04-x\n", 1), "no matrix of .github/workflows/ci.yml gives a mise_sha256 for ubuntu-26.04"},
	}
	for _, tt := range malformed {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readPin(writeWorkflow(t, tt.workflow), "linux", "amd64")
			require.ErrorContains(t, err, tt.want)
		})
	}
	t.Run("two sha256 for one runner", func(t *testing.T) {
		workflow := strings.Replace(citest.MiseWorkflow(t, "2026.10.3"), "- os: ubuntu-26.04-arm\n", "- os: ubuntu-26.04\n", 1)
		workflow = strings.Replace(workflow, "mise_sha256: "+citest.FakeMiseSha256+"\n          - os: macos-26\n", "mise_sha256: "+strings.Repeat("b", 64)+"\n          - os: macos-26\n", 1)
		_, err := readPin(writeWorkflow(t, workflow), "linux", "amd64")
		require.ErrorContains(t, err, "two different mise_sha256 for ubuntu-26.04")
	})
	t.Run("no workflow", func(t *testing.T) {
		_, err := ReadPin(t.TempDir())
		require.ErrorContains(t, err, "the version of mise is pinned in .github/workflows/ci.yml")
	})
}

// TestPinAsset names the bare binary of mise's release for each of the
// four runners (10 10.2, REQ-1237).
func TestPinAsset(t *testing.T) {
	assets := map[string]string{
		"ubuntu-26.04":     "mise-v2026.10.3-linux-x64",
		"ubuntu-26.04-arm": "mise-v2026.10.3-linux-arm64",
		"macos-26":         "mise-v2026.10.3-macos-arm64",
		"macos-26-intel":   "mise-v2026.10.3-macos-x64",
	}
	for label, want := range assets {
		assert.Equal(t, want, Pin{Version: "2026.10.3", Label: label}.Asset(), label)
	}
}

// cachedMise installs script as the cached copy of the pin by hand,
// the way setup leaves it, in a tree whose workflow pins its sha256,
// and returns the tree and the copy's path.
func cachedMise(t *testing.T, script string) (string, string) {
	t.Helper()
	sum := sha256.Sum256([]byte(script))
	root := writeWorkflow(t, citest.PinnedMiseWorkflow(t, citest.FakeMiseVersion, hex.EncodeToString(sum[:])))
	path, err := CachePath(citest.FakeMiseVersion)
	require.NoError(t, err)
	dirs, err := CacheDirs(citest.FakeMiseVersion)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(dirs[0]), 0o700))
	for _, dir := range dirs {
		require.NoError(t, os.Mkdir(dir, 0o700))
	}
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
	return root, path
}

// TestCacheDirs holds the cache to the XDG Base Directory rule on every
// platform (REQ-1236): $XDG_CACHE_HOME when it is an absolute path, and
// otherwise $HOME/.cache, macOS included.
func TestCacheDirs(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	tail := filepath.Join("romeu-e-julieta", "mise", "2026.10.3")
	for _, tt := range []struct {
		name, home, xdg, want, err string
	}{
		{name: "an absolute XDG_CACHE_HOME", home: home, xdg: xdg, want: filepath.Join(xdg, tail)},
		{name: "no XDG_CACHE_HOME", home: home, want: filepath.Join(home, ".cache", tail)},
		{name: "a relative XDG_CACHE_HOME is ignored", home: home, xdg: "cache", want: filepath.Join(home, ".cache", tail)},
		{name: "no HOME and no XDG_CACHE_HOME", err: "HOME is not set"},
		{name: "no HOME, an absolute XDG_CACHE_HOME", xdg: xdg, want: filepath.Join(xdg, tail)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", tt.home)
			t.Setenv("XDG_CACHE_HOME", tt.xdg)
			dirs, err := CacheDirs("2026.10.3")
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, dirs[len(dirs)-1])
		})
	}
}

// TestPath selects the mise every mise command of tools/ci starts
// (REQ-1236): the copy setup installed while it passes every rule, and
// the mise on the search path otherwise. The copy is checked again on
// every call.
func TestPath(t *testing.T) {
	f := citest.NewFakeMise(t)
	log := filepath.Join(t.TempDir(), "log")
	script := "#!/bin/sh\necho \"cached $*\" >> '" + log + "'\n" + citest.AnswerVersion
	root, cached := cachedMise(t, script)
	onPath := filepath.Join(f.Bin, "mise")
	require.NoError(t, os.WriteFile(onPath, []byte("#!/bin/sh\necho 'path' >> '"+log+"'\necho '2026.9.15 linux-x64'\n"), 0o700))

	assert.Equal(t, filepath.Join(f.Home, ".cache", "romeu-e-julieta", "mise", citest.FakeMiseVersion, "mise"), cached, "below $HOME/.cache, on every platform")
	got, err := Path(root)
	require.NoError(t, err)
	assert.Equal(t, cached, got)
	require.NoError(t, CheckVersion(t.Context(), root), "mise --version starts the cached copy")
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Equal(t, "cached --version\n", string(data))

	t.Run("mise which starts the cached copy", func(t *testing.T) {
		require.NoError(t, os.Remove(log))
		lockRoot := citest.LockedRoot(t, citest.LockFixture)
		require.NoError(t, os.MkdirAll(filepath.Join(lockRoot, ".github", "workflows"), 0o755))
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(PinnedWorkflow)))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(lockRoot, filepath.FromSlash(PinnedWorkflow)), src, 0o600))
		_, err = WhichPinned(t.Context(), lockRoot, "go", "go", "go")
		require.Error(t, err, "the copy prints no path")
		data, err := os.ReadFile(log)
		require.NoError(t, err)
		assert.Equal(t, "cached -C "+lockRoot+" which go\n", string(data))
	})

	fallsBack := func(t *testing.T) {
		t.Helper()
		got, err := Path(root)
		require.NoError(t, err)
		assert.Equal(t, onPath, got, "the mise on the search path")
	}
	t.Run("a copy whose bytes changed", func(t *testing.T) {
		require.NoError(t, os.WriteFile(cached, []byte(script+"# changed\n"), 0o700))
		t.Cleanup(func() { require.NoError(t, os.WriteFile(cached, []byte(script), 0o700)) })
		fallsBack(t)
		require.NoError(t, os.Remove(log))
		require.ErrorContains(t, CheckVersion(t.Context(), root), "mise is at version 2026.9.15")
		data, err := os.ReadFile(log)
		require.NoError(t, err)
		assert.Equal(t, "path\n", string(data), "the changed copy never ran")
	})
	t.Run("a copy its owner cannot run", func(t *testing.T) {
		require.NoError(t, os.Chmod(cached, 0o600))
		t.Cleanup(func() { require.NoError(t, os.Chmod(cached, 0o700)) })
		fallsBack(t)
	})
	t.Run("a copy others can write", func(t *testing.T) {
		require.NoError(t, os.Chmod(cached, 0o722))
		t.Cleanup(func() { require.NoError(t, os.Chmod(cached, 0o700)) })
		fallsBack(t)
	})
	t.Run("a symbolic link at the cache path", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), "mise")
		require.NoError(t, os.WriteFile(other, []byte(script), 0o700))
		require.NoError(t, os.Rename(cached, cached+".kept"))
		require.NoError(t, os.Symlink(other, cached))
		t.Cleanup(func() {
			require.NoError(t, os.Remove(cached))
			require.NoError(t, os.Rename(cached+".kept", cached))
		})
		fallsBack(t)
	})
	t.Run("a cache directory others can write", func(t *testing.T) {
		dirs, err := CacheDirs(citest.FakeMiseVersion)
		require.NoError(t, err)
		for _, dir := range dirs {
			require.NoError(t, os.Chmod(dir, 0o777))
			fallsBack(t)
			require.NoError(t, os.Chmod(dir, 0o720))
			fallsBack(t)
			require.NoError(t, os.Chmod(dir, 0o700))
		}
	})
	t.Run("a cache directory that is a symbolic link", func(t *testing.T) {
		dirs, err := CacheDirs(citest.FakeMiseVersion)
		require.NoError(t, err)
		moved := filepath.Join(t.TempDir(), "moved")
		require.NoError(t, os.Rename(dirs[0], moved))
		require.NoError(t, os.Symlink(moved, dirs[0]))
		t.Cleanup(func() {
			require.NoError(t, os.Remove(dirs[0]))
			require.NoError(t, os.Rename(moved, dirs[0]))
		})
		fallsBack(t)
	})
	t.Run("a tree whose workflow pins no sha256", func(t *testing.T) {
		got, err := Path(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, onPath, got)
	})
	t.Run("no mise at all", func(t *testing.T) {
		require.NoError(t, os.Remove(onPath))
		t.Cleanup(func() {
			require.NoError(t, os.WriteFile(onPath, []byte("#!/bin/sh\necho '2026.9.15 linux-x64'\n"), 0o700))
		})
		_, err := Path(t.TempDir())
		require.ErrorIs(t, err, procgroup.ErrNotFound)
	})
	t.Run("the copy is found again once it is whole", func(t *testing.T) {
		got, err := Path(root)
		require.NoError(t, err)
		assert.Equal(t, cached, got)
	})
}

// TestCheckDir refuses a directory of the cache that is not one, is a
// symbolic link, or that others can write.
func TestCheckDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "d")
	require.NoError(t, os.Mkdir(dir, 0o700))
	require.NoError(t, CheckDir(dir))
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(dir, link))
	require.ErrorContains(t, CheckDir(link), "is a symbolic link")
	file := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	require.ErrorContains(t, CheckDir(file), "is not a directory")
	require.NoError(t, os.Chmod(dir, 0o702))
	require.ErrorContains(t, CheckDir(dir), "others can write")
	require.ErrorContains(t, CheckDir(filepath.Join(dir, "none")), "no such file")
}

// TestReadPinOfTheScheduledWorkflow reads the pins of a tree whose
// ci.yml names the arm64 runners alone and whose x64.yml names the x64
// ones (10 10.2, Runners): an x64 pin comes from x64.yml, the version
// is the one of ci.yml, which x64.yml repeats, and each error names the
// file that holds the culprit.
func TestReadPinOfTheScheduledWorkflow(t *testing.T) {
	sha := func(c string) string { return strings.Repeat(c, 64) }
	entry := func(label, sum string) string {
		return "          - os: " + label + "\n            mise_sha256: " + sum + "\n"
	}
	workflow := func(version string, entries ...string) string {
		return citest.Edits(t,
			"          sha256: ${{ matrix.mise_sha256 }}\n", "          version: "+version+"\n          sha256: ${{ matrix.mise_sha256 }}\n",
			"          - os: ubuntu-26.04\n            mise_sha256: aa\n", strings.Join(entries, ""))
	}
	arm := workflow("2026.10.3", entry("ubuntu-26.04-arm", sha("2")), entry("macos-26", sha("3")))
	x64 := workflow("2026.10.3", entry("ubuntu-26.04", sha("1")), entry("macos-26-intel", sha("a")))
	tree := func(t *testing.T, ci, scheduled string) string {
		t.Helper()
		root := writeWorkflow(t, ci)
		require.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(ScheduledWorkflow)), []byte(scheduled), 0o600))
		return root
	}

	t.Run("each runner from its file", func(t *testing.T) {
		root := tree(t, arm, x64)
		for platform, want := range map[[2]string]Pin{
			{"linux", "amd64"}:  {"2026.10.3", sha("1"), "ubuntu-26.04"},
			{"linux", "arm64"}:  {"2026.10.3", sha("2"), "ubuntu-26.04-arm"},
			{"darwin", "arm64"}: {"2026.10.3", sha("3"), "macos-26"},
			{"darwin", "amd64"}: {"2026.10.3", sha("a"), "macos-26-intel"},
		} {
			pin, err := readPin(root, platform[0], platform[1])
			require.NoError(t, err, want.Label)
			assert.Equal(t, want, pin)
		}
	})
	t.Run("no x64.yml pins no x64 runner", func(t *testing.T) {
		_, err := readPin(writeWorkflow(t, arm), "linux", "amd64")
		require.ErrorContains(t, err, "no matrix of .github/workflows/ci.yml gives a mise_sha256 for ubuntu-26.04")
		_, err = readPin(writeWorkflow(t, arm), "linux", "arm64")
		require.NoError(t, err, "an arm64 runner needs no x64.yml")
	})
	t.Run("an x64 runner missing from both files", func(t *testing.T) {
		_, err := readPin(tree(t, arm, workflow("2026.10.3", entry("macos-26-intel", sha("a")))), "linux", "amd64")
		require.ErrorContains(t, err, "no matrix of .github/workflows/ci.yml or .github/workflows/x64.yml gives a mise_sha256 for ubuntu-26.04")
	})
	t.Run("x64.yml pins another version", func(t *testing.T) {
		_, err := readPin(tree(t, arm, strings.Replace(x64, "version: 2026.10.3", "version: 2026.10.4", 1)), "linux", "arm64")
		require.ErrorContains(t, err, `.github/workflows/x64.yml pins mise "2026.10.4" and .github/workflows/ci.yml pins "2026.10.3"`)
	})
	t.Run("ci.yml pins no version", func(t *testing.T) {
		noStep := strings.Replace(arm, "          version: 2026.10.3\n", "", 1)
		_, err := readPin(tree(t, noStep, x64), "linux", "amd64")
		require.ErrorContains(t, err, `.github/workflows/ci.yml pins the version of mise with one "version" input`)
	})
	t.Run("one runner with two sha256 across the files", func(t *testing.T) {
		_, err := readPin(tree(t, arm+"", workflow("2026.10.3", entry("macos-26", sha("b")))), "darwin", "arm64")
		require.ErrorContains(t, err, "two different mise_sha256 for macos-26: "+sha("3")+" in .github/workflows/ci.yml, "+sha("b")+" in .github/workflows/x64.yml")
	})
	t.Run("a malformed sha256 in x64.yml", func(t *testing.T) {
		_, err := readPin(tree(t, arm, workflow("2026.10.3", entry("ubuntu-26.04", sha("A")))), "linux", "amd64")
		require.ErrorContains(t, err, "of ubuntu-26.04 in .github/workflows/x64.yml is not 64 lowercase hex digits")
	})
	t.Run("an x64.yml that is not YAML", func(t *testing.T) {
		_, err := readPin(tree(t, arm, "jobs: ["), "linux", "arm64")
		require.ErrorContains(t, err, ".github/workflows/x64.yml: ")
	})
}
