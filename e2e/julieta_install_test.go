// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

//go:build e2e

package e2e

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The "internet" of the cases of julieta install and julieta lock: the
// container has no network but its loopback, so a local web server
// stands in for the hosts a tool comes from, and mise reaches it
// through the http backend. www serves /tmp/www on 127.0.0.1:18080,
// answers HTTP 403 for a path below /blocked/, as the proxy of a
// sandbox does for a host its policy does not allow, and logs each
// request line to /tmp/www.log. proxy is the HTTPS proxy a mise run
// is given, which logs the first line of each request it gets to
// /tmp/proxy.log and refuses it: a CONNECT names the host mise tried
// to reach.
const (
	wwwPy = `import http.server, os, sys
class H(http.server.SimpleHTTPRequestHandler):
    def log_message(self, fmt, *a):
        with open("/tmp/www.log", "a") as f:
            f.write(self.command + " " + self.path + "\n")
    def blocked(self):
        if "/blocked/" not in self.path:
            return False
        self.send_response(403)
        self.send_header("Content-Length", "20")
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(b"blocked by the proxy\n")
        return True
    def do_GET(self):
        if not self.blocked():
            super().do_GET()
    def do_HEAD(self):
        if not self.blocked():
            super().do_HEAD()
os.chdir(sys.argv[1])
http.server.ThreadingHTTPServer(("127.0.0.1", 18080), H).serve_forever()
`
	proxyPy = `import socketserver
class H(socketserver.StreamRequestHandler):
    def handle(self):
        line = self.rfile.readline().decode("latin1").strip()
        with open("/tmp/proxy.log", "a") as f:
            f.write(line + "\n")
        self.wfile.write(b"HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
class S(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
S(("127.0.0.1", 18081), H).serve_forever()
`
)

// helloTool and its lock: the tool every case installs, from the web
// server above. The checksum of its tarball, @SUM@ here and in the
// shell the sum of the file the setup makes, is what mise writes into the
// lock for the http backend when the mise.toml names one; bare is the
// same tool with none, whose lock holds a URL and no checksum.
const (
	helloToml  = "[tools]\n\"http:hello\" = { version = \"1.0\", url = \"http://127.0.0.1:18080/hello-1.0.tar.gz\", checksum = \"sha256:@SUM@\" }\n"
	helloLock  = "lockfile_version = 3\n\n[[tools.\"http:hello\"]]\nversion = \"1.0\"\nbackend = \"http:hello\"\nspecifiers = [\"1.0\"]\n\n[tools.\"http:hello\".\"platforms.linux-arm64\"]\nchecksum = \"sha256:@SUM@\"\nurl = \"http://127.0.0.1:18080/hello-1.0.tar.gz\"\n\n[tools.\"http:hello\".\"platforms.linux-x64\"]\nchecksum = \"sha256:@SUM@\"\nurl = \"http://127.0.0.1:18080/hello-1.0.tar.gz\"\n"
	bareToml   = "[tools]\n\"http:hello\" = { version = \"1.0\", url = \"http://127.0.0.1:18080/hello-1.0.tar.gz\" }\n"
	bareLock   = "lockfile_version = 3\n\n[[tools.\"http:hello\"]]\nversion = \"1.0\"\nbackend = \"http:hello\"\nspecifiers = [\"1.0\"]\n\n[tools.\"http:hello\".\"platforms.linux-arm64\"]\nurl = \"http://127.0.0.1:18080/hello-1.0.tar.gz\"\n\n[tools.\"http:hello\".\"platforms.linux-x64\"]\nurl = \"http://127.0.0.1:18080/hello-1.0.tar.gz\"\n"
	mise1Count = `grep -c "^GET /hello-1.0.tar.gz" /tmp/www.log || true`
)

// miseSetup is the script every mise case starts with, after the
// prologue of the setup cases: mise next to julieta on PATH, the
// manifest, and the two servers, started and answering.
func miseSetup(t *testing.T) string {
	t.Helper()
	return `
export PATH="$(dirname "$J"):$PATH"
export JULIETA_MANIFEST=` + manifestEnv(t, 1, originURL+"api.git") + `
mkdir -p /tmp/www/blocked
printf '#!/bin/sh\necho hello\n' > /tmp/www/hello
chmod +x /tmp/www/hello
tar -C /tmp/www -czf /tmp/www/hello-1.0.tar.gz hello
SUM=$(sha256sum /tmp/www/hello-1.0.tar.gz | cut -d' ' -f1)
# put FILE TEXT writes TEXT to FILE with @SUM@ replaced by the sum.
put() { printf '%s' "$2" | sed "s/@SUM@/$SUM/g" > "$1"; }
cat > /tmp/www.py <<'PY'
` + wwwPy + `PY
cat > /tmp/proxy.py <<'PY'
` + proxyPy + `PY
python3 /tmp/www.py /tmp/www &
python3 /tmp/proxy.py &
i=0
until curl -s -o /dev/null http://127.0.0.1:18080/hello-1.0.tar.gz && curl -s -o /dev/null -m 1 http://127.0.0.1:18081/; do
	i=$((i+1))
	[ "$i" -lt 100 ] || { echo "the local servers did not answer" >&2; exit 97; }
	sleep 0.1
done
: > /tmp/www.log
: > /tmp/proxy.log
cd ` + primaryPath + `
`
}

