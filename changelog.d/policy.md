## Fixed

- A policy whose only condition demands a key be absent, or names it under a
  negated, IfExists or unrecognised operator, no longer reads as scoped: the
  grant it leaves open to every caller is now reported public.
- A grant to every account's root user in the China, GovCloud and isolated
  partitions is now recognised as public, as the commercial partition already
  was.
- A service trust is now counted as protected against the confused-deputy
  problem only when the source key is compared against a concrete value, so a
  trust conditioned on that key being absent is reported unscoped.
- A condition on a key that scopes the request rather than the caller no
  longer counts as scoping the caller: a function URL pinned to auth type
  NONE is reported public, a KMS key open to any account through a named
  service and an SNS topic open to any account for a named delivery endpoint
  are no longer reported as scoped, and only AWS_IAM on the auth type
  restricts.
- A condition that compares the caller's ARN against a pattern with a wildcard
  account no longer counts as scoping, because every account matches it; such
  a policy is now reported open to anyone.
- A finding's explanation in the detail view now wraps to the panel instead of
  being cut at its edge, so the sentence that says what to change reaches the
  reader without turning wrap on.
