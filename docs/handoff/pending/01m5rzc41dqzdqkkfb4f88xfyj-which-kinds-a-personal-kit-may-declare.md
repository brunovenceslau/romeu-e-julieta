# Which kinds a personal kit may declare

What is at stake: high. A personal kit that widens a sandbox without a
refusal is what REQ-0574 exists to stop.

Scope: this repository, T038 and 06 6.1.

REQ-0574 (06 6.1) refuses network, mount, volume, credential and
ssh-agent capabilities in a personal kit. A descriptor's `kind` is
`workload`, `mixin` or `set` (`internal/oci/capabilities.go`), and a
`set` lists other kits by reference (`Kits`). The capabilities of those
referenced kits are not in the personal kit's own descriptor, so the
five-type check of slice 3 never sees them; a `workload` is a base
image, not a mixin (an inference from the field, not spec text). The spec does not say which kinds a personal kit may be.

Recommended: refuse, at sync, a personal kit whose kind is not `mixin`,
with an error naming the kit and the kind, before the slice 3 pull
request opens; it is one line to undo, while the gap widens a sandbox
silently. The why column of REQ-0574 ("personal kits should not widen a
sandbox silently") supports it. A change to the text of 06 6.1 is a
separate decision.

Reopen: the review of the slice 3 pull request, or the next edit of
06 6.1.
