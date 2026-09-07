## Fixed

- A Lambda function whose function-URL read is denied still reports a resource
  policy open to the internet, and the reverse. One denied check no longer
  throws away the other check's answer.
- An EKS cluster whose Kubernetes minor AWS does not publish, or whose version
  catalogue could not be read, now says its support status is unknown instead
  of claiming standard support.
- ECS tasks running the task definitions past the inspection cap now render "?"
  instead of clean. A privileged container on the 51st definition was reported
  nowhere.
- EC2 instances past the user-data inspection cap now render "?" instead of
  clean. The 51st running instance never had its boot script read.
- Backup plan selections are now read across every page, tag selections written
  as structured conditions are matched as well as the flat tag list, and a plan
  whose selection list could not be read to the end no longer lets any resource
  be called "not covered by a backup plan".
- An EBS volume selected by tag keeps its backup verdict after a restart. The
  volume's tags live only on data the disk cache does not keep, so a covered
  volume turned into a warning on the next launch.
- A denied EBS volume-status read no longer discards the backup and snapshot
  findings a9s had already worked out for the same volume.
- An ECR repository whose lifecycle-policy read is denied now renders "?".
  Anything other than a genuine "no such policy" was being read as "the policy
  is there".
- A Redshift parameter group whose pages run out before require_ssl appears now
  renders "?" for that check, and the empty result is no longer cached onto
  every other cluster sharing the group as "SSL not required".
