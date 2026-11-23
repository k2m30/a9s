package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// ProbeWave2 runs shortName's Wave 2 pass the way a sweep does: rows retained
// in the session's RowStore, the enricher reached only through the runner, so
// the runner's row filter, its declared reads and its prefetch all apply.
// Each seed is a set of lists the session has already loaded.
func ProbeWave2(t *testing.T, shortName string, clients *awsclient.ServiceClients, rows []resource.Resource, seeds ...resource.ResourceCache) runtime.ProbeEnrichmentResult {
	t.Helper()
	sess := session.New()
	sess.Clients = clients
	core := runtime.New(sess, catalog.All())
	for _, seed := range seeds {
		for name, entry := range seed {
			core.Session().RowStore.Observe(name, entry.Resources, &resource.PaginationMeta{PageSize: len(entry.Resources), TotalHint: len(entry.Resources)}, session.OriginProbe, false)
		}
	}
	core.Session().RowStore.Observe(shortName, rows, nil, session.OriginProbe, false)
	return core.ProbeEnrichment(context.Background(), clients, shortName)
}

// Wave2Result is ProbeWave2's answer as the enricher result the folds take;
// a runner error fails the test.
func Wave2Result(t *testing.T, shortName string, clients *awsclient.ServiceClients, rows []resource.Resource, seeds ...resource.ResourceCache) awsclient.IssueEnricherResult {
	t.Helper()
	res := ProbeWave2(t, shortName, clients, rows, seeds...)
	if res.Err != nil {
		t.Fatalf("%s Wave 2: %v", shortName, res.Err)
	}
	return awsclient.IssueEnricherResult{
		Findings:         res.Findings,
		AttentionDetails: res.AttentionDetails,
		FieldUpdates:     res.FieldUpdates,
		TruncatedIDs:     res.TruncatedIDs,
		Truncated:        res.Truncated,
	}
}
