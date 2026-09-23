// t566_demo_invocation_logs_test.go — every invocation the demo lists opens
// on a log holding its START and REPORT lines.
package unit

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestDemoLambdaInvocationLogs_HoldTheirStartAndReport(t *testing.T) {
	ctx := context.Background()
	clients := demo.NewServiceClients()
	fns, err := resource.FindResourceType("lambda").Fetcher(ctx, clients, "")
	if err != nil {
		t.Fatalf("lambda list: %v", err)
	}
	def := resource.FindResourceType("lambda")
	var invChild resource.ChildViewDef
	for _, c := range def.Children {
		if c.ChildType == "lambda_invocations" {
			invChild = c
		}
	}
	inv := resource.GetChildType("lambda_invocations")
	logChild := inv.Children[0]
	opened := 0
	for i := range fns.Resources {
		pctx := resource.ResolveChildContext(invChild, &fns.Resources[i], nil)
		rows, err := inv.ChildFetcher(ctx, clients, pctx, "")
		if err != nil {
			t.Fatalf("%s invocations: %v", fns.Resources[i].ID, err)
		}
		for j := range rows.Resources {
			rid := rows.Resources[j].Fields["request_id"]
			lctx := resource.ResolveChildContext(logChild, &rows.Resources[j], pctx)
			lines, err := resource.GetChildType(logChild.ChildType).ChildFetcher(ctx, clients, lctx, "")
			if err != nil {
				t.Fatalf("log of %s: %v", rid, err)
			}
			opened++
			var start, report bool
			for _, l := range lines.Resources {
				m := l.Fields["message"]
				start = start || strings.HasPrefix(m, "START RequestId: "+rid) || strings.Contains(m, `"type":"platform.start","record":{"requestId":"`+rid+`"`)
				report = report || strings.HasPrefix(m, "REPORT RequestId: "+rid) || strings.Contains(m, `"type":"platform.report","record":{"requestId":"`+rid+`"`)
			}
			if !start || !report {
				t.Errorf("%s invocation %s: log of %d lines, START %v REPORT %v", fns.Resources[i].ID, rid, len(lines.Resources), start, report)
			}
		}
	}
	if opened == 0 {
		t.Fatal("the demo lists no invocations")
	}
}