// TestJulietaInstall installs the tool of a repo's lock in the workload
// at the pinned mise (04 4.3, REQ-0403): a mise.toml with no mise.lock
// is lock-missing (RJ-340, exit 2) and nothing is installed; the
// install runs mise locked and puts the tool in the installs directory;
// the next run is skipped, with no request to the web server, since the
// lock-set digest equals the last success, and --force installs again.
func TestJulietaInstall(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+helloToml+`'
step nolock "$J" install
put mise.lock '`+helloLock+`'
step first "$J" install
step requests1 sh -c '`+mise1Count+`'
step tool sh -c 'find "$HOME/.local/share/mise/installs" -type f -name hello | head -1'
step second "$J" install
step requests2 sh -c '`+mise1Count+`'
step forced "$J" install --force
`)
	r := step(t, steps, "nolock")
	assert.Equal(t, 2, r.code, r.stderr)
	assert.True(t, strings.HasPrefix(r.stderr, "error RJ-340 lock-missing: "), r.stderr)
	assert.Contains(t, r.stderr, "fix: julieta lock --repo api\n")

	r = step(t, steps, "first")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "install api installed\ninstall web no mise.toml\n", r.stdout)
	assert.Equal(t, "1", strings.TrimSpace(step(t, steps, "requests1").stdout), "mise fetched the tool once")
	assert.NotEmpty(t, strings.TrimSpace(step(t, steps, "tool").stdout), "the tool is installed")

	r = step(t, steps, "second")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "install api skipped, the lock-set is unchanged\ninstall web no mise.toml\n", r.stdout)
	assert.Equal(t, "1", strings.TrimSpace(step(t, steps, "requests2").stdout), "the skipped install asked for nothing")

	r = step(t, steps, "forced")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "install api installed\ninstall web no mise.toml\n", r.stdout)
}

// TestJulietaInstallBlocked: a download the web server answers with HTTP
// 403, as the proxy of a sandbox does for a blocked host, fails with
// install-blocked (RJ-341), exit 1, naming the tool, the host, the sbx
// command that approves it once for this sandbox, that the approval is
// lost at recreate and the durable fix, and saying the host is not in
// the egress set of the manifest (04 4.3, REQ-0404, REQ-0637).
func TestJulietaInstallBlocked(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '[tools]
"http:blocked" = { version = "1.0", url = "http://blocked.example.org:18080/blocked/x.tar.gz", checksum = "sha256:@SUM@" }
' 
step lock "$J" lock
HTTP_PROXY=http://127.0.0.1:18080 step install "$J" install
`)
	require.Equal(t, 0, step(t, steps, "lock").code, step(t, steps, "lock").stderr)
	r := step(t, steps, "install")
	assert.Equal(t, 1, r.code, r.stderr)
	assert.True(t, strings.HasPrefix(r.stderr, "error RJ-341 install-blocked: "), r.stderr)
	assert.Contains(t, r.stderr, "mise got HTTP 403 from blocked.example.org while installing http:blocked; if that is the egress policy")
	assert.Contains(t, r.stderr, "tool: http:blocked\n")
	assert.Contains(t, r.stderr, "host: blocked.example.org\n")
	assert.Contains(t, r.stderr, "sbx policy allow network --sandbox shop blocked.example.org")
	assert.Contains(t, r.stderr, "is not in the egress set of the manifest")
	assert.Contains(t, r.stderr, "lost when the sandbox is recreated")
	assert.Contains(t, r.stderr, "gate 2")
}

// TestJulietaInstallNoSelfUpdateRequest: at the pinned mise an install
// makes no request to mise's website, where its update check goes, nor
// to the versions host, which only a fuzzy version needs, while a
// run without MISE_DISABLE_UPDATE_WARNING does make the first and a
// lock of a fuzzy version does make the second, so the proxy that logs
// them sees what mise tries (REQ-0625; 04 4.3; 10 10.1).
func TestJulietaInstallNoSelfUpdateRequest(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
export HTTPS_PROXY=http://127.0.0.1:18081
put mise.toml '`+helloToml+`'
put mise.lock '`+helloLock+`'
step install "$J" install
step quiet cat /tmp/proxy.log
mkdir -p /tmp/ctl
cd /tmp/ctl
export MISE_TRUSTED_CONFIG_PATHS=/tmp/ctl MISE_YES=1 MISE_PARANOID=1
step doctor mise doctor
step control cat /tmp/proxy.log
: > /tmp/proxy.log
cd `+primaryPath+`
printf '[tools]\njq = "latest"\n' > mise.toml
rm mise.lock
unset MISE_TRUSTED_CONFIG_PATHS MISE_YES MISE_PARANOID
step fuzzy "$J" lock
step versions cat /tmp/proxy.log
`)
	r := step(t, steps, "install")
	require.Equal(t, 0, r.code, r.stderr)
	assert.Empty(t, strings.TrimSpace(step(t, steps, "quiet").stdout), "no request left the install")

	assert.Contains(t, step(t, steps, "control").stdout, "CONNECT mise.jdx.dev:443", "mise doctor checks for an update unless told not to")
	assert.Contains(t, step(t, steps, "versions").stdout, "CONNECT mise-versions.jdx.dev:443", "a fuzzy version reads the versions host")
}

