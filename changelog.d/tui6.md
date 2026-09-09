## Fixed

- A tag value, a description or a name that carries terminal control
  characters can no longer steer the terminal a9s is drawn in. Anyone who can
  tag a resource can put an escape sequence in a value AWS hands back
  verbatim, and it used to reach the screen intact: it recoloured the rest of
  the display, rang the bell and cost the cell columns the layout never
  counted. Such text is now made inert where it enters a9s — on a loaded page,
  on an enrichment result, on a detail screen opened directly, and in the text
  of an AWS error — so the list cell, the detail row, the filter, the frame
  title and the clipboard all show the readable value and nothing else. The
  demo carries one such tag so the behaviour is visible without an account.

- The highlight positions a text screen publishes for a search are now the
  display columns they are named for, rather than byte offsets. On a line with
  a Japanese name the two are different numbers, so anything reading them
  placed its highlight several columns away from the match.

- The detail key column is decided once, where the screen is laid out, instead
  of being recomputed by the terminal renderer from a different width on every
  frame. The values on a detail screen now start in the same place whatever is
  rendering them, including with the related panel open.
