# The credential helper of romeu init answers only the config repo's host

What is at stake: high. The helper hands a token to git; a helper that
answers another host, or a helper file others can write, leaks or
replaces the credential. A change of the rule is a code change in
`romeu init` and a spec change.

Decided on 2026-10-23. Scope: this repository (T164, `romeu init
--git-credential-helper`, 03 3.4, 05 REQ-0464, REQ-1267).

- The helper answers only the config repo's host, in every command.
- An absolute-path helper must be a regular executable owned by the
  user or root and not group or other writable, with every directory up
  to `/` held to the same rule. A refusal names the fix
  (`chmod g-w <path>`).
- A refusal of the operator's input is `usage`; a refusal caused by the
  remote's content is `config-clone-failed` (RJ-356).
- When the clone fails, init removes the root and `<cfg>-env/` it made.
- Git runs every helper through `sh`, so the `!` form stays refused
  (REQ-1267).

The spec text lands in a docs pull request after T164 merges ("Apply
the per-host credential helper text to 03 3.4 and 05 REQ-0464 after
T164 merges", 01m5n79kc411gxevqamd508scw).