// TestJulietaInstallRefusesUntrustedConfig: a configuration file outside
// MISE_TRUSTED_CONFIG_PATHS is refused at the pinned mise, though
// MISE_YES=1 is set: a mise.toml in the parent of the repo, which mise
// reads from the repo's directory, is not trusted, its exec() command
// does not run and install fails; the control, mise run with the same
// variables but MISE_PARANOID, trusts and runs it (04 4.3, REQ-0426,
// REQ-0621).
func TestJulietaInstallRefusesUntrustedConfig(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+helloToml+`'
put mise.lock '`+helloLock+`'
printf '[env]\nPWNED = "{{ exec(command=\"touch /tmp/pwned\") }}"\n' > /tmp/work/mise.toml
step install "$J" install
step planted sh -c 'ls /tmp/pwned 2>&1 | head -1'
rm -f /tmp/pwned
step control env -i PATH="$PATH" HOME="$HOME" MISE_YES=1 MISE_TRUSTED_CONFIG_PATHS=`+primaryPath+` mise env
step controlplanted sh -c 'ls /tmp/pwned 2>&1 | head -1'
`)
	r := step(t, steps, "install")
	assert.Equal(t, 1, r.code, r.stderr)
	assert.Contains(t, r.stderr, "not trusted")
	assert.Contains(t, step(t, steps, "planted").stdout, "No such file", "the planted command did not run")

	assert.Equal(t, 0, step(t, steps, "control").code, "without MISE_PARANOID, MISE_YES trusts the parent's file")
	assert.Equal(t, "/tmp/pwned", strings.TrimSpace(step(t, steps, "controlplanted").stdout), "and its command runs")
}

// TestJulietaInstallLockChange: a commit on the origin that changes
// mise.lock reaches the sandbox through setup, whose install step
// installs again, and a setup after it does not (04 4.3; the lock-change
// commit of J8, T075): the web server sees one request for each tool
// the origin's locks name, and none for a setup that changes nothing.
func TestJulietaInstallLockChange(t *testing.T) {
	secondToml := helloToml + "\"http:hello2\" = { version = \"1.0\", url = \"http://127.0.0.1:18080/hello2-1.0.tar.gz\", checksum = \"sha256:@SUM@\" }\n"
	secondLock := helloLock + "\n[[tools.\"http:hello2\"]]\nversion = \"1.0\"\nbackend = \"http:hello2\"\nspecifiers = [\"1.0\"]\n\n[tools.\"http:hello2\".\"platforms.linux-arm64\"]\nchecksum = \"sha256:@SUM@\"\nurl = \"http://127.0.0.1:18080/hello2-1.0.tar.gz\"\n\n[tools.\"http:hello2\".\"platforms.linux-x64\"]\nchecksum = \"sha256:@SUM@\"\nurl = \"http://127.0.0.1:18080/hello2-1.0.tar.gz\"\n"
	steps := scenario(t, miseSetup(t)+`
cp /tmp/www/hello-1.0.tar.gz /tmp/www/hello2-1.0.tar.gz
push() {
	printf '%s' "$1" | sed "s/@SUM@/$SUM/g" > /tmp/seed-api/mise.toml
	printf '%s' "$2" | sed "s/@SUM@/$SUM/g" > /tmp/seed-api/mise.lock
	g -C /tmp/seed-api add mise.toml mise.lock
	g -C /tmp/seed-api commit -q -m "$3"
	g -C /tmp/seed-api push -q origin main
}
push '`+helloToml+`' '`+helloLock+`' first
step setup1 "$J" setup --json
step requests1 sh -c "cat /tmp/www.log"
step setup2 "$J" setup --json
step requests2 sh -c "cat /tmp/www.log"
push '`+secondToml+`' '`+secondLock+`' second
step setup3 "$J" setup --json
step requests3 sh -c "cat /tmp/www.log"
step setup4 "$J" setup --json
step requests4 sh -c "cat /tmp/www.log"
`)
	for _, name := range []string{"setup1", "setup2", "setup3", "setup4"} {
		r := step(t, steps, name)
		require.Equal(t, 0, r.code, "%s: %s", name, r.stderr)
		var rep setupJSON
		decodeStrict(t, r.stdout, &rep)
		assert.Empty(t, rep.Errors, name)
	}
	count := func(name, line string) int {
		return strings.Count(step(t, steps, name).stdout, line+"\n")
	}
	assert.Equal(t, 1, count("requests1", "GET /hello-1.0.tar.gz"), "the first lock installs hello")
	assert.Equal(t, 1, count("requests2", "GET /hello-1.0.tar.gz"), "a setup with the lock unchanged installs nothing")
	assert.Zero(t, count("requests2", "GET /hello2-1.0.tar.gz"))
	assert.Equal(t, 1, count("requests3", "GET /hello2-1.0.tar.gz"), "the lock-change commit installs the new tool")
	assert.Equal(t, 1, count("requests4", "GET /hello2-1.0.tar.gz"), "and a setup after it nothing")
}

