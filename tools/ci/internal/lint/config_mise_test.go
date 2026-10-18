// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package lint

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
)

// allowedGoModDirectives are the directives go.mod may hold, the ones it
// holds today. Every other one is a decision to review here first: an
// "ignore" takes a directory out of "./...", and so out of the lint and
// the unit steps, and a "replace" or a "tool" changes the code that is
// built.
var allowedGoModDirectives = []string{"module", "go", "require"}

// goModDirectives returns the directives of a go.mod file, each one
// time it starts a line or a block, read as the go command reads the
// file (golang.org/x/mod/modfile, which the go command vendors as
// cmd/vendor/golang.org/x/mod/modfile): a directive is the first word
// of a line, comments start with "//", and a line that ends with "("
// opens a block whose lines are its entries, up to the line ")".
func goModDirectives(data []byte) []string {
	var found []string
	inBlock := false
	for line := range strings.SplitSeq(string(data), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case inBlock:
			inBlock = line != ")"
		default:
			found = append(found, strings.Fields(line)[0])
			inBlock = strings.HasSuffix(line, "(")
		}
	}
	return found
}

// TestGoModDirectives pins goModDirectives on the forms of a directive.
func TestGoModDirectives(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"line", "module m\n\ngo 1.27.0\n", []string{"module", "go"}},
		{"block", "module m\n\nrequire (\n\ta v1.0.0\n\tb v1.0.0 // indirect\n)\n", []string{"module", "require"}},
		{"ignore line", "module m\n\nignore ./esc\n", []string{"module", "ignore"}},
		{"ignore block", "module m\n\nignore (\n\t./esc\n)\n", []string{"module", "ignore"}},
		{"after a block", "require (\n\ta v1.0.0\n)\nreplace a => ./a\n", []string{"require", "replace"}},
		{"indented, with a comment", "// about it\n  tool x // the tool\n", []string{"tool"}},
		{"CRLF line ends", "module m\r\nignore ./esc\r\n", []string{"module", "ignore"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, goModDirectives([]byte(tt.src)))
		})
	}
}

// TestOnlyAllowedGoModDirectives checks that go.mod holds no directive
// outside allowedGoModDirectives: an "ignore" there takes a directory
// out of "./...", and the linter never reads it.
func TestOnlyAllowedGoModDirectives(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "go.mod"))
	require.NoError(t, err)
	found := goModDirectives(data)
	require.NotEmpty(t, found, "the directives of go.mod")
	for _, name := range found {
		assert.Contains(t, allowedGoModDirectives, name, "go.mod holds a directive that is not allowed")
	}
}

// allowedMiseTables are the tables a mise configuration file of this
// repository may hold, with the keys each may hold. The [env] table,
// the "env_file" setting, [tasks] and [hooks] are not on the list: each
// one adds to the environment of what mise starts, or starts a program
// of its own. [tools] holds the mise.PinnedTools alone, each with the
// version mise.lock locks (miseFindings), so no tool is named by a path
// or by a version that the lock does not hold.
var allowedMiseTables = map[string][]string{
	"settings": {"lockfile"},
	"tools":    mise.PinnedTools,
}

// bareKey matches a TOML key that needs no quotes
// (https://toml.io/en/v1.0.0#keys).
var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// tomlKey writes a tool name as a key of mise.toml: bare when TOML
// allows it, quoted otherwise, as a backend name with ":" or "/" is.
func tomlKey(name string) string {
	if bareKey.MatchString(name) {
		return name
	}
	return `"` + name + `"`
}

// lockedVersion matches a version as mise.lock holds one: exact, three
// numbers.
var lockedVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// TestLockedVersions checks that mise.lock locks the mise.PinnedTools and no
// other, each at an exact version.
func TestLockedVersions(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "mise.lock"))
	require.NoError(t, err)
	locked, err := mise.LockedVersions(data)
	require.NoError(t, err)
	assert.ElementsMatch(t, mise.PinnedTools, slices.Collect(maps.Keys(locked)), "the tools of mise.lock")
	for tool, version := range locked {
		assert.Regexp(t, lockedVersion, version, "the version of %s in mise.lock", tool)
	}
}

