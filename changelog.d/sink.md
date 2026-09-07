## Added

- A row a9s could not inspect now says so. Its Status cell reads `not inspected` instead of a lifecycle word, and its detail view carries an Attention entry reading `Not inspected`, saying the checks for that row did not answer because of a cap or an API error. Its colour is unchanged: an unknown posture is not an issue, it is just not a clean bill of health.
- Types with more resources than one sweep inspects now mark the whole tail. Past the 50th row a list no longer shows rows that look inspected and healthy when nothing looked at them.

## Removed

- The `!` / `~` glyph on a list row. A row carrying a finding is already coloured for it, so the glyph could never appear.
