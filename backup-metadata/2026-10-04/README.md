# Production Suite Source Backup: 2026-10-04

Branch: `production/suite-20261003-001`. Root source is the frozen inventory
from `D:\Open source project\.local-builds\production-suite-20261003-001\source\new-api`.
This sparse build-source inventory omits historical `docs` and `bin` files;
their absence here is not a deletion from the development `main` branch.

`integrations/risuai` archives 597 selected frozen RisuAI source files.
`integrations/video-canvas` archives 517 selected files from the local
Infinite Canvas project, whose source directory has no Git metadata.
Their licenses and source attribution are preserved. Video Canvas equivalence
to the deployed static assets was not established.

`release-support/production-suite-20261003-001` contains selected deployment,
packaging, migration and HTTPS-fix helpers in their original relative layout.
These are archived source artifacts, not a runnable release tree in this
checkout: the helpers' referenced generated assets, binaries and runtime
bundles are intentionally absent. Do not run them against production directly.

The captured live New API executable SHA256 is
`2c205bf966a6d300b8b3a4086329baa4af1f2234973974641b76c7e334d4a7db`.
The original `candidate-manifest.json` describes packaging-time state;
its `uploaded=false` and `activated=false` are not current deployment status.
Live Compose mounts confirm the suite is active.

The HTTPS fix includes a generated Tavern runtime, not a clean upstream source
patch. Its bundle build metadata and deploy helpers are archived, but the
generated `server.cjs` is excluded. Exact source-to-binary reproduction remains
unverified. `snapshot.json` is an inventory, not a full reproducibility claim.
No server write, build, restart, paid request or restore drill occurred here.
