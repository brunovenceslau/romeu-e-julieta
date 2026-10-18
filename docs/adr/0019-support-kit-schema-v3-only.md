# 19. Support kit schema v3 only

Date: 2026-10-18

## Status

Proposed

## Context

sbx documents the v3 kit schema as the direction for new kits. A
project can use kits of one schema or of several. The specification
fixes the choice in
[06 6.1](../spec/06-kits.md#61-decisions): v3 only, never mixed with v1
or v2 kits, so every project uses v3 end to end. The v3 format is still
at milestone tags, so the choice is exposed to churn; the specification
meets that with a digest pin of the frontend and a go/no-go probe before
any kit is written (Q7 in the [index](../spec.md)).

This record was written on 2026-10-18, because the specification cites
the decision as authority. The decision is already in the first
specification commit of 2026-09-30.

### Alternatives considered

- **v1 or v2 kits, or a mix.** Not taken: one grammar to test is
  cheaper than several, and v3 is the direction sbx documents.
- **A published kit of our own, or v2 kits as a fallback.** Kept as the
  options of a no-go: if the probe A11 finds that a local v3 kit
  cannot be built, kit work stops and a record states the result and
  the options, each with its trigger (wait for the next frontend
  milestone, or reconsider v2 kits or a published kit; Q7 reopens).

## Decision

Product kits and personal kits use the v3 schema only, and a project
never mixes v3 with v1 or v2 kits. There is no v2 fallback in v1.

## Consequences

- One grammar is tested and one path is built.
- The format is unstable until sbx publishes a stable v3, so a frontend
  bump is a reviewed change and runs the probes that
  [11 11.4](../spec/11-host-probes.md#114-probe-lifecycle) lists.
- If the go/no-go probe fails, the decision is reopened by a new record.
- The specification, the reviews, the notes and the commit messages give
  no reason for rejecting v1 and v2 kits beyond the one-grammar reason
  and sbx's direction.
