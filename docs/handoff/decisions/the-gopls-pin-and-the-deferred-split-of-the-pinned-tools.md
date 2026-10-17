# The gopls pin and the deferred split of the pinned tools

Decided on 2026-10-12, in #44 (gopls 0.23.0 as a mise dev tool):

- The CI cost of building gopls from source is accepted (docs/plan.md
  records the pin, with its form and no checksum).
- Splitting the pinned tools into those every run installs and those a
  developer machine installs is deferred; no trigger was recorded.
- The approval line of the pin uses the bare form until T005 lands.
