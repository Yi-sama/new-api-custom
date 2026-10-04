# Studio Variant Backup: 2026-10-04

Branch: `archive/studio-20261004`. Frozen source:
`D:\Open source project\github-backups\new-api-custom`.
This preserves the independently modified Studio worktree, not the newer
development tree on `main`.

`deployment-overlays/2026-10-04/opt/image-studio/` contains the exact two
selected deployed Python sources from the v28 image-size fix. Their server
paths and raw hashes are recorded in `snapshot.json`. Compose layer order
and the non-secret server inventory are in `us-server-infra` for this date.

`python -B -m unittest test_image_size test_open_webui_bridge` passed 30 tests
in `ops/image-studio` on the frozen snapshot. This is not complete browser,
paid-generation or production restore acceptance. No deployment was performed.
