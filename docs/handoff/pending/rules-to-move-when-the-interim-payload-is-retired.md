# Rules to move when the interim payload is retired

What is at stake: a rule that lives only in the interim sandbox payload
is lost when romeu and julieta replace it, and builders in every
repository go back to the old behavior.

Scope: every repository and sandbox of the maintainer.

The rules this repository's records mean for the payload, each with
whether it was sent and its later home, the personal kit in the config
repository (REQ-0608) unless named otherwise:

- Search for a maintained library before writing one; import, copy a
  small proven piece with its license and origin, or write by hand with
  a stated reason (ADR 0038, and the pending note
  "library-first-in-the-sandbox-payload"). Sent: a prompt was pasted
  into the payload's session on 2026-10-20; its pull request is not yet
  reported. Open: the personal kit, or a product skill under `skills/`
  if adopters get it too.
- State the scope of every decision record (ADR 0039), as a sentence
  in the payload's "How to work". Not yet sent.

The payload holds other rules that came from elsewhere; listing them
belongs to the payload's own repository.

Reopen: when the migration of the payload to the config repository is
planned, when a record means another rule for the payload, or when a
payload session reports its pull request.
