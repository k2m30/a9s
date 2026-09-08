## Fixed

- Columns stay lined up when a value is written in characters the terminal
  paints two cells wide. A CJK service name in the cost grid, a resource
  alias on the main menu or a title in a list header used to leave its cell
  one cell short, and every column to its right slid over by one.
- A value carrying a tab or another invisible control character no longer
  shortens the cell it is painted into and shifts the rest of the row.
