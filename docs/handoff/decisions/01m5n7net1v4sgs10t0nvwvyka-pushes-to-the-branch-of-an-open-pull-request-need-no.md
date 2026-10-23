# Pushes to the branch of an open pull request need no approval in the session of 2026-10-23

What is at stake: low. A push to the branch of an open pull request
reruns its CI and can be reverted by another push; a force push is made
only with a lease on the tip the session last saw.

Decided on 2026-10-23. Scope: this session only, in this repository.

Opening a pull request, with the push that opens it, and a push to the
branch of an open pull request (a fix, or a rebase with
`--force-with-lease`), need no approval in this session. A push of a
branch with no pull request stays as "Pushes and pull requests are
asked first from 2026-10-22" (01m5gywvgc85vj8k4txzppjz6d) says. This
replaces "Opening a pull request needs no approval in the session of
2026-10-23" (01m5n3gdx9cpp905t4q440dc70), which covered the opening
alone; two fix pushes to open pull requests (#261 and #263) were made
under the wider reading before it was stated, and the wider reading was
then decided.
