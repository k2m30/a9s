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

## Fixed

- The reason a call failed is now read from the error's own fields, so an AWS
  message that legitimately mentions a request id keeps its words instead of
  being cut short there, and a failure whose response carries a request id, a
  host id or an encoded authorization blob never puts any of them on screen.
- An AWS response that carries no error code is no longer shown as a success:
  the row says the call failed.
- A multi-line AWS message no longer breaks the flash and the menu row it is
  rendered on.
