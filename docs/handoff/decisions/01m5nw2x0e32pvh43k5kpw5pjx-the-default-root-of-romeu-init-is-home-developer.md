# The default root of romeu init is HOME/Developer

What is at stake: low. The default only applies when `romeu init` runs
without `--root`; a wrong choice costs one `init` with `--root`, and no
host tree exists yet.

Decided on 2026-10-23. Scope: every adopter of the product; written in
REQ-0136 (02 2.3), 00 0.1, J1 (09) and `internal/cmd/romeu/init`.

The default root of the host tree is `$HOME/Developer`, the folder macOS
already offers for code, instead of `$HOME/dev`. `--root` stays the one
way to choose another path: no environment variable moves the root
(REQ-0150).

Reopen: a supported host where `$HOME/Developer` is a poor default.
