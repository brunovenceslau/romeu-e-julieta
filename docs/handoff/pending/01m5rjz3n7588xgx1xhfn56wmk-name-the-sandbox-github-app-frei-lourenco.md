# Name the sandbox GitHub App Frei Lourenco

What is at stake: low. A name only; renaming an App later changes its
slug and the `[bot]` login on new commits, not the access it grants.

Scope: every repository and sandbox of the maintainer; written here
until the App exists, then in the record that creates it.

When the sandbox moves to a GitHub App token
([ADR 0011, keep the administrator's pull-request bypass and give the sandbox a GitHub App token](../../adr/0011-keep-the-administrator-s-pull-request-bypass-and-give-the-sandbox-a-github-app-token.md)),
the App is named Frei Lourenco (slug `frei-lourenco`, free on
2026-10-25 by `gh api apps/frei-lourenco`), proposed on 2026-10-25. In
the play the friar is outside both houses, trusted by both, and grants
what lets Romeo and Juliet meet; the App sits outside the host (romeu)
and the sandbox (julieta) and grants each installation's short-lived
token. The names that follow it:

- an installation of the App on a repository: the friar's cell;
- the installation token, valid for one hour: the potion, which wears
  off;
- the piece inside the sandbox that fetches the token and hands it to
  `git` and `gh`: the Nurse (`ama-da-julieta`, free as a user name on
  the same day), who carries the rope ladder.

The friar's letter that never arrives is the failure to design for: a
token that expires in the middle of a push.

Reopen: the authentication change of ADR 0011 is scheduled.
