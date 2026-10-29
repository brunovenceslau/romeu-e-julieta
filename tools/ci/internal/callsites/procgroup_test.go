// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brunovenceslau/romeu-e-julieta/tools/ci/internal/citest"
)

// TestCallsitesProcgroupTree is the scan of 04 4.1, Subprocesses, item
// 4, over this module: every subprocess starts in internal/procgroup
// and nothing else maps a signal, and only a test file sets the seam of
// the timeout table. It ran as a test of internal/procgroup until T039.
func TestCallsitesProcgroupTree(t *testing.T) {
	root := citest.ModuleRoot(t)
	module, err := modulePath(root)
	require.NoError(t, err)
	tr, err := parseModule(root, module)
	require.NoError(t, err)
	assert.Empty(t, execFindings(tr))
}

// procgroupFixture is the tree of the positive control, written to a
// temporary directory, since a Go file under testdata would escape the
// linters. Each file outside procgroup tries one way around the rule;
// the rest is what the scan admits or leaves out.
var procgroupFixture = map[string]string{
	"internal/procgroup/run.go": `package procgroup

import (
	"os/exec"
	"os/signal"
	"syscall"
)

func Start(name string) error { return exec.Command(name).Start() }

func Notify() { signal.Ignore(); _ = syscall.Exec }

func WithTestTimeout() {}

func SetBeforeStepAside() {}
`,
	"internal/fixture/exec.go": `package fixture

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"internal/fixture/seam.go": `package fixture

import pg "example.com/fixture/internal/procgroup"

func Shorten() { pg.WithTestTimeout(); pg.SetBeforeStepAside() }
`,
	"internal/fixture/seam_test.go": `package fixture

import (
	"os/exec"
	"testing"

	"example.com/fixture/internal/procgroup"
)

func TestShorten(t *testing.T) {
	procgroup.WithTestTimeout()
	procgroup.SetBeforeStepAside()
	_ = exec.Command("true").Run()
}
`,
	"internal/fixture/other.go": `package fixture

import "example.com/fixture/internal/other"

func Other() { other.WithTestTimeout() }
`,
	"internal/fixture/signal.go": `package fixture

import "os/signal"

func Ignore() { signal.Ignore() }
`,
	"internal/fixture/start.go": `package fixture

import (
	sys "syscall"
	"os"

	"golang.org/x/sys/unix"
)

func Start() {
	_, _ = os.StartProcess("/bin/true", nil, nil)
	_, _ = sys.ForkExec("/bin/true", nil, nil)
	_ = sys.Exec("/bin/true", nil, nil)
	_ = unix.Exec("/bin/true", nil, nil)
	f := os.StartProcess
	_ = f
}
`,
	"internal/fixture/dot.go": `package fixture

import . "example.com/fixture/internal/procgroup"

func Shorten() { WithTestTimeout() }
`,
	"internal/fixture/cgo.go": `package fixture

// int system(const char*);
import "C"

func Run() { C.system(nil) }
`,
	"internal/fixture/linkname.go": `package fixture

import _ "unsafe"

//go:linkname forkExec syscall.forkExec
func forkExec()
`,
	"internal/procgroup/sub/sub.go": `package sub

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"internal/fixture/imports.go": `package fixture

import (
	_ "go/build"
	_ "go/importer"
	_ "golang.org/x/sys/execabs"
	_ "net/http/cgi"
	_ "plugin"
)
`,
	"internal/fixture/tools.go": `package fixture

import (
	"example.com/fixture/tools/ci/git"
	_ "example.com/fixture/tools"
	_ "example.com/fixture/toolsx"
)

func Run() { _ = git.Repo{} }
`,
	"tools/cix/git.go": `package cix

import (
	_ "go/build"

	_ "example.com/fixture/tools/ci/git"
)
`,
	"internal/fixture/raw.go": `package fixture

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func Raw(trap uintptr) {
	_, _, _ = syscall.Syscall(syscall.SYS_EXECVE, 0, 0, 0)
	_, _, _ = syscall.RawSyscall6(59, 0, 0, 0, 0, 0, 0)
	_, _, _ = unix.Syscall(unix.SYS_CLONE3, 0, 0, 0)
	_, _, _ = unix.RawSyscall(trap, 0, 0, 0)
	_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, 0, 0, 0)
	_ = unix.ForkExec
	f := syscall.Syscall6
	_ = f
}
`,
	"tools/cix/x.go": `package cix

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"tools/ci/http.go": `package ci

import _ "net/http/cgi"
`,
	"internal/procgroupx/x.go": `package procgroupx

import "os/exec"

func Start() error { return exec.Command("gh").Start() }
`,
	"tools/ci/ci.go": `package ci

import (
	_ "go/build"
	"os/exec"
	"os/signal"
)

func Start(name string) error { signal.Ignore(); return exec.Command(name).Start() }
`,
	"internal/fixture/testdata/ignored.go": `package ignored

import "os/exec"
`,
	"internal/.hidden/ignored.go": `package ignored

import "os/exec"
`,
	"internal/_ignored/ignored.go": `package ignored

import "os/exec"
`,
}

// TestCallsitesProcgroupFixture is the positive control of the scan, over
// procgroupFixture.
func TestCallsitesProcgroupFixture(t *testing.T) {
	root := writeTree(t, procgroupFixture)
	tr, err := parseModule(root, "example.com/fixture")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"internal/fixture/cgo.go: imports C, whose calls start programs past this package",
		"internal/fixture/dot.go: dot-imports example.com/fixture/internal/procgroup, which hides its calls from this scan",
		"internal/fixture/exec.go: imports os/exec outside internal/procgroup",
		"internal/fixture/imports.go: imports go/build outside internal/procgroup",
		"internal/fixture/imports.go: imports go/importer outside internal/procgroup",
		"internal/fixture/imports.go: imports golang.org/x/sys/execabs outside internal/procgroup",
		"internal/fixture/imports.go: imports net/http/cgi outside internal/procgroup",
		"internal/fixture/imports.go: imports plugin outside internal/procgroup",
		"internal/fixture/linkname.go: has a go:linkname directive, which reaches past this package",
		"internal/fixture/raw.go: calls golang.org/x/sys/unix.ForkExec outside internal/procgroup",
		"internal/fixture/raw.go: calls golang.org/x/sys/unix.RawSyscall with a trap the scan cannot name",
		"internal/fixture/raw.go: calls golang.org/x/sys/unix.Syscall with SYS_CLONE3",
		"internal/fixture/raw.go: calls syscall.RawSyscall6 with a trap the scan cannot name",
		"internal/fixture/raw.go: calls syscall.Syscall with SYS_EXECVE",
		"internal/fixture/raw.go: takes syscall.Syscall6 as a value, which hides its trap",
		"internal/fixture/seam.go: sets procgroup.SetBeforeStepAside outside a test file",
		"internal/fixture/seam.go: sets procgroup.WithTestTimeout outside a test file",
		"internal/fixture/seam_test.go: imports os/exec outside internal/procgroup",
		"internal/fixture/signal.go: imports os/signal outside internal/procgroup",
		"internal/fixture/start.go: calls golang.org/x/sys/unix.Exec outside internal/procgroup",
		"internal/fixture/start.go: calls os.StartProcess outside internal/procgroup",
		"internal/fixture/start.go: calls os.StartProcess outside internal/procgroup",
		"internal/fixture/start.go: calls syscall.Exec outside internal/procgroup",
		"internal/fixture/start.go: calls syscall.ForkExec outside internal/procgroup",
		"internal/fixture/tools.go: imports example.com/fixture/tools outside tools/, whose packages start programs past this package",
		"internal/fixture/tools.go: imports example.com/fixture/tools/ci/git outside tools/, whose packages start programs past this package",
		"internal/procgroup/sub/sub.go: imports os/exec outside internal/procgroup",
		"internal/procgroupx/x.go: imports os/exec outside internal/procgroup",
		"tools/ci/ci.go: imports go/build outside internal/procgroup",
		"tools/ci/ci.go: imports os/exec outside internal/procgroup",
		"tools/ci/ci.go: imports os/signal outside internal/procgroup",
		"tools/ci/http.go: imports net/http/cgi outside internal/procgroup",
		"tools/cix/git.go: imports go/build outside internal/procgroup",
		"tools/cix/x.go: imports os/exec outside internal/procgroup",
	}, whereMsgs(execFindings(tr)))
}
