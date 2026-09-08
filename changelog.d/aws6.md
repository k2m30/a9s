## Fixed

- Detail screens no longer show a raw AWS constant beside the readable form of
  the same fact. Thirty-five fields across thirty-two resource types now read
  as words wherever they appear, including the ones no list column shows and
  which therefore had no way to ask: a Managed Airflow environment's endpoint
  management and last update status, an API Gateway API's protocol, an ECS
  service's launch type and status, and a node group's status. A field reads
  the same way in its column and in its detail row, because both read one
  declaration on the resource type.

- A CodeBuild project's source type, an ECR repository's tag mutability and a
  Kinesis stream's mode read as words on a real account. They were correct in
  the demo and raw on screen: the per-resource view files an installation
  carries name those columns by their AWS field path, and the readable-wording
  declaration did not recognise that spelling.

- A list column shows the same value on a real account as in the demo. The
  column set was resolved by two different rules — one for an installation
  with view files, which is all of them, and another for the demo — so a cell
  could be right in `--demo` and wrong on screen. The Handler column was
  missing from the Lambda list on every real account for the same reason.

- A log event's status reads as a word. The classification is a9s's own
  judgement about the line rather than anything CloudWatch returned, and it
  showed as `ERROR`, `WARN`, `REPORT` or `META`. The same applies to a
  CloudTrail event's outcome, which showed the raw AWS error code, and to a
  role whose trust policy allows anyone, which read `WILDCARD`.

- A folder inside an S3 folder now opens. The second level of the object
  browser dispatched nothing at all, so the screen stayed where it was.

- An S3 folder row no longer renders three blank cells. A folder has no size,
  no last-modified instant and no storage class, and a blank cell reads the
  same as a value nobody managed to fetch. Those cells say so now, and folders
  still sort first.

- A related panel no longer answers "none" for a resource the demo account
  does not model. Drilling into one now says there is no such thing, which is
  what it always was.

- A request a9s builds wrong is no longer reported as a resource that went
  away. Auto Scaling, CloudFormation and Athena answer a missing resource with
  the same error code they use for a malformed request, so a bad call was
  silently marked "not inspected" and the operator was told nothing had
  failed.

- An AMI row no longer carries its id under four field names, and a Transfer
  agreement no longer carries each resolved partner profile under two. One
  fact under two names is a fact that can disagree with itself.

## Changed

- Per-resource view files in `~/.a9s/views/` are regenerated on next start.
  Whether a field reads as words is a property of the resource type now, not
  of one column, so the `humanize` key a column could carry is no longer read.
  Widths, sources and column order an operator has changed stay theirs.
