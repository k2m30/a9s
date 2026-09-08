## Fixed

- Refreshing a list while it was still loading no longer lets the older result win. A list that was opened and then refreshed before the first fetch came back could end up showing, and saving, the rows the refresh had already replaced.
- Opening a list, going back and opening it again now re-checks it against AWS instead of showing the earlier visit's rows as if they were current. The list still appears instantly, marked as refreshing while the check runs.
- A refresh that comes back with fewer rows than the list is known to have no longer shrinks the count in the title. A list of 55 whose first page holds 50 stays "55+" instead of dropping to "50+".
