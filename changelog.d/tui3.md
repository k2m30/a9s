## Fixed

- Columns stay lined up when a value is written in characters the terminal
  paints two cells wide. A CJK service name in the cost grid, a resource
  alias on the main menu or a title in a list header used to leave its cell
  one cell short, and every column to its right slid over by one.
- A value carrying a tab or another invisible control character no longer
  shortens the cell it is painted into and shifts the rest of the row.
- The header keeps the profile and region when a status message is written in
  wide characters. The message was cut to fit by counting characters rather
  than the width they paint, so a message twice as wide as its slot squeezed
  the account out of the header.
- A detail field named in non-Latin characters no longer pushes every value on
  the screen to the right. The key column was sized by the bytes the name takes
  to store instead of the columns it paints.
