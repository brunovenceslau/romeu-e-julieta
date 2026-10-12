---
id: T109
title: "The dependency batch"
milestone: none
module: ci-bootstrap
fate: removed
owns: []
needs: []
after: []
ask_first: []
sitting: none
---

# T109 - The dependency batch

Removed on 2026-10-12, in the review of the re-plan: `go mod tidy`
drops a requirement that no package imports, so a batch that adds
`BurntSushi/toml` and `santhosh-tekuri/jsonschema/v6` to `go.mod` with
no importer cannot satisfy its own acceptance row, and every task that
needed it would have waited on it for nothing. Each dependency of
12 12.1's list lands with its first importer instead (rule 7 of
`docs/plan.md`): the JSON Schema validator with T051, the TOML parser
with T053. The id stays so that a citation of it still resolves.
