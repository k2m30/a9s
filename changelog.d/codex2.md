## Changed

- On a large account the public-snapshot check now reads up to 50 pages instead of 10, so fewer snapshots fall past it and read `?`.

## Fixed

- Two AWS profiles whose names differ only around a hyphen (`team-` with `us-east-1`, `team` with `-us-east-1`) shared one cache directory and overwrote each other's cached rows. Each pair now gets its own directory.
- A profile or region name containing a slash, backslash or space no longer reads the cache directory of a different pair whose name happens to sanitize the same way. Such a pair starts cold once and keeps its own cache from then on.
- An IAM role whose inline policies a9s is not allowed to read no longer renders as inspected and clean. The refusal is reported with the rest of the list's partial failures, and the role's policy facts read `?`.
- A backup plan seen once on an early page no longer counts as fully checked when the job walk stops at its limit: a failed job could be on a page a9s never read, so the plan reads `?`.
- A snapshot past the limit of the public-share walk reads `?` instead of private.
- An IAM policy that grants everything on everything is now flagged whether its `Statement` is written as an object or as a list. It was only detected as a list.
- A VPC endpoint policy that grants a wildcard action to everyone but confines it to named buckets is no longer reported as open to anyone. The grant must be unrestricted on all three of principal, action and resource.
- `PowerUserAccess` is no longer reported as an administrator policy. AWS's definition of it withholds IAM, Organizations and Account, so its holder cannot grant itself permissions.
- S3 throttling (`SlowDown`) is now shown as throttling, and EC2's `UnauthorizedOperation` as access denied, on every surface that phrases a failure. Both used to read as a generic error.
- A list column whose title matches two stored field names differing only by case now shows the same value on every start instead of one at random.
- The role's policy list no longer highlights a customer-managed policy in red just because someone named it `AdministratorAccess`. AWS's own policies are matched by their ARN, in every partition.
- A role's inline policy written with a single `Statement` object no longer loses the resources it names, so the pivots that read them resolve.
- An IAM role that is deleted while a9s is looking at it no longer reports as a failed lookup. It is treated as the ordinary race it is, like every other resource type.
- A refused call while checking IAM roles or groups is now reported instead of passing silently. Those two checks built a failure list and threw it away.
- A resource that is deleted while a9s is checking it no longer counts as a failed check anywhere. The row still says it was not inspected.