// lockPlatforms returns the platforms mise.lock holds a download of
// each tool for, in mise's names: the Targets, the supported
// targets, with darwin written macos and amd64 written x64. So a target
// added to Targets fails TestLockIntegrity until mise.lock carries
// it.
func lockPlatforms() []string {
	osName := map[string]string{"linux": "linux", "darwin": "macos"}
	archName := map[string]string{"amd64": "x64", "arm64": "arm64"}
	var platforms []string
	for _, t := range Targets {
		platforms = append(platforms, osName[t.GOOS]+"-"+archName[t.GOARCH])
	}
	slices.Sort(platforms)
	return platforms
}

// The line grammar of mise.lock as lockIntegrityFindings reads it: the
// form "mise lock" writes (measured with mise 2026.10.3), and no other.
// A key is followed by one string or one array of strings, on one line,
// so a value that spans lines, such as a multi-line string, is not a
// key line.
var (
	lockVersionLine    = regexp.MustCompile(`^lockfile_version = [0-9]+$`)
	lockToolHeader     = regexp.MustCompile(`^\[\[tools\.("[^"]+"|[A-Za-z0-9_-]+)\]\]$`)
	lockPlatformHeader = regexp.MustCompile(`^\[tools\.("[^"]+"|[A-Za-z0-9_-]+)\."platforms\.([a-z0-9-]+)"\]$`)
	lockKeyLine        = regexp.MustCompile(`^([a-z_]+) = ("[^"\\]*"|\[("[^"\\]*"(, "[^"\\]*")*)?\])$`)
)

// lockEntryKeys are the keys of a [[tools.<name>]] entry, and
// lockPlatformKeys those of one of its platform tables.
var (
	lockEntryKeys    = []string{"version", "backend", "specifiers"}
	lockPlatformKeys = []string{"checksum", "url", "url_api", "provenance"}
)

// lockChecksum matches a checksum as mise.lock holds one.
var lockChecksum = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// lockIntegrityFindings returns what in a mise.lock does not hold the
// integrity form of 12 12.1. A tool of the mise go backend, one whose
// name starts with "go:" and whose backend is that same name, is built
// from source, so it holds no platform table: its integrity rests on
// the Go checksum database. Every other tool holds a platform table for
// each of lockPlatforms, each with a sha256 checksum, and none for
// another platform. The file is read line by line, and each line must
// be blank, a comment, the lockfile_version line before the first
// header, a [[tools.<name>]] header, a [tools.<name>."platforms.<p>"]
// header that follows its tool's header, or a one-line key of
// lockEntryKeys or lockPlatformKeys in its table; a tool has one entry
// and one table per platform. So a lock that mise.LockedVersions reads the
// same line by line holds no spaced header, multi-line string or
// second entry that the two readers could see past.
func lockIntegrityFindings(data []byte) []string {
	type entry struct {
		backend   string
		checksums map[string]string
	}
	entries := map[string]*entry{}
	var order, bad []string
	tool, platform, headerSeen := "", "", false
	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case lockVersionLine.MatchString(line) && !headerSeen:
		case lockToolHeader.MatchString(line):
			headerSeen = true
			tool, platform = strings.Trim(lockToolHeader.FindStringSubmatch(line)[1], `"`), ""
			if _, dup := entries[tool]; dup {
				bad = append(bad, tool+": a second entry")
				tool = ""
				continue
			}
			entries[tool] = &entry{checksums: map[string]string{}}
			order = append(order, tool)
		case lockPlatformHeader.MatchString(line):
			headerSeen = true
			m := lockPlatformHeader.FindStringSubmatch(line)
			name := strings.Trim(m[1], `"`)
			e, ok := entries[name]
			switch {
			case !ok:
				bad = append(bad, name+": a platform table before its entry")
				tool, platform = "", ""
			case func() bool { _, dup := e.checksums[m[2]]; return dup }():
				bad = append(bad, name+": a second platform table for "+m[2])
				tool, platform = "", ""
			default:
				tool, platform = name, m[2]
				e.checksums[platform] = ""
			}
		case lockKeyLine.MatchString(line) && tool != "":
			m := lockKeyLine.FindStringSubmatch(line)
			key, value := m[1], strings.Trim(m[2], `"`)
			switch {
			case platform == "" && slices.Contains(lockEntryKeys, key):
				if key == "backend" {
					entries[tool].backend = value
				}
			case platform != "" && slices.Contains(lockPlatformKeys, key):
				if key == "checksum" {
					entries[tool].checksums[platform] = value
				}
			default:
				bad = append(bad, fmt.Sprintf("line %q", line))
			}
		default:
			bad = append(bad, fmt.Sprintf("line %q", line))
		}
	}
	platforms := lockPlatforms()
	for _, tool := range order {
		e := entries[tool]
		if strings.HasPrefix(tool, "go:") && e.backend == tool {
			if len(e.checksums) > 0 {
				bad = append(bad, tool+": a platform table, for a tool built from source")
			}
			continue
		}
		if strings.HasPrefix(e.backend, "go:") {
			bad = append(bad, tool+": backend "+e.backend+" for a tool of another name")
		}
		for _, p := range platforms {
			if !lockChecksum.MatchString(e.checksums[p]) {
				bad = append(bad, tool+": no sha256 checksum for "+p)
			}
		}
		for _, p := range slices.Sorted(maps.Keys(e.checksums)) {
			if !slices.Contains(platforms, p) {
				bad = append(bad, tool+": a platform table for "+p)
			}
		}
	}
	return bad
}

