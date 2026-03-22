# Child View: ECR Repositories --> Images

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on a repository in the ECR Repositories list
- **Frame title:** `ecr-images(47) — payment-api`
- **View stack:** ECR --> Images --> (detail/YAML via d/y)
- **Esc** returns to ECR list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
ecr_images:
  list:
    Tag(s):
      key: image_tags
      width: 24
    Digest:
      key: digest_short
      width: 16
    Pushed At:
      path: ImagePushedAt
      width: 22
    Size:
      path: ImageSizeInBytes
      width: 12
    Scan Status:
      path: ImageScanStatus.Status
      width: 14
    Findings:
      key: finding_counts
      width: 20
  detail:
    - ImageDigest
    - ImageTags
    - ImagePushedAt
    - ImageSizeInBytes
    - ImageManifestMediaType
    - ArtifactMediaType
    - ImageScanStatus
    - ImageScanFindingsSummary
    - LastRecordedPullTime
```

Note on computed fields:
- `image_tags`: comma-separated tags from `ImageTags[]` (e.g., "v2.3.1, latest"). Untagged images show "<untagged>"
- `digest_short`: first 12 chars of `ImageDigest` after the `sha256:` prefix (e.g., `a1b2c3d4e5f6`)
- `finding_counts`: summarized from `ImageScanFindingsSummary.FindingSeverityCounts` (e.g., "0C 2H 5M" for 0 critical, 2 high, 5 medium). Empty if not scanned.

Source struct: `ecrtypes.ImageDetail`

## AWS API

- `ecr:DescribeImages` with `repositoryName`
- Paginated via `nextToken`
- **Sorting:** Results can be sorted by `imagePushedAt` descending (newest first) using `filter` parameter
- **Latency:** Fast (<1 second) for repositories with fewer than 500 images. Larger repositories may take 2-3 seconds.
- **Note:** Finding severity counts come from `ImageScanFindingsSummary` which is populated only if ECR image scanning is enabled for the repository. If not, the Scan Status and Findings columns show "— ".

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── ecr-images(47) — payment-api ──────────────────────────────────┐
│ TAG(S)                   DIGEST           PUSHED AT              SIZE     SCAN…  │
│ v2.3.1, latest           a1b2c3d4e5f6     2026-03-22 02:15       245 MB  COMP…  │
│ v2.3.0                   e5f6a7b8c9d0     2026-03-21 14:00       243 MB  COMP…  │
│ v2.2.9                   c9d0e1f2a3b4     2026-03-20 10:30       241 MB  COMP…  │
│ staging-a1b2c3d4         34a5b6c7d8e9     2026-03-22 01:45       245 MB  COMP…  │
│ <untagged>               d8e9f0a12b3c     2026-03-19 08:00       239 MB  COMP…  │
│ v2.2.8                   2b3c4d5e6f7a     2026-03-18 16:00       240 MB  COMP…  │
│ v2.2.7                   f1e2d3c4b5a6     2026-03-15 11:00       238 MB  COMP…  │
│   · · · (40 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to show Findings column:
```
│ SCAN STATUS    FINDINGS             │
│ COMPLETE       0C 0H 3M             │
│ COMPLETE       0C 0H 3M             │
│ COMPLETE       0C 0H 3M             │
│ COMPLETE       0C 1H 5M             │
│ COMPLETE       1C 3H 12M            │
│ COMPLETE       0C 0H 2M             │
│ COMPLETE       0C 0H 2M             │
```

Row coloring by scan findings severity (entire row):
- Any CRITICAL findings: RED `#f7768e`
- Any HIGH findings (no critical): YELLOW `#e0af68`
- Clean or medium/low only: PLAIN `#c0caf5`
- `<untagged>` images: DIM `#565f89`
- Scan status `FAILED`: RED `#f7768e`
- Scan status `IN_PROGRESS` or `PENDING`: YELLOW `#e0af68`

Selected row: full-width blue background overrides all coloring.

## Copy Behavior

`c` copies the full image URI (`{accountId}.dkr.ecr.{region}.amazonaws.com/{repo}:{tag}` or `@{digest}` for untagged). This is the string you paste into a Kubernetes deployment or ECS task definition.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ ECR IMAGES            GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy URI                           <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
