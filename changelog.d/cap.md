## Fixed

- A resource whose posture check ran out of pages before reaching it now says `not inspected` instead of rendering as healthy. On large accounts the checks for EC2 instances, EBS volumes, RDS instances, DocumentDB clusters and backup plans stop after a fixed number of pages, and every row past that point used to look clean.
- A check that finished its last batch without an error no longer erases the fact that an earlier part of the same check was cut short. The list could report a complete count when it had really seen only part of the account.
- A supporting line whose own text ends in wording like `+3 more` is no longer swallowed and miscounted when a second batch adds lines to the same finding.
- The backup-plan coverage check no longer reads tags whose result it then throws away.
- A backup plan with more than five failed jobs in the last day no longer shows five of them and drops the rest without saying so. The detail now lists as many as any other finding does and closes with `… +K more` for the remainder, with the most recent failure and the partial-job count kept at the top where a long list cannot push them off.

## Changed

- The demo account now has a target group with more failing targets than one detail view shows, so the closing `… +2 more` line of a long finding is visible in `--demo`.
