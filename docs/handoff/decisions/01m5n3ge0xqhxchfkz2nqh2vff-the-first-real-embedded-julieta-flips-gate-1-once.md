# The first real embedded julieta flips gate 1 once

What is at stake: low. Gate 1 asks once more on a machine, at the
moment the first real julieta is embedded; nothing is gated wrongly.

Decided on 2026-10-23. Scope: this repository, T077 and gate 1 (01 1.4).

A source build embeds the empty placeholder julieta of 02 2.1 under the
fixed version `v0.0.0-placeholder` (`internal/cmd/romeu/approve`, T042).
The first real embedded julieta (T077) changes the julieta digest, so
it flips gate 1 once on each machine; that is expected, and the pull
request that embeds it says so. It is not a defect of gate 1.

Trigger to look again: the placeholder guard test of T042, which fails
when the placeholder is no longer the embedded julieta, is the signal
that the flip has happened.
