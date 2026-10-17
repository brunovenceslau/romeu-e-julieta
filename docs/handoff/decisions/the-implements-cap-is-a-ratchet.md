# The Implements cap is a ratchet

Decided on 2026-10-17, for the `plan` check of T106: an Implements line
of more than 40 ids is a finding, except for the 13 tasks that held
more when the check landed (T001, T002, T005, T008, T021, T033, T043,
T049, T051, T061, T066, T068, T069), listed in `plan.OverCap`. A test
fails when one of them comes down to the cap, so the list only
shrinks. Splitting the open ones is a re-plan, the pending item
"split-the-open-tasks-over-the-implements-cap".