// TestJulietaInstallSetupNoop: julieta setup with nothing to change,
// the install skip included, takes at most 3 s, the median of 5, in the
// container against local origins (S8, REQ-0791). The bound is S8's own:
// a run over it is a defect to fix, never a flake to retry.
func TestJulietaInstallSetupNoop(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+helloToml+`'
put mise.lock '`+helloLock+`'
step warm "$J" setup --json
step timing sh -c 'for i in 1 2 3 4 5; do s=$(date +%s%N); "$1" setup >/dev/null 2>&1 || echo failed; e=$(date +%s%N); echo $(( (e - s) / 1000000 )); done' sh "$J"
`)
	require.Equal(t, 0, step(t, steps, "warm").code, step(t, steps, "warm").stderr)
	var ms []int
	for f := range strings.FieldsSeq(step(t, steps, "timing").stdout) {
		n, err := strconv.Atoi(f)
		require.NoError(t, err, "a run of setup failed: %q", step(t, steps, "timing").stdout)
		ms = append(ms, n)
	}
	require.Len(t, ms, 5)
	slices.Sort(ms)
	t.Logf("julieta setup no-op, 5 runs in ms: %v", ms)
	assert.LessOrEqual(t, ms[2], 3000, "the median of the no-op setup, in ms (S8)")
}

// TestJulietaInstallRefusesAMissingChecksum: install runs the check of
// lock --check first and refuses a lock whose entry holds no checksum,
// exit 6, check-findings, with no request to the web server; the
// exceptions file of the repo lets it install (decided on 2026-10-22).
//
// The container has no network, so an attested aqua tool cannot be
// installed here to show what MISE_PARANOID=1 adds: the verification of
// its attestation, which reaches tuf-repo-cdn.sigstore.dev. That was
// measured outside the container, through a proxy that logs the hosts
// mise reaches (07 7.4), and internal/mise's TestEnviron holds the
// variable in the environment of every run.
func TestJulietaInstallRefusesAMissingChecksum(t *testing.T) {
	steps := scenario(t, miseSetup(t)+`
put mise.toml '`+bareToml+`'
put mise.lock '`+bareLock+`'
step refused "$J" install
step requests1 sh -c '`+mise1Count+`'
mkdir .julieta
printf 'http:hello the test server publishes no checksum\n' > .julieta/lock-exceptions
step excused "$J" install
step requests2 sh -c '`+mise1Count+`'
`)
	r := step(t, steps, "refused")
	assert.Equal(t, 6, r.code, r.stderr)
	assert.Contains(t, r.stderr, "error RJ-328 check-findings: ")
	assert.Contains(t, r.stderr, "api http:hello linux-x64: lock-no-checksum")
	assert.Equal(t, "0", strings.TrimSpace(step(t, steps, "requests1").stdout), "nothing was fetched")
	r = step(t, steps, "excused")
	assert.Equal(t, 0, r.code, r.stderr)
	assert.Equal(t, "1", strings.TrimSpace(step(t, steps, "requests2").stdout))
}
