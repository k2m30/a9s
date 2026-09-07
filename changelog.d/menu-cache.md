## Fixed

- The main menu no longer dims every type whose count came from the cache
  while the live sweep re-checks it. A cached count is a real count and its
  list opens on Enter, so the row renders like any other; only a type the
  sweep confirmed empty is dimmed.
