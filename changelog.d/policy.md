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
