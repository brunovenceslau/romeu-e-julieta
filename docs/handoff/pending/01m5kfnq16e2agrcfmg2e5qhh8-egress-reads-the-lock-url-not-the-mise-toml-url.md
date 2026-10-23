# Egress reads the lock url, not the mise.toml url

What is at stake: low. For an `http:` tool in locked mode, mise contacts
the `url` of `mise.toml`, which can differ from the lock host `Derive`
reads. It fails closed: that host can be in no egress set, so the proxy
blocks it and `install-blocked` names it.

Scope: this repository.

Decided on 2026-10-23, in the review of #255 (T036): leave it; 07 7.5
chose to read the lock only. The option is to read the `url` options of
`mise.toml` as well.

Reopen: the first project that uses an `http:` tool.
