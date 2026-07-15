// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package demo

import (
	"io"
	"net/http"
	"strings"
)

// registerAllHandlers registers all demo handlers onto t.
// STS and Cost Explorer are the only services still routed through the HTTP
// transport; every other service is covered by typed fakes
// (core/demo/fakes). Cost Explorer has no typed-fake path — costs.Store/
// Query are shaped around SDK JSON responses, not a Go client interface —
// so it is served here like STS (see core/demo/costs_handlers.go).
func registerAllHandlers(t *Transport) {
	registerSTSHandlers(t)
	registerCostExplorerHandlers(t)
}

// ---------------------------------------------------------------------------
// STS (awsquery — returns XML)
// ---------------------------------------------------------------------------

func registerSTSHandlers(t *Transport) {
	t.Handle("sts", "GetCallerIdentity", func(_ *http.Request) (*http.Response, error) {
		body := `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Account>123456789012</Account>
    <Arn>arn:aws:sts::123456789012:assumed-role/demo-admin/session</Arn>
    <UserId>AROADEMO123456789:session</UserId>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>demo-request-id</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`
		return &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/xml"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
}
