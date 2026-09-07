## Fixed

- A cluster or security group reachable from every IPv6 address now reads as open to the internet. Reachability is decided by the address range itself, so a rule written `::/0` counts exactly as one written `0.0.0.0/0`, and a range covering the whole address space counts however it is spelled.
- A CloudFront origin whose bucket name contains dots, and one served from a China endpoint, are now checked like any other. Both were silently skipped, so a missing origin bucket and an origin without an origin access control went unreported for them.
- Volumes, buckets, Glue jobs, pipelines and EventBridge targets in the China and GovCloud partitions are no longer treated as absent. The identifiers a9s builds and matches now carry the partition of the region the session is connected to, instead of assuming the commercial one.
- A volume in a Local Zone is now matched against its parent region's backup plans. Its region was previously read off the availability zone name, which for a Local Zone names a region that does not exist, so the volume reported itself unprotected.
