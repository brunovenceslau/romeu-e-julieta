// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brunovenceslau/romeu-e-julieta/internal/procgroup"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/check"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/mise"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetup runs setup against a fake mise and a fake go: it trusts
// mise.toml by its path, installs, and downloads the modules with the
// pinned go command, the one step whose environment keeps the go
// command's default proxy.
func TestSetup(t *testing.T) {
	r := citest.NewTree(t)
	r.Write("mise.lock", citest.LockFixture)
	r.Write(mise.PinnedWorkflow, citest.MiseWorkflow(t, citest.FakeMiseVersion))
	r.Commit("fixture")
	gitPath, err := procgroup.LookPath("git")
	require.NoError(t, err)
	sleepPath, err := procgroup.LookPath("sleep")
	require.NoError(t, err)
	f := citest.NewFakeMise(t)
	require.NoError(t, os.Symlink(gitPath, filepath.Join(f.Bin, "git")))
	log := filepath.Join(t.TempDir(), "log")
	goCmd := filepath.Join(f.Installs(), "go", "1.27.0", "bin", "go")
	require.NoError(t, os.MkdirAll(filepath.Dir(goCmd), 0o755))
	require.NoError(t, os.WriteFile(goCmd, []byte("#!/bin/sh\necho \"go $* GOPROXY=${GOPROXY-unset} GOFLAGS=$GOFLAGS HTTPS_PROXY=${HTTPS_PROXY-unset} DOCKER_HOST=${DOCKER_HOST-unset}\" >> '"+log+"'\n"), 0o700))
	linter := f.Tool(t, "golangci-lint", "2.14.0", "golangci-lint")
	script := "#!/bin/sh\n" + citest.AnswerVersion + "if [ \"$3\" = which ]; then\ncase \"$4\" in\ngo) echo '" + goCmd + "' ;;\ngolangci-lint) echo '" + linter + "' ;;\nesac\nexit 0\nfi\necho \"mise $* HTTPS_PROXY=${HTTPS_PROXY-unset} DOCKER_HOST=${DOCKER_HOST-unset} MISE=${MISE_OVERRIDE_CONFIG_FILENAMES-unset},${MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES-unset},${MISE_ENV-unset},${MISE_AUTO_ENV-unset} GO=${GOENV-unset},${GOTOOLCHAIN-unset}\" >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte(script), 0o700))
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid")
	t.Setenv("DOCKER_HOST", "unix:///nowhere")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("MISE_ENV", "ci")
	// The install builds govulncheck and gopls with the go command, so it
	// gets GOENV=off and GOTOOLCHAIN=local even when the process holds
	// others.
	// passThroughEnv already drops these two, so the override itself is
	// held by TestMiseEnvList.
	t.Setenv("GOENV", "/from/the/process")
	t.Setenv("GOTOOLCHAIN", "go1.99.0")

	code, out := citest.Run(t, r, nil, Run)
	assert.Equal(t, check.ExitOK, code, out)
	for _, w := range []string{"ok    mise trust", "ok    mise install", "ok    go mod download"} {
		assert.Contains(t, out, w)
	}
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	root, err := filepath.EvalSymlinks(r.Dir)
	require.NoError(t, err)
	// Both mise steps run with mise.MiseEnv, whatever the process holds.
	assert.Equal(t, "mise trust "+filepath.Join(root, "mise.toml")+" HTTPS_PROXY=unset DOCKER_HOST=unset MISE=mise.toml,none,,false GO=off,local\n"+
		"mise install HTTPS_PROXY=http://proxy.invalid DOCKER_HOST=unset MISE=mise.toml,none,,false GO=off,local\n"+
		"go mod download GOPROXY=unset GOFLAGS=-mod=readonly HTTPS_PROXY=http://proxy.invalid DOCKER_HOST=unset\n", string(data))

	t.Run("a hosted run without the mise environment stops before mise", func(t *testing.T) {
		require.NoError(t, os.Remove(log))
		t.Setenv("GITHUB_ACTIONS", "true")
		for _, kv := range mise.MiseEnv { // whatever the caller exports
			key, _, _ := strings.Cut(kv, "=")
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "the workflow sets MISE_OVERRIDE_CONFIG_FILENAMES=mise.toml for every mise run")
		assert.NoFileExists(t, log, "no mise command ran")
	})

	// The hosted run: the workflow's env table, and the variables that
	// jdx/mise-action v5.1.1 exports to the steps after it with
	// "env: false" (src/index.ts, setEnvVars: MISE_TRUSTED_CONFIG_PATHS
	// as the workspace, since an empty value counts as unset there,
	// MISE_YES=1 and MISE_LOG_LEVEL=info). The setup step needs the
	// workspace trusted for the first "go run" through the shim, so none
	// of them may stop it; and none reaches a mise run of tools/ci.
	t.Run("a hosted run keeps what mise-action exports out of the mise runs", func(t *testing.T) {
		dump := filepath.Join(t.TempDir(), "dump")
		hosted := "#!/bin/sh\n" + citest.AnswerVersion + "if [ \"$3\" = which ]; then\ncase \"$4\" in\ngo) echo '" + goCmd + "' ;;\ngolangci-lint) echo '" + linter + "' ;;\nesac\nexit 0\nfi\n" +
			"echo \"TRUSTED=${MISE_TRUSTED_CONFIG_PATHS-unset} YES=${MISE_YES-unset} LOG=${MISE_LOG_LEVEL-unset} CD=${MISE_CD-unset} ENVFILE=${MISE_ENV_FILE-unset} CEILING=${MISE_CEILING_PATHS-unset}\" >> '" + dump + "'\n"
		require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte(hosted), 0o700))
		t.Setenv("GITHUB_ACTIONS", "true")
		for _, kv := range mise.MiseEnv {
			key, value, _ := strings.Cut(kv, "=")
			t.Setenv(key, value)
		}
		t.Setenv("MISE_TRUSTED_CONFIG_PATHS", r.Dir)
		t.Setenv("MISE_YES", "1")
		t.Setenv("MISE_LOG_LEVEL", "info")
		t.Setenv("MISE_CD", "/elsewhere")
		t.Setenv("MISE_ENV_FILE", ".env")
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitOK, code, out)
		got, err := os.ReadFile(dump)
		require.NoError(t, err)
		root, err := filepath.EvalSymlinks(r.Dir)
		require.NoError(t, err)
		want := "TRUSTED=unset YES=unset LOG=unset CD=unset ENVFILE=unset CEILING=" + filepath.Dir(root) + "\n"
		assert.Equal(t, want+want, string(got), "mise trust and mise install get the ceiling and none of what the action exported")
	})

	t.Run("a mise of another version stops before mise runs anything else", func(t *testing.T) {
		require.NoError(t, os.WriteFile(log, nil, 0o600))
		r.Write(mise.PinnedWorkflow, citest.MiseWorkflow(t, "2026.10.4"))
		r.Commit("pins another mise")
		t.Cleanup(func() {
			r.Write(mise.PinnedWorkflow, citest.MiseWorkflow(t, citest.FakeMiseVersion))
			r.Commit("pins the fake's mise again")
		})
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitError, code, out)
		assert.Contains(t, out, "ci: mise is at version "+citest.FakeMiseVersion+", and .github/workflows/ci.yml pins 2026.10.4")
		data, err := os.ReadFile(log)
		require.NoError(t, err)
		assert.Empty(t, string(data), "no mise trust, no mise install")
	})

	t.Run("a failing mise stops before the download", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte("#!/bin/sh\n"+citest.AnswerVersion+"exit 1\n"), 0o700))
		code, out := citest.Run(t, r, nil, Run)
		assert.Equal(t, check.ExitFail, code, out)
		assert.Contains(t, out, "FAIL  mise trust")
		assert.NotContains(t, out, "go mod download")
	})

	// "mise install" downloads and builds the pinned tools, so it runs
	// under the class of every mise install, procgroup.Mise; "mise
	// trust" is a command step, under procgroup.ToolStep. Each case
	// stalls one mise command and shortens the limit of one class only:
	// the stalled command ends at the limit of its own class.
	tests := []struct {
		name    string
		stalls  string
		class   procgroup.Class
		wantErr string
	}{
		{"mise install runs under the mise class", "install", procgroup.Mise, "ran past the 100ms limit of class mise"},
		{"mise trust runs under the tool-step class", "trust", procgroup.ToolStep, "ran past the 100ms limit of class tool-step"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stall := "#!/bin/sh\n" + citest.AnswerVersion + "if [ \"$1\" = " + tt.stalls + " ]; then exec '" + sleepPath + "' 30; fi\nexit 0\n"
			require.NoError(t, os.WriteFile(filepath.Join(f.Bin, "mise"), []byte(stall), 0o700))
			ctx := procgroup.WithTestTimeout(t.Context(), tt.class, 100*time.Millisecond)
			var out bytes.Buffer
			e := check.Env{Dir: r.Dir, GitEnv: r.Env, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out}
			start := time.Now()
			code := citest.Exit(ctx, e, Run)
			assert.Equal(t, check.ExitFail, code, out.String())
			assert.Contains(t, out.String(), "FAIL  mise "+tt.stalls)
			assert.Contains(t, out.String(), tt.wantErr)
			assert.Less(t, time.Since(start), 10*time.Second, "the stalled command ends at the limit of its class")
		})
	}
}