// TestLockIntegrity checks that mise.lock holds the integrity form of
// 12 12.1 for each tool it locks (lockIntegrityFindings).
func TestLockIntegrity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(citest.ModuleRoot(t), "mise.lock"))
	require.NoError(t, err)
	assert.Empty(t, lockIntegrityFindings(data), "mise.lock")
}

// TestLockPlatforms checks that the platforms of mise.lock are the four
// that mise.toml names in its header, read from Targets.
func TestLockPlatforms(t *testing.T) {
	assert.Equal(t, []string{"linux-arm64", "linux-x64", "macos-arm64", "macos-x64"}, lockPlatforms())
}

// TestLockIntegrityFindings checks what lockIntegrityFindings refuses
// in a lock, each case with the findings it returns, in order.
func TestLockIntegrityFindings(t *testing.T) {
	sum := "sha256:" + strings.Repeat("ab", 32)
	platforms := func(tool string, skip string) string {
		var b strings.Builder
		for _, p := range lockPlatforms() {
			if p == skip {
				continue
			}
			b.WriteString("[tools." + tool + `."platforms.` + p + "\"]\nchecksum = \"" + sum + "\"\nurl = \"https://example.invalid\"\n\n")
		}
		return b.String()
	}
	noSums := []string{"gh: no sha256 checksum for linux-arm64", "gh: no sha256 checksum for linux-x64", "gh: no sha256 checksum for macos-arm64", "gh: no sha256 checksum for macos-x64"}
	head := "# @generated\n\nlockfile_version = 3\n\n"
	gh := "[[tools.gh]]\nversion = \"2.0.0\"\nbackend = \"aqua:cli/cli\"\nspecifiers = [\"2.0.0\"]\n\n"
	gopls := "[[tools.\"go:golang.org/x/tools/gopls\"]]\nversion = \"0.23.0\"\nbackend = \"go:golang.org/x/tools/gopls\"\n\n"
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"a downloaded tool with every checksum", head + gh + platforms("gh", ""), nil},
		{"a tool built from source with no platform table", head + gopls, nil},
		{"a downloaded tool missing one platform", gh + platforms("gh", "macos-x64"), []string{"gh: no sha256 checksum for macos-x64"}},
		{"a downloaded tool with no platform table", gh, noSums},
		{"a platform table without its checksum", gh + strings.Replace(platforms("gh", ""), "checksum = \""+sum+"\"\n", "", 1), []string{"gh: no sha256 checksum for linux-arm64"}},
		{"a checksum that is not sha256", gh + strings.Replace(platforms("gh", ""), sum, "blake3:00", 1), []string{"gh: no sha256 checksum for linux-arm64"}},
		{"a sha256 checksum one digit short", gh + strings.Replace(platforms("gh", ""), sum, sum[:len(sum)-1], 1), []string{"gh: no sha256 checksum for linux-arm64"}},
		{"a sha256 checksum in upper case", gh + strings.Replace(platforms("gh", ""), sum, strings.ToUpper(sum), 1), []string{"gh: no sha256 checksum for linux-arm64"}},
		{"a platform outside the list", gh + platforms("gh", "") + "[tools.gh.\"platforms.windows-x64\"]\nchecksum = \"" + sum + "\"\n", []string{"gh: a platform table for windows-x64"}},
		{"a platform outside the list with no checksum", gh + platforms("gh", "") + "[tools.gh.\"platforms.windows-x64\"]\nurl = \"u\"\n", []string{"gh: a platform table for windows-x64"}},
		{"a tool built from source with a platform table", gopls + platforms(`"go:golang.org/x/tools/gopls"`, ""), []string{"go:golang.org/x/tools/gopls: a platform table, for a tool built from source"}},
		{"a downloaded tool whose backend claims go", strings.Replace(gh, "aqua:cli/cli", "go:github.com/cli/cli/v2/cmd/gh", 1), append([]string{"gh: backend go:github.com/cli/cli/v2/cmd/gh for a tool of another name"}, noSums...)},
		{"a backend that holds go: after its prefix", strings.Replace(gh, "aqua:cli/cli", "aqua:go:x", 1), noSums},
		{"a go: name with another backend", strings.Replace(gopls, "backend = \"go:golang.org/x/tools/gopls\"", "backend = \"aqua:x/gopls\"", 1), []string{"go:golang.org/x/tools/gopls: no sha256 checksum for linux-arm64", "go:golang.org/x/tools/gopls: no sha256 checksum for linux-x64", "go:golang.org/x/tools/gopls: no sha256 checksum for macos-arm64", "go:golang.org/x/tools/gopls: no sha256 checksum for macos-x64"}},
		{"checksums inside a multi-line string", "[[tools.gh]]\nversion = \"2.0.0\"\nbackend = \"aqua:cli/cli\"\nnote = \"\"\"\n" + platforms("gh", "") + "\"\"\"\n", []string{`line "note = \"\"\""`, `line "\"\"\""`}},
		{"a header with spaces", "[[ tools.gh ]]\nversion = \"2.0.0\"\n", []string{`line "[[ tools.gh ]]"`, `line "version = \"2.0.0\""`}},
		{"a second entry of one tool", gh + gh + platforms("gh", ""), []string{"gh: a second entry", `line "version = \"2.0.0\""`, `line "backend = \"aqua:cli/cli\""`, `line "specifiers = [\"2.0.0\"]"`}},
		{"a second platform table", gh + platforms("gh", "") + "[tools.gh.\"platforms.linux-x64\"]\nchecksum = \"" + sum + "\"\n", []string{"gh: a second platform table for linux-x64", `line "checksum = \"` + sum + `\""`}},
		{"a platform table before its entry", "[tools.gh.\"platforms.linux-x64\"]\nchecksum = \"" + sum + "\"\n\n" + gh + platforms("gh", ""), []string{"gh: a platform table before its entry", `line "checksum = \"` + sum + `\""`}},
		{"a key of a platform table in the entry", "[[tools.gh]]\nchecksum = \"" + sum + "\"\n", append([]string{`line "checksum = \"` + sum + `\""`}, noSums...)},
		{"an unknown key", strings.Replace(gh, "\n\n", "\nnote = \"x\"\n\n", 1) + platforms("gh", ""), []string{`line "note = \"x\""`}},
		{"an unknown key of a platform table", gh + strings.Replace(platforms("gh", ""), "url = ", "note = \"x\"\nurl = ", 1), []string{`line "note = \"x\""`}},
		{"the lockfile_version after a header", gh + platforms("gh", "") + "lockfile_version = 3\n", []string{`line "lockfile_version = 3"`}},
		{"a key outside any table", "version = \"1.0.0\"\n", []string{`line "version = \"1.0.0\""`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, lockIntegrityFindings([]byte(tt.src)))
		})
	}
}

