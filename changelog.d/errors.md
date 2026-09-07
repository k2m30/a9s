## Added

- A TLS failure, a DNS failure and a refused connection now say which one
  happened, in the menu row, the sweep title and the error log. They used to
  share one word, "transport", though the fix for each is different: a
  certificate is a proxy or a wrong clock, a resolver failure is name
  resolution, and a refused connection is the network path.

## Changed

- Every entry in the `!` error log now reads the same way: what was being
  fetched, then why it failed, with the region named only when the region is
  the reason. A partial result, a service the region does not offer and a
  failed connect used to arrive in three different shapes.
- A failed connect now shows the same sentence in the flash and in the error
  log instead of the raw error in one and its own phrasing in the other.
- An AWS error shown on the status bar now says the reason the call failed
  rather than the raw AWS message, so a denial no longer fills the line with
  an encoded authorization blob.
- A probe that fails with nothing to show now names the reason on the status
  bar. It used to show the internal outcome and class names, as in
  "probe ec2: failed: transport".

## Fixed

- The reason a call failed is now read from the error's own fields, so an AWS
  message that legitimately mentions a request id keeps its words instead of
  being cut short there, and a failure whose response carries a request id, a
  host id or an encoded authorization blob never puts any of them on screen.
- An AWS response that carries no error code is no longer shown as a success:
  the row says the call failed.
- A multi-line AWS message no longer breaks the flash and the menu row it is
  rendered on.
- When the last resource type checked at startup is the one that fails, its
  message stays on screen. Finishing the check used to clear it in the same
  breath, so that one failure was never seen.
