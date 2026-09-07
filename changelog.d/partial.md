## Fixed

- A Lambda function whose function-URL read is denied still reports a resource
  policy open to the internet, and the reverse. One denied check no longer
  throws away the other check's answer.
- An EKS cluster whose Kubernetes minor AWS does not publish, or whose version
  catalogue could not be read, now says its support status is unknown instead
  of claiming standard support.
- ECS tasks running the task definitions past the inspection cap, and EC2
  instances past the user-data cap, are now recorded as uninspected instead of
  clean, and the type's menu count carries its "+" for them. A privileged
  container on the 51st definition and a credential in the 51st instance's
  boot script were reported nowhere. The rows themselves still render as
  before: a9s has no per-row marker for an uninspected row yet.
- Backup plan selections are now read across every page, a tag selection
  written as a structured condition is matched as well as the flat tag list,
  and a plan whose selection list could not be read to the end no longer lets
  any resource be called "not covered by a backup plan".
- A backup plan that selects by excluding a tag, or by requiring two tags at
  once, no longer looks like a plan that covers everything either tag names.
  a9s cannot work out the real reach of such a selection, so it now says
  nothing about coverage instead of hiding a resource that is genuinely
  unprotected.
- An EBS volume selected by tag keeps its backup verdict after a restart. The
  volume's tags live only on data the disk cache does not keep, so a covered
  volume turned into a warning on the next launch.
- A denied EBS volume-status read no longer discards the backup and snapshot
  findings a9s had already worked out for the same volume.
- An ECR repository whose lifecycle-policy read is denied is now recorded as
  uninspected. Anything other than a genuine "no such policy" was being read
  as "the policy is there".
- A Redshift parameter group whose pages run out before require_ssl appears no
  longer reports "SSL not required", and the empty result is no longer cached
  onto every other cluster sharing the group; the check is recorded as
  uninspected and retried on the next look.
- Lambda functions, Auto Scaling groups and CodeBuild projects past the
  inspection cap are recorded as uninspected instead of clean, the same as EC2
  instances and ECS tasks, with the menu count's "+" as the visible signal.
  Past the 50th, a public function policy, a launch configuration's posture
  and a failed build were reported nowhere.
