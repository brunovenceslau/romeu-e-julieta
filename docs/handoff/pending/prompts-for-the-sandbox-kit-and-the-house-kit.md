# Prompts for the sandbox kit and the house kit

Two prompts were drafted on 2026-10-16. The operator pastes them into the
sessions that own those kits. They are not yet merged into one final
version each.

- The sandbox kit:
  - The image ships mise 2026.9.15, while repositories pin 2026.10.3.
  - Subagent briefs now need a `SKILLS:` line, and a Go brief must name
    cc-skills-golang:golang-how-to and cc-skills-golang:golang-gopls.
  - After the mise swap, `mise trust` was needed in each checkout,
    because gh is a mise shim.
- The house kit:
  - Name the orchestrator "orchestrator-builder".
  - State that every builder, the orchestrator included, adversarially
    reviews its own work before the external review.
  - Decide where session-state notes live in a public repository.

Recommended: ask the next session for the two final prompts in one
message.

Reopen: the operator asks for the prompts.
