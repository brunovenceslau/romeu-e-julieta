# Current state

## Where we stopped

Checkpoint of 2026-10-22, second part: "Session of 2026-10-22, second
part, T160 to T163 and T081" (01m5j74wzb96z8hka0zzy40q6j). Pushes and
pull requests stay asked first ("Pushes and pull requests are asked
first from 2026-10-22", 01m5gywvgc85vj8k4txzppjz6d).

## Next steps

1. The queue chosen for the session of 2026-10-22, third part, at
   most eight tasks, the critical path first: T021; then T033, T031,
   T029 and T032, each stacked on the T021 branch; then T150, T154
   and T053. T021 alone unblocks the four stacked ones: every other
   need of theirs is merged (measured 2026-10-22).
2. T077 once A12 is measured ("T077 waits for the A12 measurement",
   01m5gywvj5vj90yfedah9y670b).
3. T161 once its open questions are answered and T088 is built (M6)
   (01m5e14094yhqhsxq2g941pwrf).
4. T111, the cut-over of M1, at sitting M1 in the config repository.
5. The pending notes 01m5gywvh9pdmte8k85q87wwdc (T060 rule),
   01m5gywvk15asanh438p4mmwfs (e2e output),
   01m5gywvkw8rp4jj2cvctvzg57 (stale unit mentions) and
   01m5gz0rpmzwvmwqt9ebd95rfq (the T159 record's name), each when its
   trigger fires.

## Open for the operator

1. Measure A12 at the sitting, which unblocks T077. Recommended: at the
   next sitting.
2. Answer the open questions of T161, the scheduled pin-freshness run
   among them. Recommended: before the next pin bump.
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
7. Say whether the product skill mechanism ("The product offers
   subagents their skills at the right time",
   01m5gywvmtxk1kppkg3k1e1ae0) is planned now or waits for the sandbox
   kit's measurement. Recommended: wait for the measurement, then plan
   with T079.
8. Change the payload's handoff check so the T159 Done record can take
   its canonical name (01m5gz0rpmzwvmwqt9ebd95rfq). Recommended: in the
   payload's next session.

## In flight

- `feat/t077-julieta-kit` (T077, pushed, no PR).
- `docs/checkpoint-2026-10-22b` (this checkpoint).

Branch and pull request state is NOT recorded here: measure it with
~/.sbx-kit/claude-home/bin/handoff_state.py.
