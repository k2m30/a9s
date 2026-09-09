// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// page_delivery_test.go — delivering a page a test built by hand.
//
// A production page message is built by the fetch command that asked for it,
// which stamps the instance of the screen it was issued by. A test that hands
// the controller a messages.ResourcesLoaded it wrote itself is standing in for
// that command, so it owes the message the same identity: without it the page
// names no screen and can only be routed by resource type, which is a guess as
// soon as two lists of one type are stacked.
package unit_test

import (
	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// handlePage delivers a hand-built page to c, stamped for the list screen c is
// currently showing — the screen the test drove to get here, and the one a
// real dispatch would have named. Drop-in for c.Handle at a page call site.
func handlePage(c *app.Controller, page messages.ResourcesLoaded) (app.ViewState, []runtime.TaskRequest) {
	stamped, _ := tuitest.StampPage(c.GetListInstance(), page).(messages.ResourcesLoaded)
	return c.Handle(stamped)
}
