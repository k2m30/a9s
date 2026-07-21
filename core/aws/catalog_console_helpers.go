// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"net/url"

	"github.com/k2m30/a9s/v3/core/consolelink"
)

// Shared ConsoleURL-building helpers used by the per-category catalog data
// files (catalog_<cat>.go) for child types whose console link needs the same
// shape regardless of which parent service produced the row.

// cloudWatchLogStreamConsoleURL builds the CloudWatch Logs console deep link
// for one stream within group, shared by every child type whose rows are (or
// belong to) a CloudWatch log stream: log_streams, log_events,
// lambda_invocations, lambda_invocation_logs, ecs_svc_logs, cb_build_logs.
// Returns "" when either input is empty — a log group/stream page with a
// missing path segment is not a working console URL.
func cloudWatchLogStreamConsoleURL(region, group, stream string) string {
	if group == "" || stream == "" {
		return ""
	}
	return consolelink.Regional(region, "cloudwatch/home?region="+region+"#logsV2:log-groups/log-group/"+url.PathEscape(group)+"/log-events/"+url.PathEscape(stream))
}
