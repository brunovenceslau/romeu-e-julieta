# The julieta-claude kit's root install step acts on the agent-owned ~/.claude/skills

What is at stake: low to medium. A root write that follows a planted
symlink lands outside the directory it was meant for.

Scope: the `julieta-claude` kit of this repository
(`kits/julieta-claude`).

From the security audit of #291 of the sandbox kit repository (severity Low): the kit's last
install step runs as root `rm -rf`, `cp -R` and `chmod -R` under
`/home/agent/.claude/skills`, a directory the agent owns. If install
re-runs on a used sandbox, a symlink planted there sends those root
writes elsewhere. Not measured: whether sbx re-runs install on restart.

Proposed: `test ! -L` on `~/.claude` and `~/.claude/skills` first, or
stage the files root-owned.

Reopen: the next change to `kits/julieta-claude`, or a measure of
install re-runs.
