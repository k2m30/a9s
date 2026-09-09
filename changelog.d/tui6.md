## Fixed

- A tag value, a description or a name that carries terminal control
  characters can no longer steer the terminal a9s is drawn in. Anyone who can
  tag a resource can put an escape sequence in a value AWS hands back
  verbatim, and it used to reach the screen intact: it recoloured the rest of
  the display, rang the bell and cost the cell columns the layout never
  counted. Such text is now made inert where it enters a9s — on a loaded page,
  on an enrichment result, on a detail screen opened directly, and in the text
  of an AWS error — so the list cell, the detail row, the filter, the frame
  title and the clipboard all show the readable value and nothing else. In the
  demo, one EC2 instance is named through such a tag, so the behaviour is
  visible in its list row, its filter, its detail title and its copy without
  an account.

- The highlight positions a text screen publishes for a search are now the
  display columns they are named for, rather than byte offsets. On a line with
  a Japanese name the two are different numbers, so anything reading them
  placed its highlight several columns away from the match.

- The detail key column is decided once, where the screen is laid out, instead
  of being recomputed by the terminal renderer from a different width on every
  frame. The values on a detail screen now start in the same place whatever is
  rendering them, including with the related panel open.

- The error a failed refresh leaves over a list is inert too. It is the same
  AWS message the banner shows, and it was painted verbatim.

- A search finds the match where it is painted. A name holding a character
  whose lowercase form is a different length, a Turkish dotted I among them,
  used to shift the highlight off the word by one column for the rest of the
  line.

- The status column is as wide as the list body says it is. It was declared in
  one place and widened again in the terminal renderer, so a second lane
  reading the same body laid the column out differently.

- A Cost Explorer refusal and an identity failure read as text, not as
  terminal commands. Both quote what they refused — a dimension value, a tag
  key, a profile name — and both were painted verbatim.

- A revealed secret is painted inert while the clipboard still yields it
  exactly as stored, and the screen says so when the two differ. A secret is
  the one value taken away to be used verbatim, so it is not cleaned on its
  way to the clipboard — but painting its control characters would let it
  drive the terminal.

- Text an operator can put in AWS no longer reaches the screen raw from the
  places the first pass missed: a description read straight off the SDK
  struct, an identifier painted in a frame title, a row cached by the
  background availability probe, and the banner the terminal paints for a
  failed fetch. Identifiers are still copied exactly as AWS knows them.

- Searching a text screen highlights the match itself, whatever the text
  holds: a combining mark before or inside it, a double-width name, a letter
  whose lowercase form is a different length. The scan also runs once per
  document and query instead of on every keypress and every frame.

- Shrinking the terminal re-lays-out an open detail screen instead of
  clipping the text that was wrapped for the old width.