// miseFindings returns what a mise configuration file holds outside
// allowedMiseTables: a key before the first table, a table that is not
// on the list, a key of a table that is not on its list, a tool whose
// value is not the version that locked holds for it, as "1.27.0" with
// its quotes, and a line that is neither a table nor a "key = value"
// line on one line, as a value that spans lines is.
func miseFindings(data []byte, locked map[string]string) []string {
	var bad []string
	table, inTable := "", false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			table = strings.TrimSpace(strings.Trim(line, "[]"))
			_, inTable = allowedMiseTables[table]
			if !inTable {
				bad = append(bad, "table ["+table+"]")
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.Trim(strings.TrimSpace(key), `"'`)
		switch {
		case !ok:
			bad = append(bad, "line "+line)
		case table == "":
			bad = append(bad, "key "+key+" before the first table")
		case inTable && !slices.Contains(allowedMiseTables[table], key):
			bad = append(bad, "key "+key+" in ["+table+"]")
		case table == "tools" && strings.TrimSpace(value) != `"`+locked[key]+`"`:
			bad = append(bad, "value of "+key+" in [tools]")
		}
	}
	return bad
}

// TestMiseFindings pins miseFindings on the forms that add to the
// environment, on the forms that name a tool mise did not install from
// the lock, and on the configuration of this repository.
func TestMiseFindings(t *testing.T) {
	locked := map[string]string{"go": "1.27.0", "golangci-lint": "2.14.0"}
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{"the repository's", "# about\n[settings]\nlockfile = true\n\n[tools]\ngo = \"1.27.0\"\ngolangci-lint = \"2.14.0\"\n", nil},
		{"env table", "[env]\nGOFLAGS = \"-tags=hide\"\n", []string{"table [env]"}},
		{"env subtable", "[env._]\nfile = \".env\"\n", []string{"table [env._]"}},
		{"env array of tables", "[[env]]\nGOFLAGS = \"x\"\n", []string{"table [env]"}},
		{"dotted env key at the top", "env.GOFLAGS = \"x\"\n", []string{"key env.GOFLAGS before the first table"}},
		{"env file setting", "[settings]\nenv_file = \".env\"\n", []string{"key env_file in [settings]"}},
		{"tasks", "[tasks.lint]\nrun = \"true\"\n", []string{"table [tasks.lint]"}},
		{"value over lines", "[tools]\ngo = [\n\"1\",\n]\n", []string{"value of go in [tools]", "line \"1\",", "line ]"}},
		{"a tool by a path", "[tools]\ngolangci-lint = \"path:./.lint\"\n", []string{"value of golangci-lint in [tools]"}},
		{"a version the lock does not hold", "[tools]\ngo = \"1.26.0\"\n", []string{"value of go in [tools]"}},
		{"a version prefix", "[tools]\ngo = \"1.27\"\n", []string{"value of go in [tools]"}},
		{"an inline table", "[tools]\ngo = { version = \"1.27.0\" }\n", []string{"value of go in [tools]"}},
		{"a quoted key", "[tools]\n\"golangci-lint\" = \"2.14.0\"\n", nil},
		{"a tool that is not pinned", "[tools]\nnode = \"24.0.0\"\n", []string{"key node in [tools]"}},
		{"a tool subtable", "[tools.golangci-lint]\nversion = \"2.14.0\"\n", []string{"table [tools.golangci-lint]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, miseFindings([]byte(tt.src), locked))
		})
	}
}

