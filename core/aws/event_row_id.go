// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// event_row_id.go holds the row identity of the event views whose API hands
// out no event ID of its own.
package aws

import (
	"fmt"
	"hash/fnv"
	"time"
)

// eventRowID is the ID of an event row read from an API that hands out no
// event ID of its own: the instant the event happened at full precision, and
// a digest of what it says. Several events of one service share an instant,
// and an event's place in the response moves with the window it is read in,
// so a row is keyed by both.
func eventRowID(at time.Time, content ...string) string {
	h := fnv.New64a()
	for _, c := range content {
		_, _ = h.Write([]byte(c))
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("%s-%016x", at.UTC().Format("20060102T150405.000000000Z"), h.Sum64())
}

// eventRowIDFromMillis is eventRowID for an API that timestamps in epoch
// milliseconds.
func eventRowIDFromMillis(ms int64, content ...string) string {
	return eventRowID(time.UnixMilli(ms), content...)
}

// eventRowIDs keys each event of one response: a log stream may carry the
// same line twice in one millisecond, and nothing in such an event tells the
// two apart, so what separates them is that one comes before the other. The
// count of identical events already seen in the response is what each row
// carries beyond its instant and its content; an event with no twin carries
// none of it and keeps the key its instant and content alone make, whatever
// arrives above it later.
type eventRowIDs struct {
	seen map[string]int
}

// at keys the next event of the response: two events of one instant carrying
// one line are distinct events, and their order in the response is the only
// thing that distinguishes them.
func (k *eventRowIDs) at(ms int64, content ...string) string {
	id := eventRowIDFromMillis(ms, content...)
	if k.seen == nil {
		k.seen = map[string]int{}
	}
	n := k.seen[id]
	k.seen[id] = n + 1
	if n == 0 {
		return id
	}
	return fmt.Sprintf("%s-%d", id, n)
}
