# Current state

## Where we stopped

Checkpoint of 2026-10-22: "Session of 2026-10-21 and 2026-10-22, the CI
tasks, T158 to T160 and T077" (01m5gywvffxt1tnh2fnn3s8cbc). The queue
ran under "Work authorized while the operator is away on 2026-10-21"
(01m5eg0qefcvwdx05q4vnqcwfe), narrowed by "Pushes and pull requests are
asked first from 2026-10-22" (01m5gywvgc85vj8k4txzppjz6d).

## Next steps

1. #237 (T160): the maintainer's merge.
2. T081 (the next kit of M2), after reading its task.
3. The plan of "Dependency bumps run as a script, not by hand"
   (01m5e14094yhqhsxq2g941pwrf), docs only.
4. Plan "Readers of formats between binaries ignore unknown members"
   (01m5egnpyfywadcsxpy0naz3hj); ask first to build it.
5. T077 once A12 is measured ("T077 waits for the A12 measurement",
   01m5gywvj5vj90yfedah9y670b).
6. T111, the cut-over of M1 (T060 has landed).
7. The pending notes 01m5gywvh9pdmte8k85q87wwdc (T060 rule),
   01m5gywvk15asanh438p4mmwfs (e2e output) and
   01m5gywvkw8rp4jj2cvctvzg57 (stale unit mentions), each when its
   trigger fires.

## Open for the operator

1. Merge #237 (T160). Recommended: now, it is green.
2. Measure A12 at the sitting, which unblocks T077. Recommended: at the
   next sitting.
3. Answer the three questions of "Prefer the harness's defaults and
   publish what is left" (01m5drzqvjrf3e7p7b2v68temw). Recommended:
   before more CI tooling.
4. Approve or decline the two proposals of "Validate Mermaid diagrams"
   (01m5drzqwhebfeqegfbe7gmte9). Recommended: the payload rule now, the
   CI check with the first Mermaid block.
5. ADR 0040 was merged as Accepted in #217 without a `decisions`
   approval line. Recommended: say whether that merge approves it, or
   it goes back to Proposed.
6. Paste the payload prompts left from 2026-10-21, report the sandbox
   kit's pull request for T112 (00000000008h7a0ey78frxzgfr), and say
   when the GitHub App of ADR 0011 changes the sandbox's
   authentication. Recommended: one session each.
7. Decide the scheduled pin-freshness run
   (01m5e14094yhqhsxq2g941pwrf). Recommended: with the bump plan.
8. Say whether the product skill mechanism ("The product offers
   subagents their skills at the right time",
   01m5gywvmtxk1kppkg3k1e1ae0) is planned now or waits for the sandbox
   kit's measurement. Recommended: wait for the measurement, then plan
   with T079.
9. Delete the local branch `wip/t156-part4`, whose content landed in
   #233. Recommended: delete.

## In flight

- #237 `feat/t160-snapshots-disabled-report` (T160).
- `feat/t077-julieta-kit` (T077, pushed, no PR).
- `docs/checkpoint-2026-10-22` (this checkpoint).

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
