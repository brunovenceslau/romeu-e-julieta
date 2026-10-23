# Apply the per-host credential helper text to 03 3.4 and 05 REQ-0464 after T164 merges

Trigger: the pull request of T164 (`romeu init --git-credential-helper`)
merges.

The text to apply (the helper answers only the config repo's host) is
in the body of that pull request. The plan check refuses 03 and 05 in
the owns of T164, so a follow-up docs pull request applies it. The
rule is recorded in "The credential helper of romeu init answers only
the config repo's host" (01m5n79kanj94f68cwybw96ym1).
