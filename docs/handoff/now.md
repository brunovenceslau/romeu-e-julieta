# Current state

## Where we stopped

Checkpoint of 2026-10-25, second part: T111 wave 1 merged in the sandbox
kit repository; T038 to be rebuilt from main (01m5rtp0fp919zpq1f3gs67ngy).
Pushes and pull requests need no approval here, within the limits of
01m5nbzfb4p92djhp10c92jprp. Work for another repository goes into an
issue there (01m5rnxnykc2nbn1d2dvyyxmsw).

## Next steps

1. T111 sitting M1: the runbook's steps 6 and 7 after the recreate
   (did the kit load; then `julieta setup`), and its Record lines.
2. T038 (issue #271): rebuild from main, never consulting
   `feat/t038-render-kits` (01m5rtp0fp919zpq1f3gs67ngy); T165 after T164.
3. Build T170; T161 once T088 is built
   (01m5e14094yhqhsxq2g941pwrf).
4. The pending notes, each when its trigger fires: 01m5rtp0hjczzy9f66ez7rxvna,
   01m5rtp0kfzjckbdct0y77531j, 01m5rpbjkc62s2c7qecj7wf9bk, 01m5rjz3n7588xgx1xhfn56wmk,
   01m5rjz3p5x27yebkayt1hjydb, 01m5ngx0cy47r560n6hqjh4y1c, 01m5ngabwa6akgjr9eetw78kq7,
   01m5ndjpzgf0rmwkr7gc1j6wdh, 01m5n920q50b79dxxqe5ghss4r, 01m5n79kdbtk9dpq7y87sfkkaq,
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
9. On the host, for sitting M1: the runbook's steps 1 to 3 (bin
   `~/.local/share/julieta/bin`). Recommended: before the recreate.
10. Decide memory per repository and per project, and its durability
    (01m5rtp0hjczzy9f66ez7rxvna). Recommended: before multi-repo work.

## In flight

- The pull request of this checkpoint. `feat/t077-os-base`,
  `feat/t077-julieta-kit` (superseded by #273) and
  `feat/t038-render-kits` (ignored) are left as they were.

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
