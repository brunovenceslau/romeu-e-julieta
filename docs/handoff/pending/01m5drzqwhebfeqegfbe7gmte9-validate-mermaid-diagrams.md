# Validate Mermaid diagrams

What is at stake: small. A Mermaid block that does not parse shows on
GitHub as an error box instead of a diagram; on 2026-10-20 a `;` inside a
sequence message broke a published one.

Scope: this repository for the check; every repository and sandbox of
the maintainer for the habit, through the sandbox payload.

Found on 2026-10-20: mermaid's own parser (`mermaid.parse()`, package
`mermaid` 11.4.1 under jsdom 25.0.1 in Node 22) reproduced GitHub's
error exactly and passed the fixed diagram. No `.md` file of this
repository holds a Mermaid block today; ADR 0001 rule 17 allows them,
and the state diagrams of REQ-1110 will be generated.

Proposed, not approved:

1. `tools/ci docs` parses every Mermaid block, through the official
   mermaid-cli image pinned by digest, as the license step runs `reuse`,
   so the Go CI gains no Node toolchain (the `checks` surface and a new
   image: ask first).
2. A payload rule: parse every Mermaid diagram before publishing it (a
   gist, a pull request, a doc), with the parser shipped as a kit tool.

Reopen: when the first Mermaid block enters `docs/` or the REQ-1110
generator is planned, or when the operator approves either proposal.
