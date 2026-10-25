# Current state

## Where we stopped

Checkpoint of 2026-10-25: "Session of 2026-10-25: T111 re-planned onto
the julieta-claude kit, tools/manifest and T079"
(01m5rnxnxm3xt38f3jewy56g1w).
Pushes and pull requests need no approval in this repository, within
the limits that record names (01m5nbzfb4p92djhp10c92jprp). Work for
another repository goes into an issue there
(01m5rnxnykc2nbn1d2dvyyxmsw).

## Next steps

1. T111 at sitting M1, ready now that T079 is merged: the wave 1 edits
   of romeu-e-julieta and verona in the sandbox kit repository, the kit
   pinned at 26d49d75 (01m5nw2x1dw37z6sc6tebk991e); its runbook and
   cut-over are merged there, and its issue #287 keeps the atomic
   replace of the host-built julieta open.
2. T038, the next of the critical path (issue #271); T165 after T164.
3. Build T170; T161 once T088 is built
   (01m5e14094yhqhsxq2g941pwrf).
4. The pending notes, each when its trigger fires: 01m5rjz3n7588xgx1xhfn56wmk, 01m5rjz3p5x27yebkayt1hjydb, 01m5ngx0cy47r560n6hqjh4y1c, 01m5ngabwa6akgjr9eetw78kq7, 01m5ndjpzgf0rmwkr7gc1j6wdh, 01m5n920q50b79dxxqe5ghss4r, 01m5n79kdbtk9dpq7y87sfkkaq,
   01m5gywvh9pdmte8k85q87wwdc, 01m5gywvk15asanh438p4mmwfs, 01m5gywvkw8rp4jj2cvctvzg57,
   01m5gz0rpmzwvmwqt9ebd95rfq, 01m5kfnpybcnf22gpxwws14g7x, 01m5kfnpzsnmv53xtkkhenqkz1,
   01m5kfnq16e2agrcfmg2e5qhh8, 01m5kgfdf3t5hd41trsv35dss5 (in T043).

## Open for the operator

1. Answer the open questions of T161, the pin-freshness run among
   them. Recommended: before the next pin bump.
2. Answer the three questions of "Prefer the harness's defaults and
   publish what is left" (01m5drzqvjrf3e7p7b2v68temw). Recommended:
   before more CI tooling.
3. Approve or decline the two proposals of "Validate Mermaid diagrams"
   (01m5drzqwhebfeqegfbe7gmte9). Recommended: the rule now, the CI
   check with the first Mermaid block.
4. ADR 0040 was merged as Accepted in #217 without a `decisions`
   approval line. Recommended: say whether that merge approves it.
5. Paste the payload prompts left from 2026-10-21, report the kit pull
   request for T112 (00000000008h7a0ey78frxzgfr), and say when the
   GitHub App of ADR 0011 changes authentication. Recommended: later.
6. Plan the product skill mechanism (01m5gywvmtxk1kppkg3k1e1ae0); its
   trigger, the build of T079, fired with #277. Recommended: plan it
   before T114, with 01m5jmx65egty4eh39y8sqewhe.
7. Let the payload's handoff check accept the T159 Done record name
   (01m5gz0rpmzwvmwqt9ebd95rfq). Recommended: the payload's next session.
8. Review the over-optimizing rules note (01m5jc2v5gfyxyrbjv7wymbr9a).
   Recommended: the next session on the kit's repository.
9. On the host, for sitting M1: pick the shared bin directory outside
   every read-write mount and `$HOME/Developer`, then the runbook's
   host steps. Recommended: after the wave 1 pull request is open.

## In flight

- The pull request of this checkpoint. The branches
  `feat/t077-os-base` and `feat/t077-julieta-kit` are superseded by
  #273 and left as they were.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
