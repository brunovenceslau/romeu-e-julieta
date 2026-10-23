# T170 measures whether macos-26 runs a Linux container before the Mac pair is placed

What is at stake: low. The measurement costs one CI run; a wrong
placement of the Mac pair only moves a test between CI and a sitting.

Decided on 2026-10-23. Scope: this repository, T170.

T170 measures whether the `macos-26` runner runs a Linux container
before it decides where the Mac pair (see "The only supported pair is a
Mac on Apple silicon with a linux/arm64 sandbox", 01m5n79k7zb9yvw4bneb3pcv96) is tested: in CI
if it does, at a sitting on the maintainer's Mac if it does not.

Reopen: the T170 measurement is recorded.
