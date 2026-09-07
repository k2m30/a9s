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
  NONE is reported public, KMS keys and SNS topics open to any account
  through a named service are no longer reported as scoped, and only
  AWS_IAM on the auth type restricts.
