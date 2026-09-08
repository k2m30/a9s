## Fixed

- A log group with no retention policy now shows "never expire" in the Retention column instead of leaving it blank next to the warning that explains it; a group with one shows "30 days".
- A session that lost its region no longer builds commercial ARNs: the backup pivot on a bucket and the CloudFormation pivot on a Glue job say "cannot tell" instead of reporting a confident zero.
- Regions in the isolated, secret and European Sovereign partitions are recognised as their own partitions, read from the SDK's own catalogue, so an ARN built for them is no longer a commercial one that matches nothing.
- Two failures that arrive together are now shown on one line instead of spilling a second line into a flash or a menu row.
- A service response the SDK could not model is shown by its leading clause, so a host or a socket address in it cannot push the failure off the line.
- An ECS service that cannot place a task now shows AWS's own reason for it, so the operator can tell insufficient memory from ports in use without leaving a9s; the same applies to a failing load balancer health check, which now shows the codes AWS reported.
- A failed ECS deployment shows its rollout reason on a row of its own, the same way the service events do, instead of crammed behind a colon with AWS's raw status word in it.
- The demo account now has an ECS service that cannot place its tasks and one whose tasks fail their health checks, so both read the same as they would against a real account.
- A local file a9s cannot read no longer reports itself as a network failure; a missing, unreadable or wrong-shaped file now says what the operating system said.
