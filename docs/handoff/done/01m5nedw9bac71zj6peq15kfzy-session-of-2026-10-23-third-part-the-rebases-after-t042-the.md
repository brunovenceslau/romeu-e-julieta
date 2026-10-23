# Session of 2026-10-23, third part: the rebases after T042, the helper text, A12 and T077

Done on 2026-10-23, after the checkpoint of #266. Each pull request
passed `go run ./tools/ci all` on the committed tree, the forbidden-names
gate on the tree and the body, and the hosted CI; those marked "review"
had a code review before they opened.

- #264 T170 (plan): rebased onto main after #263, so its macOS race ran
  with the fix of gate's prompt order; merge e94df8ea.
- #265 T164: rebased onto main after #263; the conflicts were the rows
  of RJ-355 and RJ-356 in errors.md, exit-codes.md and 04 4.4, both
  kept; merge 3a9c9345.
- #266 the checkpoint of the second part; merge b0b62a35.
- #267 docs: the per-host credential helper text in 03 3.4 and 05
  REQ-0464, with its `spec` approval line, closing "Apply the per-host
  credential helper text to 03 3.4 and 05 REQ-0464 after T164 merges"
  and "On macOS the race of internal/gate
  on main can hang until the test timeout",
  both removed there; and "Pushes and pull requests need no
  approval in this repository from 2026-10-23"
  (01m5nbzfb4p92djhp10c92jprp); review; merge 0121b247.
- #268 docs: the ticks of T042 and T164; review; open at this
  checkpoint.
- A12 measured on the Mac (sbx v0.49.2, host Darwin arm64, workload
  c0132e89): tzdata and zsh missing; handle `sbx-kit-claude`; five
  volumes under `~/.claude` and `/var/lib/docker`, none surviving
  `sbx rm` and a same-name `sbx create`; `sbx env rm` and the ssh-agent
  check not measured. The record is in T077's task file on its branch.
- T077 built on `feat/t077-os-base`, not yet reviewed:
  "T077 waits for review on feat/t077-os-base, then its pull request"
  (01m5nedwa9kff3b724ytw2xay2).

Decisions of the session, each with its date:

- 2026-10-23: a re-run of a failed job is not used for the macOS hang
  of #264 and #265: a re-run of a `pull_request` run keeps the merge
  commit of its first event, so the branches were rebased instead.
- 2026-10-23: A12 is measured with sbx on the Mac, not with docker on
  the pinned image in the sandbox.
- 2026-10-23: `os-base` carries tzdata and zsh.

Lessons:

- A re-run of a `pull_request` run tests the merge commit of the
  original event; a fix on main reaches a pull request only by a new
  event, such as a rebase.
- In zsh a variable holding a command is not split into words; a
  command list for the operator uses `eval` or no variable.
- `bash -s` reads its script from stdin, and `sbx exec` and
  `sbx create` read stdin too; a script for the operator is written to
  a file and run with stdin from `/dev/null`.
- The docker on the Mac had no buildx, and `docker-credential-osxkeychain`
  was not on its PATH; an empty `DOCKER_CONFIG` reads a public image.
- The container e2e of a kit that installs from the Debian mirror needs
  `deb.debian.org:80` approved in the sandbox.
