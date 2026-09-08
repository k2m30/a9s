// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

import "time"

// renderedTimeLayout is the shape a timestamp takes on screen once it carries
// a time of day, and the shape fieldpath renders a time.Time into. Parsing it
// back is what makes CanonicalValue idempotent and what lets it settle a value
// that has already been through one formatter.
const renderedTimeLayout = "2006-01-02 15:04"

// CanonicalValue renders a scalar the one way a9s shows it, and is applied
// wherever a value becomes cell or row text — the list cell and the detail
// row. It is deliberately not applied per lane: a value read off an SDK struct
// and a value read out of the Fields map land on the same screen one line
// apart, so the surface owns the conventions and neither lane has its own.
//
// Three conventions, and no fourth:
//
//   - A bool reads as Yes or No, however it arrived. AWS models some of these
//     as real bools and some as the strings "true"/"false" (a CloudTrail
//     event's ReadOnly), and an operator cannot see which.
//
//   - A timestamp reads as the day and the time of day.
//
//   - A timestamp with no time of day reads as the day alone. AWS reports
//     several fields at day granularity (a secret's last-accessed date, an
//     AMI's deprecation time), and rendering one as "<date> 00:00" claims a
//     midnight nobody measured.
//
// Numbers are deliberately left alone: a decimal is an engine version at least
// as often as it is a number, and rendering "1.10" as "1.1" is a wrong answer
// rather than a tidier one. A column whose two declarations disagree about a
// number is fixed in the defaults instead.
//
// It applies to one scalar. A line lifted out of a raw document — a CloudTrail
// event's YAML dump, a policy body — is the document's own text and reaches
// the screen whole, so no convention rewrites a word inside it.
func CanonicalValue(v string) string {
	switch v {
	case "true":
		return "Yes"
	case "false":
		return "No"
	}
	// ponytail: midnight stands for "no time of day". AWS reports day-granular
	// fields exactly that way and a measured instant almost never lands there,
	// but a real event in the first minute of a day does render as its day.
	// Carry the granularity on the field if that ever matters.
	for _, layout := range []string{time.RFC3339, renderedTimeLayout} {
		t, err := time.Parse(layout, v)
		if err != nil {
			continue
		}
		// The hour and the minute, which is all the rendered shape carries.
		// Testing the seconds too would be more precise and not idempotent: a
		// value measured at 00:00:59 renders as "<date> 00:00", and a second
		// pass over that — a cached cell re-entering the surface — would then
		// disagree with the first. Two passes must say the same thing.
		if t.Hour() == 0 && t.Minute() == 0 {
			return t.Format(time.DateOnly)
		}
		return t.Format(renderedTimeLayout)
	}
	return v
}
