# The open parts of A12: ssh-agent forwarding and the volume list

What is at stake: medium. A12 (11 11.1, REQ-0990) was measured on
2026-10-23 on the supported pair (a Mac on Apple silicon, sbx v0.49.2, a
linux/arm64 sandbox) against the pinned workload, which settled the
packages `os-base` carries (T077). Two of its parts stay open, and the
rules that read them (06 6.2, 01 1.2 and the socket forwarding of
06 6.4) rest on them.

Scope: this repository.

- The ssh-agent forwarding was not measured. The workload declares no
  ssh-agent capability, so `SSH_AUTH_SOCK` was unset inside with or
  without a host agent, and `ssh-add -L` listed no key. The probe needs a
  kit that declares the capability, likely `git-ssh-sign` (T081), with a
  one-key agent set for the `sbx` process and again with it unset.
- The volumes the workload declares are
  `/home/agent/.claude/projects` (2g), `sessions`, `todos`,
  `shell-snapshots` and `statsig` below `/home/agent/.claude` (512m
  each), and `/var/lib/docker`; none survived `sbx rm` and a recreate of
  the same name (markers absent after the recreate). The volume list of
  06 6.2 (REQ-0580) and the volume row of 01 1.2 say what a recreate
  keeps is the A12 record; the list `rm` and `recreate` print and the
  word `retire` says are to be written from this result. The probe ran
  `sbx rm`, where 11 11.1 names `sbx env rm`.

Reopen: a kit with an ssh-agent capability built at a sitting (T081, or
B2), for the first part; the first task that prints the volume list of
`rm`, `recreate` or `retire` (04 4.2), for the second.
