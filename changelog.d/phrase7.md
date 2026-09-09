## Changed

- Every attention signal a9s can raise now carries a sentence saying what the
  condition means for your workload and what to do about it. Two hundred and
  sixty-eight signals previously showed a two- or three-word status and nothing
  else, on the detail view and in the generated signal tables alike.

- A status phrase now names one condition at one colour. Words that used to
  appear in both yellow and red say which is which: an instance AWS stopped
  itself reads `stopped by AWS`, a database you stopped reads
  `stopped (storage still billed)`, a secret inside its recovery window reads
  `scheduled for deletion`, and a database or warehouse with a routable address
  reads `public endpoint` rather than borrowing the wording S3 uses for objects
  anyone can download. Redshift's two different `modifying` states, and the
  certificate expiry tiers on certificates and database instances, now read
  differently from each other too.

- A signal that only greys a row no longer declares an explanation nothing
  shows. The seven grey signals that carried one now carry the phrase alone,
  which is all the list and the status cell ever rendered.

- A placeholder in a signal's list text is written one way. `latest run <status>`
  and `shard <shard id>` replace the two spellings that shouted, and the two
  CloudTrail delivery signals no longer name an SDK field where the value goes.
  What you see on a row is unchanged; this is the wording in the signal tables.

- A CloudTrail delivery failure reads as what went wrong rather than as the
  API's name for it. The status cell led with the error code (`AccessDenied:`)
  and pushed the sentence that names the cause off the end of the line.

- A timestamp shown in a status cell reads as the day, the way every other
  date on the screen does. One trail row showed the raw value the SDK returned.

- Fifty attention signals that described a condition and stopped now say what
  to do about it, or say plainly that nothing can be done for a state nobody
  can act on. A row whose details could not be read says which permission to
  grant, instead of only that the name is all a9s can show.

- The six broken Redshift cluster states read as what is wrong rather than as
  the colour and the API's status word: `out of storage`, `node hardware
  failed`, `parameter group rejected`, `restore did not complete`, `subnet
  group cannot host the cluster`, `encryption key store unreachable`.

- Twenty-nine signal explanations that stated something AWS does not do have
  been rewritten against the service documentation. Among them: a Step
  Functions workflow without logging still keeps ninety days of execution
  history unless it is an express workflow, adding brokers to an MSK cluster
  does not restart the ones already running, a Kinesis stream that is updating
  is only resharding some of the time, and a Simple Email Service identity that
  cannot send is usually incomplete rather than switched off.

### Fixed

- Six signals no longer fire on a healthy resource:

  - A queue using the encryption Amazon SQS manages for you is no longer
    reported as unencrypted. Only a queue with neither that nor a key of your
    own is.
  - An SNS topic without a key of your own is described as what it is, a topic
    whose message encryption uses a key you cannot audit, rather than as one
    whose messages sit in the clear.
  - An EKS cluster on Kubernetes 1.28 or newer is no longer told its secrets
    are unencrypted. Those versions encrypt secrets with an AWS-owned key
    without being asked; only an older cluster has none.
  - A VPC whose subnets are covered by flow logs no longer reads as having no
    record of its traffic. A flow log attached to a subnet or a network
    interface writes the same records as one attached to the VPC, and all three
    now count.
  - A task the scheduler stopped during a deployment, or that Spot reclaimed,
    is no longer red. Those are the platform doing its job, and they now grey
    out like any other clean stop; a task that failed to start or whose
    essential container exited is still red.
  - A CloudFront distribution in front of an S3 static-website endpoint is no
    longer told to talk to that origin over HTTPS. The endpoint only serves
    HTTP, so there is nothing to change.

- The detail row under a CloudFront distribution reaching its origin without
  TLS is labelled in words. It read `OriginProtocolPolicy`, which is the name
  of the API field rather than anything an operator says.

- Demo mode now shows the corrected checks working. It carries a queue using
  the encryption SQS manages, a cluster on a version that encrypts secrets by
  itself, a VPC covered by a flow log on its subnet, a distribution in front of
  an S3 website endpoint, and a task Spot reclaimed, none of which is flagged,
  beside the rows that still are.
