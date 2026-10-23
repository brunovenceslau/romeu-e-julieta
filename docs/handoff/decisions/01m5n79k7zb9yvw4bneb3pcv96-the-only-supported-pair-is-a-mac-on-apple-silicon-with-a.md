# The only supported pair is a Mac on Apple silicon with a linux/arm64 sandbox

What is at stake: medium. Spec text and tests that name another pair
promise support nobody measures; a wrong promise is cheap to correct
while no adopter relies on it.

Decided on 2026-10-23. Scope: this repository (the spec and the plan).

The only supported pair of host and sandbox is a Mac on Apple silicon
with a linux/arm64 sandbox. A Linux host with a sandbox of its own
architecture is a test vehicle, not a supported pair; that is not
changed now. 00 0.2 and REQ-0868 name an Intel Mac, and sbx does not
run on an Intel Mac; the correction is pending ("Correct the Intel Mac
in 00 0.2 and REQ-0868", 01m5n79kdbtk9dpq7y87sfkkaq) and T170 records it.

Reopen: a second host and sandbox pair is measured to work.
