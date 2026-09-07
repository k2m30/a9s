## Added

- The first screen now says what it is doing. While a9s re-checks which resource types exist, the menu title counts its way through them (`verifying 12/71`), in the same place the enrichment counter already used, and clears when the sweep is done.
- A resource type whose check was refused now says why. The row keeps its last known count and reads `denied`, `expired`, `throttled` or `error` where its alias sits, so "not checked yet" no longer looks identical to "checked and refused".
- When every check fails the same way, the title says it once (`session expired`, `sweep: access denied`) instead of marking all 71 rows.

## Changed

- A check that a role is not allowed to run is now one log line for the whole resource type: the action the role lacks, how many resources it covered, and one example. It used to be one line per resource, each carrying a request id, a host id and an encoded authorization message.

## Fixed

- Opening a resource list now marks that type verified on the menu. It used to keep showing last session's count as unverified until the background sweep happened to reach it.
