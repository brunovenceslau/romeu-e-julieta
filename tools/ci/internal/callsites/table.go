// SPDX-FileCopyrightText: 2026 Bruno Venceslau
// SPDX-License-Identifier: GPL-3.0-only

package callsites

// Kind is one kind of call the check lists.
type Kind string

// The kinds of call of I16 (REQ-0499) and of the os/exec rule
// (REQ-0500), plus the runner of 04 4.1 (Subprocesses, item 1).
const (
	// KindRemove is os.Remove and os.RemoveAll, and the calls of the same
	// meaning: syscall and unix Unlink, Unlinkat and Rmdir, and the
	// Remove and RemoveAll methods of an os.Root.
	KindRemove Kind = "os.Remove*"
	// KindRename is a rename over an existing file: os.Rename and its
	// syscall and unix forms, the Rename method of an os.Root, and the
	// atomic replace of renameio (WriteFile, Symlink and
	// CloseAtomicallyReplace), which ends in a rename.
	KindRename Kind = "rename-over"
	// KindTruncate is the O_TRUNC flag.
	KindTruncate Kind = "O_TRUNC"
	// KindUpdateRefDelete is git update-ref with -d or --delete.
	KindUpdateRefDelete Kind = "update-ref -d"
	// KindBranchDelete is git branch with -D, -d or --delete.
	KindBranchDelete Kind = "branch -D"
	// KindGCPrune is git gc with a --prune flag.
	KindGCPrune Kind = "gc --prune"
	// KindEnvRm is the argv "env", "rm" of sbx.
	KindEnvRm Kind = "sbx env rm"
	// KindExec is exec.Command and exec.CommandContext.
	KindExec Kind = "exec.Command"
	// KindRunner is procgroup.Run and procgroup.CombinedOutput, the
	// runner every subprocess of the product starts through.
	KindRunner Kind = "procgroup runner"
)

// AnyFunc in the Func of a row admits every function of the package:
// the rows of the runner, which 04 4.1 admits by package.
const AnyFunc = "*"

// Site is one row of the admitted-call-site table: a call kind admitted
// in one function of one package. Package is a directory below the
// module root; Func is the name of a function ("name") or of a method
// ("Type.name", no star); a call outside every function, such as a
// package-level variable, has the name "" and is never admitted.
type Site struct {
	Kind          Kind
	Package, Func string
}

// Admitted is the one admitted-call-site table (05 5.2, REQ-0481). A
// call site is admitted here by package and function name and in no
// other way: a marker comment in the function admits nothing. A row
// widens I16, so this file is on the checks surface and each row carries
// the checks approval line.
var Admitted = []Site{
	// os/exec: internal/procgroup alone (04 4.1, Subprocesses, item 4).
	{KindExec, "internal/procgroup", "Run"},

	// The runner: sbxdrv and gitsafe for romeu, mise and hooks for
	// julieta (04 4.1, Subprocesses, item 1; REQ-0500). The doctor's
	// secret-argv row and layout have no call site yet; each joins in the
	// pull request that writes its first one.
	{KindRunner, "internal/sbxdrv", AnyFunc},
	{KindRunner, "internal/gitsafe", AnyFunc},
	{KindRunner, "internal/mise", AnyFunc},
	{KindRunner, "internal/hooks", AnyFunc},

	// I16 (05 5.2): the functions that remove temp files, candidate dirs
	// and kit dirs no longer referenced by the live sbxenv.yaml, replace
	// files under .romeu/bin/ by rename, and run sbx env rm for rm.
	// Temporary files and directories.
	{KindRemove, "internal/gitsafe", "Runner.BundleOf"},
	{KindRemove, "internal/cmd/romeu/init", "cloneConfig"},
	// Candidate dirs.
	{KindRemove, "internal/state", "RenderCandidate"},
	{KindRemove, "internal/state", "Candidate.remove"},
	{KindRemove, "internal/render", "removeCandidateDir"},
	// Kit dirs no longer referenced, and the half-copied .tmp-* dir a
	// crash leaves. The three functions of internal/render other than
	// replaceFile land with the promotion commit of T039.
	{KindRemove, "internal/render", "pruneKitDirs"},
	{KindRemove, "internal/render", "removeKitTemp"},
	// Files under .romeu/bin/ replaced by rename; render.json and
	// sbxenv.yaml go through the same function.
	{KindRename, "internal/render", "replaceFile"},
	// sbx env rm, for rm.
	{KindEnvRm, "internal/sbxdrv", "EnvRm"},
}
