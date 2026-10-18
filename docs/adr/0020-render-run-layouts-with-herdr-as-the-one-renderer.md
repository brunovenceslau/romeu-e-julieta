# 20. Render run layouts with herdr as the one renderer

Date: 2026-10-18

## Status

Proposed

## Context

julieta renders the run layout of a project, its panes and command
steps, into a terminal multiplexer inside the sandbox. The specification
names herdr as that multiplexer
([06 6.4](../spec/06-kits.md#64-product-kits)). herdr is pre-1.0 and
kept by one upstream, which publishes one binary per platform with no
checksum file and no signature; the measurement of 2026-10-01 over its
last three releases is in that section. The decision is listed in the
specification as Q16 (herdr pre-1.0 churn) with its mitigation, and the
survival of herdr across the loss of the host terminal was added to
block A in review round 1 (A13, finding D14 of
[round 1](../reviews/round-1.md)).

The specification lists tmux and cmux renderers among the non-goals of
v1 ([00 0.3](../spec/00-scope.md#03-non-goals-v1)). This record was
written on 2026-10-18, because the specification cites the decision as
authority.

### Alternatives considered

- **tmux or cmux as renderers.** Not taken in v1 (a non-goal). The run
  layout format names no multiplexer, and the renderer interface is
  designed together with the second renderer.

## Decision

herdr is the one run-layout renderer of v1, and it is pinned. The run
layout format stays free of any multiplexer name. The pin and its
mechanism are in
[06 6.4](../spec/06-kits.md#64-product-kits).

## Consequences

- The pre-1.0 risk is carried by the pin, the protocol number that is
  checked and fails closed, and a hash that the kit install step
  verifies. What the hash does not prove is a residual risk of
  [05 5.4](../spec/05-security.md#54-known-residual-risks-accepted-in-v1).
- A second renderer is a row of the
  [Deferred decisions](../spec.md#in-the-product).
- The specification, the reviews, the notes and the commit messages give
  no reason for choosing herdr over another multiplexer.
