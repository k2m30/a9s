package unit

// qa_resourcelist_banner_truncated_zero_test.go — P2.3 regression test.
//
// Bug: findingsBanner() early-returns "" when enrichmentIssueCount == 0 &&
// len(findingsByID) == 0, which silently suppresses the case where
// Truncated=true but Findings=empty (enricher hit throttling and only scanned
// clean resources). The user gets no signal that the count is a lower bound.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// truncatedZeroTypeDef returns a minimal ResourceTypeDef for truncation tests.
func truncatedZeroTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
}

func truncatedZeroResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-tz-01", Name: "server-01", Status: "running",
			Fields: map[string]string{"name": "server-01", "state": "running"},
		},
		{
			ID: "i-tz-02", Name: "server-02", Status: "running",
			Fields: map[string]string{"name": "server-02", "state": "running"},
		},
		{
			ID: "i-tz-03", Name: "server-03", Status: "running",
			Fields: map[string]string{"name": "server-03", "state": "running"},
		},
		{
			ID: "i-tz-04", Name: "server-04", Status: "running",
			Fields: map[string]string{"name": "server-04", "state": "running"},
		},
		{
			ID: "i-tz-05", Name: "server-05", Status: "running",
			Fields: map[string]string{"name": "server-05", "state": "running"},
		},
	}
}

// buildTruncatedZeroModel returns a loaded ResourceListModel with plenty of
// viewport height so rendering reaches the banner code path.
func buildTruncatedZeroModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	td := truncatedZeroTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 30)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    truncatedZeroResources(),
	})
	return m
}

// TestResourceList_BannerShownWhenTruncatedZeroFindings verifies that when
// enrichmentTruncated=true and there are no findings at all, the banner still
// appears to inform the user that the count is a lower bound.
//
// Current bug: findingsBanner() returns "" immediately when
// enrichmentIssueCount==0 && len(findingsByID)==0, before it ever checks
// m.enrichmentTruncated — so the truncation signal is lost.
func TestResourceList_BannerShownWhenTruncatedZeroFindings(t *testing.T) {
	m := buildTruncatedZeroModel(t)

	// count=0, truncated=true, no findings — models the case where the enricher
	// was throttled before it could scan any resources.
	m.SetEnrichmentState(0, true, nil)

	out := m.View()
	plain := stripANSI(out)

	// The banner must signal truncation.  Accept any phrasing that conveys
	// "lower bound" or "truncated".
	hasBanner := strings.Contains(plain, "lower bound") ||
		strings.Contains(plain, "truncated") ||
		strings.Contains(plain, "Truncated") ||
		strings.Contains(plain, "lower-bound")

	if !hasBanner {
		t.Errorf("View() should show a truncation banner when Truncated=true && Findings=empty "+
			"(P2.3 bug: findingsBanner early-returns '' before checking enrichmentTruncated)\nGot:\n%s", plain)
	}
}
