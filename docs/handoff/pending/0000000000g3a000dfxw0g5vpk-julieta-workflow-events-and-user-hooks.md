# julieta workflow events and user hooks

- An idea of 2026-10-19: julieta emits events of the user's own
  workflow (a pull request merged, a session started, a task closed)
  and runs hooks the user declares for them, as `julieta hooks run`
  (I21) runs a repository's git hooks today. A merged pull request
  would then tick its task with no hand step.
- Open: julieta puts no structure on open work in a sandbox today
  (`julieta memory` holds free text), so an event about a task needs a
  task shape first; and a sandbox hears of a merge only by asking
  GitHub.
- What is at stake: none yet; it is an idea, recorded so it is weighed
  when the evidence comes.

Reopen: after sitting M1, when a third repeated hand step needs an
event git does not give.