// TestMiseConfigs checks every mise configuration file of the
// repository, tracked or not (mise.toml, mise.local.toml, .mise.toml,
// .config/mise and the others whose path names mise), against
// allowedMiseTables and the versions of mise.lock, and that mise.toml
// pins each of mise.PinnedTools. fast starts no tool through mise, so an
// [env] table cannot reach a step; this keeps one out of "go vet" and
// "go test" too, when a developer starts them through mise. No
// .tool-versions file is allowed, since mise reads one as a
// configuration of tools too.
func TestMiseConfigs(t *testing.T) {
	root := citest.ModuleRoot(t)
	lock, err := os.ReadFile(filepath.Join(root, "mise.lock"))
	require.NoError(t, err)
	locked, err := mise.LockedVersions(lock)
	require.NoError(t, err)
	var configs []string
	for _, f := range repoFiles(t, root, "*mise*") {
		if strings.HasSuffix(f, ".toml") {
			configs = append(configs, f)
		}
	}
	require.Contains(t, configs, "mise.toml")
	for _, f := range configs {
		data, err := os.ReadFile(filepath.Join(root, f))
		require.NoError(t, err)
		assert.Empty(t, miseFindings(data, locked), "%s holds what is not on allowedMiseTables", f)
		if f == "mise.toml" {
			lines := strings.Split(string(data), "\n")
			for _, tool := range mise.PinnedTools {
				assert.Contains(t, lines, tomlKey(tool)+` = "`+locked[tool]+`"`, "mise.toml pins %s", tool)
			}
		}
	}
	assert.Empty(t, repoFiles(t, root, "*.tool-versions"), "the .tool-versions files of the repository")
}

