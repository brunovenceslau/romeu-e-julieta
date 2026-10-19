# romeu-e-julieta
Organize development environments around Docker Sandboxes: romeu on the host, julieta in the sandbox.

## The plan viewer

`go run ./tools/planviewer` writes `plan-viewer.html` at the repository root (git ignores it) and prints its path. The page is one file with no network use: each task of `docs/plan/tasks` is a node, colored by status, with its dependencies as edges. Run it again to refresh it. `go run ./tools/planviewer -ready` prints the ready ids, the same as `go run ./tools/ci plan --ready`.
