a9s uses **read-only** AWS API calls exclusively. The following managed policies provide sufficient access:

- `ReadOnlyAccess` (broad read-only access to all services)
- Or individual service policies like `AmazonEC2ReadOnlyAccess`, `AmazonS3ReadOnlyAccess`, etc.

a9s will gracefully handle permission errors -- resources you don't have access to will show an error message instead of crashing.
