# Child View: S3 Buckets → Objects

**Status:** Implemented
**Tier:** MUST-HAVE

## Navigation

- **Entry:** Press Enter on a bucket in the S3 bucket list
- **Frame title:** `s3-objects(N) — my-bucket-name` (with prefix path if inside a folder)
- **View stack:** S3 Buckets → S3 Objects → (detail/YAML via d/y)
- **Special behavior:** Enter on a common prefix (folder) navigates deeper into that prefix. Esc goes up one prefix level, or back to bucket list if at root.

## views.yaml

```yaml
s3_objects:
  list:
    Key:
      path: Key
      width: 36
    Size:
      path: Size
      width: 12
    Storage Class:
      path: StorageClass
      width: 16
    Last Modified:
      path: LastModified
      width: 22
  detail:
    - Key
    - Size
    - LastModified
    - StorageClass
    - ETag
    - Owner
```

## views_reference.yaml

Source struct: `s3types.Object`

```
- ChecksumAlgorithm[]
- ChecksumType
- ETag
- Key
- LastModified
- Owner.DisplayName
- Owner.ID
- RestoreStatus.IsRestoreInProgress
- RestoreStatus.RestoreExpiryDate
- Size
- StorageClass
```

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────────── s3-objects(156) — data-pipeline-logs ──────────────────────────┐
│ KEY                                  SIZE         STORAGE CLASS    LAST MODIFIED    │
│ 2024/01/15/api-requests.json.gz      2.4 MB       STANDARD        2024-01-15 09:22 │
│ 2024/01/15/batch-output.parquet      148 MB       STANDARD        2024-01-15 14:30 │
│ 2024/01/14/                          —            —               —                │
│ 2024/01/13/                          —            —               —                │
│   · · · (152 more)                                                                 │
└────────────────────────────────────────────────────────────────────────────────────┘
```

Folders (common prefixes) show `—` for Size, Storage Class, and Last Modified. Enter on a folder navigates into that prefix.

## AWS API

- `s3:ListObjectsV2` — paginated via `ContinuationToken`
- Delimiter `/` for folder-style navigation
- `Prefix` parameter set to current path within the bucket