// TestGoDirectiveEqualsMisePin checks that the go directive of go.mod
// is the go version mise.toml pins, so the toolchain the checks run
// and the one the module declares cannot drift apart.
func TestGoDirectiveEqualsMisePin(t *testing.T) {
	root := citest.ModuleRoot(t)
	mod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	toml, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	require.NoError(t, err)
	directive, ok := goDirective(mod)
	require.True(t, ok, "go.mod holds a go directive")
	pin, ok := misePin(toml, "go")
	require.True(t, ok, "mise.toml pins go")
	assert.Equal(t, pin, directive, "the go directive of go.mod and the go pin of mise.toml")
}

// goDirective returns the version of the go directive of a go.mod.
func goDirective(data []byte) (string, bool) {
	for line := range strings.SplitSeq(string(data), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if f := strings.Fields(line); len(f) == 2 && f[0] == "go" {
			return f[1], true
		}
	}
	return "", false
}

// misePin returns the version a mise.toml pins for tool in [tools]. It
// reads the one-line forms miseFindings allows and nothing else, and
// TestMiseConfigs runs miseFindings on the same file.
func misePin(data []byte, tool string) (string, bool) {
	inTools := false
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inTools = line == "[tools]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if inTools && ok && strings.Trim(strings.TrimSpace(key), `"'`) == tool {
			return strings.Trim(strings.TrimSpace(value), `"`), true
		}
	}
	return "", false
}

// TestGoDirectiveParsers pins goDirective and misePin on the forms they
// must read and on the ones that must not match.
func TestGoDirectiveParsers(t *testing.T) {
	v, ok := goDirective([]byte("module x\n\ngo 1.27.2 // note\n"))
	assert.True(t, ok)
	assert.Equal(t, "1.27.2", v)
	_, ok = goDirective([]byte("module x\n// go 1.27.2\n"))
	assert.False(t, ok, "a comment is no directive")
	_, ok = goDirective([]byte("module x\ntoolchain go1.27.2\n"))
	assert.False(t, ok, "toolchain is no go directive")
	v, ok = misePin([]byte("[settings]\ngo = \"0.0.0\"\n[tools]\ngo = \"1.27.2\"\n"), "go")
	assert.True(t, ok)
	assert.Equal(t, "1.27.2", v)
	v, ok = misePin([]byte("[tools]\n\"go\" = \"1.27.2\"\n"), "go")
	assert.True(t, ok, "a quoted key")
	assert.Equal(t, "1.27.2", v)
	_, ok = misePin([]byte("[settings]\ngo = \"1.27.2\"\n"), "go")
	assert.False(t, ok, "a go key outside [tools] is no pin")
}
