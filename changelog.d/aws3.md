## Fixed

- A finding's wording now agrees with its own number: a certificate one day from expiry reads "expires in 1 day", a single failed backup job reads "1 job failed", and one exposed load-balancer listener reads "port 80 in the clear" instead of "ports 80".
- A CloudTrail event whose error code was not recorded no longer renders "failed:" with nothing after it.
- A bucket that refused two different permission checks now names both in the failure line, instead of reporting only the first and failing again once that one was granted.
- A backup or DocumentDB scan stopped by a refused page now says which page it was, instead of offering "page 3" as if it were a resource you could go and look at.
- A node group or cluster whose details could not be read keeps its warning colour after a cache reload, even if the wording of the status cell changes.
- The SNS list's Topic Name column shows the topic's name instead of its ARN, and the subscription list's Confirmed column shows whether the endpoint confirmed instead of repeating the subscription ARN. Both corrections reach an existing installation on the next launch: a view file an earlier build wrote takes the new source for a column you have not edited yourself.
- A DocumentDB cluster AWS reported no status for no longer renders a bare ": in progress", and an EKS cluster past standard support says so even when the version number is missing from a restored row.
- A security group opening one sensitive port now reads "port 22 open to 0.0.0.0/0" instead of "ports 22", and the instance exposure check reads the group's port list from a field of its own rather than by taking that sentence apart.
- A backup plan with one job in the window reads "partial: 1 of 1 resource skipped" instead of "1 resources".
- An auto scaling group with one unhealthy instance reads "1 unhealthy instance" instead of "1 unhealthy instance(s)".
- An SNS subscription AWS returned no ARN for no longer shows as confirmed on the by-topic subscription list, and a deleted one says so; both subscription lists now read the confirmation state the same way.
