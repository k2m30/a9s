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
