package resource

import (
	"fmt"
	"regexp"
	"strconv"
)

// Universal-rule-7 `(+N)` suffix helpers. Any resource type with multiple
// coexisting §4 findings renders the top phrase in the Status column plus a
// `(+N)` suffix so the operator sees there is more to open for. These helpers
// are resource-agnostic — the suffix format is the same across dbi, ec2, ecr,
// and every future type that stacks findings.

var findingSuffixRe = regexp.MustCompile(` \(\+\d+\)$`)

// StripFindingSuffix removes any trailing " (+N)" from a Status phrase:
//
//	"publicly accessible (+1)"   → "publicly accessible"
//	"no automated backups (+2)"  → "no automated backups"
//	"storage-full"               → "storage-full"
//
// Used by Color funcs to match the base phrase; the count itself does not
// change the color — color is driven by the top (shown) phrase.
func StripFindingSuffix(s string) string {
	return findingSuffixRe.ReplaceAllString(s, "")
}

// BumpFindingSuffix increments the `(+N)` suffix on a Status phrase, or adds
// `(+1)` when no suffix is present. Used by Wave 2 enrichers when they stack
// onto an existing Wave 1 phrase:
//
//	"publicly accessible"         → "publicly accessible (+1)"
//	"no automated backups (+1)"   → "no automated backups (+2)"
//	"no automated backups (+2)"   → "no automated backups (+3)"
func BumpFindingSuffix(s string) string {
	re := regexp.MustCompile(`^(.*) \(\+(\d+)\)$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return s + " (+1)"
	}
	n, _ := strconv.Atoi(m[2])
	return fmt.Sprintf("%s (+%d)", m[1], n+1)
}
