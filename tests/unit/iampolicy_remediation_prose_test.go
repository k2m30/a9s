// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// A remediation that tells the reader to add a condition has to say what the
// condition must assert. Naming the key is not enough for the engine that
// raises the finding: a key present under Null, a negated operator or an
// IfExists variant leaves the grant exactly as open as it was, and the
// finding stays. A reader who follows the sentence literally and still sees
// the row has been told to do something that does not work.
func TestFindingDetail_ConditionRemediationSaysWhatTheConditionMustAssert(t *testing.T) {
	var violations []string
	for _, td := range resource.AllResourceTypes() {
		for _, f := range td.Findings {
			d := strings.ToLower(f.Detail)
			if !strings.Contains(d, "condition") {
				continue
			}
			if !strings.Contains(d, "add a") && !strings.Contains(d, "add an") {
				continue
			}
			if strings.Contains(d, "equal") {
				continue
			}
			violations = append(violations, string(f.Code)+" = "+f.Detail)
		}
	}
	if len(violations) == 0 {
		return
	}
	sort.Strings(violations)
	t.Errorf("%d remediation Detail(s) tell the reader to add a condition without saying it must "+
		"require the key to equal a value you expect:\n  %s",
		len(violations), strings.Join(violations, "\n  "))
}
