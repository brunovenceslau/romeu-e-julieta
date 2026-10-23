# Pushes and pull requests need no approval in this repository from 2026-10-23

What is at stake: low. A push to a branch other than main reruns its CI
and is undone by another push, a pull request is closed, and merging
stays with the maintainer; what cannot be undone, a public text that
names what it must not, is caught by the forbidden-names gate that
still passes first.

Decided on 2026-10-23. Scope: this repository, every session until it
is changed; the rule is written here only.

Pushing a branch other than main, a rebase of the branch of an open
pull request with `--force-with-lease`, opening a pull request, and
editing its title or body need no approval. These still do: a push to
main, a force push without a lease, deleting a branch, tag or release,
and every point of the sandbox payload's "Safety" list that this does
not name, among them every push once the repository has an Actions
secret or an environment with a reviewer or a secret. The
forbidden-names gate (`go run ./tools/ci hygiene`, and `--file` for a
body) passes before each push and pull request (ADR 0036), and an
ask-first surface still needs its approval line approved. This replaces
"Pushes and pull requests are asked first from 2026-10-22"
(01m5gywvgc85vj8k4txzppjz6d) and the session-only "Pushes to the branch
of an open pull request need no approval in the session of 2026-10-23"
(01m5n7net1v4sgs10t0nvwvyka).
