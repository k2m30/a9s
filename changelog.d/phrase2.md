## Added

- A web ACL attached to nothing now says `not associated with any resource`
  instead of borrowing the logging finding's wording, so the two conditions
  read as the two problems they are.
- Target-health rows in a target group's child view now carry a finding and a
  colour: an unhealthy or unreachable target no longer renders green with its
  reason in a plain cell.

## Changed

- A database instance whose server certificate expires within a month now
  colours its row red rather than yellow, and says so under its own signal;
  beyond a month it stays a yellow warning.
- A CodeBuild project whose last build failed now reads `latest build failed`
  and carries the build's end date as a row in the detail view, instead of
  folding the date into the status text only when AWS reported one.
- A container repository with vulnerabilities now always names both counts,
  as in `0 critical, 5 high vulnerabilities`, rather than dropping the half
  that is zero.
- A stopped ECS task now reads its stop code in plain English —
  `stopped: essential container exited` rather than
  `stopped: EssentialContainerExited` — and the raw identifier is stated once,
  in the finding's supporting row.
- A disabled AMI now reads `disabled` and a deregistered one `deregistered`
  under one signal, rather than the doc promising only the second.
- An Aurora or DocumentDB cluster in a status a9s does not recognise now reads
  `<status>: in progress` like every other transitional status, instead of the
  bare keyword.

## Fixed

- Wave-2 findings read their wording from the signal catalog too: the helper
  every enricher emits through no longer takes a phrase at all, so the text on
  a row cannot differ from the text the docs and the detail view show.
- Every Wave-1 finding is now built in one place and reads its wording from
  the signal catalog, so the text on a row, the sentence in its detail view
  and the generated signals page can no longer drift apart. A value AWS
  supplies that contains an angle bracket no longer swallows the rest of the
  phrase.
