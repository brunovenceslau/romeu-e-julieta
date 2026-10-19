# git versions on the host and in the sandbox

- Sitting M0 read git 2.56.0 on the host and 2.47.3 in the workload.
  Neither is installed by this product: the host's comes from the
  maintainer's package manager and is held to the floor of 05 5.1
  (REQ-0469), and the sandbox's comes with the workload, so it moves
  only with the workload pin, a reviewed diff.
- Decided on 2026-10-19: no git pin on either side, and no common
  version; `gitsafe` uses only what the older of the two supports
  (`GIT_NO_LAZY_FETCH` came with 2.45.0, internal/gitsafe/history.go). A Debian package keeps its upstream
  number while it takes security fixes, so its number is not compared
  with the host's.

Reopen: a `gitsafe` verb needs a git newer than the workload's, or a
workload pin change lowers the workload's git.
