# T038 slices 1 to 3 built from main

Done on 2026-10-25. T038 (issue #271) is built in slices from main,
without the branch `feat/t038-render-kits`, as the checkpoint
01m5rtp0fp919zpq1f3gs67ngy records. Slices 1 and 2 each had a code review and a security audit
before its pull request, and their findings were fixed in its second
commit.

- Slice 1, #285: `render.NewBin`, `Bin.WriteTo` and `Bin.Write` write
  `julieta-linux-<arch>` and `SHA256SUMS` into `.romeu/bin` through
  `os.Root` with renameio and a directory fsync (03 3.8). The audit
  showed that `O_NOFOLLOW` gives no protection through
  `os.Root.OpenFile`; renameio's unique temp file replaced it. CI green.
- Slice 2, #286: `render.ProductKits` materializes the product kits of
  `kits.FS` into the candidate and renders `julietaBin`, checked against
  the arg's declared pattern at sync. CI green.
- Slice 3, branch `feat/t038-personal-kits` (stacked on #286), commit
  db0b3e46: the personal kits from the config commit through
  `gitsafe.Fsck` and `gitsafe.WalkTree`, the five refused capability
  types of REQ-0574, and every kit read and checked before any is
  written. Built and pushed; not yet reviewed or audited, and no pull
  request. It renames `ProductKits` to `KitMaterializer`.

The open question slice 3 raised is the pending item
01m5rzc41dqzdqkkfb4f88xfyj "Which kinds a personal kit may declare".
