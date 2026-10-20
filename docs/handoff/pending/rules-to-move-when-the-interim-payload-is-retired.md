# Rules to move when the interim payload is retired

What is at stake: a rule that lives only in the interim sandbox payload
is lost when romeu and julieta replace it, and builders in every
repository go back to the old behavior.

Scope: every repository and sandbox of the maintainer.

The rules this repository's records sent to the payload, each with its
later home, the personal kit in the config repository (REQ-0608) unless
named otherwise:

- Search for a maintained library before writing one; import, copy a
  small proven piece with its license and origin, or write by hand with
  a stated reason ("library-first-in-the-sandbox-payload"; sent to the
  payload's session on 2026-10-20). Open: the personal kit, or a product
  skill under `skills/` if adopters get it too.
- State the scope of every decision record (ADR 0039): the record
  itself lives here; the payload's house rule on records gains the same
  sentence.

The payload holds other rules that came from elsewhere; listing them
belongs to the payload's own repository.

Reopen: when the migration of the payload to the config repository is
planned, or when a record sends another rule to the payload.
