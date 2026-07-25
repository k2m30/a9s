//go:build integration

package integration

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

type fullIntegrationCountExpectation struct {
	count     int
	truncated bool
}

type fullIntegrationRelatedHopScenario struct {
	name              string
	sourceType        string
	firstTargetType   string
	firstDisplayName  string
	returnTargetType  string
	returnDisplayName string
}

type fullIntegrationCountResolver func(t *testing.T, resourceType string) fullIntegrationCountExpectation

func fullIntegrationCountExpectationsFromCounts(counts map[string]int) map[string]fullIntegrationCountExpectation {
	expected := make(map[string]fullIntegrationCountExpectation, len(counts))
	for shortName, count := range counts {
		expected[shortName] = fullIntegrationCountExpectation{count: count}
	}
	return expected
}

func fullIntegrationStaticCountResolver(expected map[string]fullIntegrationCountExpectation) fullIntegrationCountResolver {
	return func(t *testing.T, resourceType string) fullIntegrationCountExpectation {
		t.Helper()
		exp, ok := expected[resourceType]
		if !ok {
			t.Fatalf("missing first-page expectation for %s", resourceType)
		}
		return exp
	}
}

func fullIntegrationLiveCountResolver(clients *awsclient.ServiceClients) fullIntegrationCountResolver {
	return func(t *testing.T, resourceType string) fullIntegrationCountExpectation {
		t.Helper()
		return fullIntegrationExpectedFirstPageCount(t, clients, resourceType)
	}
}

func fullIntegrationExpectedFirstPageCount(t *testing.T, clients *awsclient.ServiceClients, resourceType string) fullIntegrationCountExpectation {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rt := resource.FindResourceType(resourceType)
	if rt == nil {
		t.Fatalf("unknown resource type %s", resourceType)
	}
	pf := resource.GetPaginatedFetcher(resourceType)
	if pf == nil {
		t.Fatalf("resource %s (%s) has no paginated fetcher; full integration test cannot show a count", rt.ShortName, rt.Name)
	}
	result, err := pf(ctx, clients, "")
	if err != nil && len(result.Resources) == 0 {
		// Rows + composite error together are the designed E5 partial-
		// success outcome (listed-but-denied demo witnesses); only a
		// row-less error is a harness failure.
		t.Fatalf("fetcher for %s (%s) failed: %v", rt.ShortName, rt.Name, err)
	}
	truncated := result.Pagination != nil && result.Pagination.IsTruncated
	return fullIntegrationCountExpectation{count: len(result.Resources), truncated: truncated}
}

func fullIntegrationExpectedFirstPageCounts(t *testing.T, clients *awsclient.ServiceClients) map[string]fullIntegrationCountExpectation {
	t.Helper()
	expected := make(map[string]fullIntegrationCountExpectation)
	for _, rt := range resource.AllResourceTypes() {
		expected[rt.ShortName] = fullIntegrationExpectedFirstPageCount(t, clients, rt.ShortName)
	}
	return expected
}

func fullIntegrationRunAllResourceBaseline(t *testing.T, clients *awsclient.ServiceClients, newModel func() tui.Model, resolveExpected fullIntegrationCountResolver) {
	t.Helper()
	for _, rt := range resource.AllResourceTypes() {
		rt := rt
		t.Run(rt.ShortName, func(t *testing.T) {
			m := newModel()
			fullIntegrationRunResourceBaseline(t, clients, m, resolveExpected, rt)
		})
	}
}

func fullIntegrationRunResourceBaseline(t *testing.T, clients *awsclient.ServiceClients, m tui.Model, resolveExpected fullIntegrationCountResolver, rt resource.ResourceTypeDef) {
	t.Helper()
	expected := resolveExpected(t, rt.ShortName)

	loaded := fullIntegrationOpenResourceList(t, &m, rt.ShortName)
	frameDisplayed, _ := fullIntegrationFindFrameDisplayCount(fullIntegrationStripANSI(fullIntegrationViewContent(m)), rt.ShortName)
	t.Logf("list %s: displayed=%s loaded=%d expected=%s", rt.ShortName, frameDisplayed, len(loaded.Resources), fullIntegrationExpectedDisplay(expected))
	if got := len(loaded.Resources); got != expected.count {
		t.Fatalf("%s list loaded %d resources, expected %d from fetcher", rt.ShortName, got, expected.count)
	}
	fullIntegrationAssertFrameContains(t, m, fullIntegrationFrameCount(rt.ShortName, expected))

	if expected.count == 0 {
		return
	}
	defs := resource.GetRelated(rt.ShortName)
	if len(defs) == 0 {
		return
	}

	selected, relatedResults, ok := fullIntegrationDescribeSelectedResourceMaybeRelated(t, &m, rt.ShortName)
	if !ok {
		return
	}
	detailContext := fullIntegrationDetailContext(rt.ShortName+" baseline detail", selected)
	t.Logf("%s selected resource: id=%s name=%q", rt.ShortName, selected.ID, selected.Name)
	expectedRelated := fullIntegrationExpectedRelatedCounts(t, clients, rt.ShortName, selected)
	coldUnknown := fullIntegrationAssertRelatedResults(t, rt.ShortName, expectedRelated, relatedResults, detailContext)
	fullIntegrationAssertRelatedCountsInView(t, m, rt.ShortName, expectedRelated, coldUnknown, detailContext)
}

func fullIntegrationRunRelatedHopScenario(t *testing.T, clients *awsclient.ServiceClients, m *tui.Model, expectedTopLevel map[string]fullIntegrationCountExpectation, scenario fullIntegrationRelatedHopScenario) {
	t.Helper()
	sourceExpected, ok := expectedTopLevel[scenario.sourceType]
	if !ok {
		t.Fatalf("%s: missing first-page expectation for %s", scenario.name, scenario.sourceType)
	}
	if sourceExpected.count == 0 {
		t.Skipf("%s: %s has zero resources; cannot run related-hop scenario", scenario.name, scenario.sourceType)
	}

	sourceLoaded := fullIntegrationOpenResourceList(t, m, scenario.sourceType)
	sourceDisplayed, _ := fullIntegrationFindFrameDisplayCount(fullIntegrationStripANSI(fullIntegrationViewContent(*m)), scenario.sourceType)
	t.Logf("%s source list %s: displayed=%s loaded=%d expected=%s", scenario.name, scenario.sourceType, sourceDisplayed, len(sourceLoaded.Resources), fullIntegrationExpectedDisplay(sourceExpected))
	if got := len(sourceLoaded.Resources); got != sourceExpected.count {
		t.Fatalf("%s: %s list loaded %d resources, expected %d from fetcher", scenario.name, scenario.sourceType, got, sourceExpected.count)
	}
	fullIntegrationAssertFrameContains(t, *m, fullIntegrationFrameCount(scenario.sourceType, sourceExpected))

	firstResource, firstResults := fullIntegrationDescribeSelectedResource(t, m, scenario.sourceType)
	sourceContext := fullIntegrationDetailContext(scenario.name+" source detail", firstResource)
	t.Logf("%s source selected resource: id=%s name=%q", scenario.name, firstResource.ID, firstResource.Name)
	expectedFirst := fullIntegrationExpectedRelatedCounts(t, clients, scenario.sourceType, firstResource)
	coldUnknownFirst := fullIntegrationAssertRelatedResults(t, scenario.sourceType, expectedFirst, firstResults, sourceContext)
	fullIntegrationAssertRelatedCountsInView(t, *m, scenario.sourceType, expectedFirst, coldUnknownFirst, sourceContext)

	relatedResource, relatedResults := fullIntegrationEnterRelatedSingleDetail(t, m, scenario.firstTargetType, scenario.firstDisplayName)
	firstRelatedContext := fullIntegrationDetailContext(scenario.name+" first related detail", relatedResource)
	t.Logf("%s first related selected resource: id=%s name=%q", scenario.name, relatedResource.ID, relatedResource.Name)
	expectedRelated := fullIntegrationExpectedRelatedCounts(t, clients, scenario.firstTargetType, relatedResource)
	coldUnknownRelated := fullIntegrationAssertRelatedResults(t, scenario.firstTargetType, expectedRelated, relatedResults, firstRelatedContext)
	fullIntegrationAssertRelatedCountsInView(t, *m, scenario.firstTargetType, expectedRelated, coldUnknownRelated, firstRelatedContext)

	returnCount := expectedRelated[scenario.returnDisplayName]
	if returnCount <= 0 {
		t.Fatalf("%s: related %s has %s count %d; test needs a navigable return row", scenario.name, scenario.firstTargetType, scenario.returnDisplayName, returnCount)
	}
	fullIntegrationEnterRelatedList(t, m, scenario.returnTargetType, scenario.returnDisplayName)
	returnDisplayed, _ := fullIntegrationFindFrameDisplayCount(fullIntegrationStripANSI(fullIntegrationViewContent(*m)), scenario.returnTargetType)
	t.Logf("%s return list %s: displayed=%s expected=%d", scenario.name, scenario.returnTargetType, returnDisplayed, returnCount)
	fullIntegrationAssertFrameContains(t, *m, fmt.Sprintf("%s(%d)", scenario.returnTargetType, returnCount))

	if returnCount > 1 {
		*m, _ = fullIntegrationApplyMsg(*m, fullIntegrationKeyPress("j"))
	}
	returnResource, returnResults := fullIntegrationDescribeSelectedResource(t, m, scenario.returnTargetType)
	returnContext := fullIntegrationDetailContext(scenario.name+" return detail", returnResource)
	t.Logf("%s return selected resource: id=%s name=%q", scenario.name, returnResource.ID, returnResource.Name)
	expectedReturn := fullIntegrationExpectedRelatedCounts(t, clients, scenario.returnTargetType, returnResource)
	coldUnknownReturn := fullIntegrationAssertRelatedResults(t, scenario.returnTargetType, expectedReturn, returnResults, returnContext)
	fullIntegrationAssertRelatedCountsInView(t, *m, scenario.returnTargetType, expectedReturn, coldUnknownReturn, returnContext)
}

func fullIntegrationNewReadyModelWithClients(t *testing.T, profile, region string, clients *awsclient.ServiceClients) tui.Model {
	t.Helper()
	m := tui.New(profile, region, tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	// Stamp the live ConnectGen, as every production dispatch site does:
	// HandleClientsReady compares it for strict equality, so an unstamped
	// message is silently dropped and the model never leaves the pre-connect
	// state — every downstream assertion then reads an empty app.
	m, _ = fullIntegrationApplyMsg(m, messages.ClientsReady{Clients: clients, Region: region, Gen: m.Core().ConnectGen()})
	return m
}

func fullIntegrationAssertMainMenuCounts(t *testing.T, m tui.Model, expected map[string]fullIntegrationCountExpectation) {
	t.Helper()
	plain := fullIntegrationStripANSI(fullIntegrationViewContent(m))
	var missing []string
	for _, rt := range resource.AllResourceTypes() {
		exp, ok := expected[rt.ShortName]
		if !ok {
			missing = append(missing, rt.ShortName+" missing expectation")
			continue
		}
		suffix := fmt.Sprintf("%s (%d)", rt.Name, exp.count)
		if exp.truncated {
			suffix = fmt.Sprintf("%s (%d+)", rt.Name, exp.count)
		}
		displayed, ok := fullIntegrationFindMainMenuDisplayCount(plain, rt)
		if ok {
			t.Logf("main menu %s: displayed=%s expected=%s", rt.ShortName, displayed, fullIntegrationExpectedDisplay(exp))
		} else {
			t.Logf("main menu %s: displayed=<missing> expected=%s", rt.ShortName, fullIntegrationExpectedDisplay(exp))
		}
		if !strings.Contains(plain, suffix) {
			missing = append(missing, suffix)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("main menu missing %d expected count(s): %s\nview:\n%s", len(missing), strings.Join(missing, ", "), plain)
	}
}

func fullIntegrationExpectedRelatedCounts(t *testing.T, clients *awsclient.ServiceClients, sourceType string, source resource.Resource) map[string]int {
	t.Helper()
	defs := resource.GetRelated(sourceType)
	if len(defs) == 0 {
		t.Fatalf("%s has no related defs", sourceType)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cache := make(resource.ResourceCache)
	for _, def := range defs {
		if _, ok := cache[def.TargetType]; ok {
			continue
		}
		pf := resource.GetPaginatedFetcher(def.TargetType)
		if pf == nil {
			continue
		}
		result, err := pf(ctx, clients, "")
		if err != nil && len(result.Resources) == 0 {
			// E5 partial success (see above): degraded rows may accompany a
			// composite error; only a row-less error fails the harness.
			t.Fatalf("related target fetcher for %s -> %s failed: %v", sourceType, def.TargetType, err)
		}
		truncated := result.Pagination != nil && result.Pagination.IsTruncated
		cache[def.TargetType] = resource.ResourceCacheEntry{
			Resources:   result.Resources,
			IsTruncated: truncated,
			Pagination:  result.Pagination,
		}
	}

	expected := make(map[string]int, len(defs))
	for _, def := range defs {
		if def.Checker == nil {
			t.Fatalf("%s related def %q has nil checker", sourceType, def.DisplayName)
		}
		result := def.Checker(ctx, clients, source, cache)
		if result.Err != nil {
			t.Fatalf("%s related def %q failed: %v", sourceType, def.DisplayName, result.Err)
		}
		expected[def.DisplayName] = result.Count
	}
	return expected
}

func fullIntegrationOpenResourceList(t *testing.T, m *tui.Model, resourceType string) messages.ResourcesLoaded {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = fullIntegrationApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: resourceType,
	})
	raw := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)
	*m, _ = fullIntegrationApplyMsg(*m, loaded)
	return loaded
}

func fullIntegrationDescribeSelectedResource(t *testing.T, m *tui.Model, resourceType string) (resource.Resource, []messages.RelatedCheckResult) {
	t.Helper()
	res, results, ok := fullIntegrationDescribeSelectedResourceMaybeRelated(t, m, resourceType)
	if !ok {
		t.Fatalf("describe selected %s returned no related check command", resourceType)
	}
	return res, results
}

func fullIntegrationDescribeSelectedResourceMaybeRelated(t *testing.T, m *tui.Model, resourceType string) (resource.Resource, []messages.RelatedCheckResult, bool) {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = fullIntegrationApplyMsg(*m, fullIntegrationKeyPress("d"))
	raw := fullIntegrationRequireCmdMsg(t, cmd, "describe selected "+resourceType)
	nav, ok := raw.(messages.Navigate)
	if !ok {
		t.Fatalf("describe selected %s returned %T, expected messages.Navigate", resourceType, raw)
	}
	if nav.Resource == nil {
		t.Fatalf("describe selected %s returned NavigateMsg with nil resource", resourceType)
	}
	res := *nav.Resource

	var relatedCmd tea.Cmd
	*m, relatedCmd = fullIntegrationApplyMsg(*m, nav)
	if relatedCmd == nil {
		return res, nil, false
	}
	return res, fullIntegrationRunRelatedChecksFromStartCmd(t, m, relatedCmd, resourceType), true
}

func fullIntegrationEnterRelatedSingleDetail(t *testing.T, m *tui.Model, targetType, displayName string) (resource.Resource, []messages.RelatedCheckResult) {
	t.Helper()
	rel := fullIntegrationEnterFocusedRelated(t, m, targetType, displayName)
	var cmd tea.Cmd
	*m, cmd = fullIntegrationApplyMsg(*m, rel)
	if cmd == nil {
		t.Fatalf("related %q navigation returned nil cmd; expected fetch+auto-open detail", displayName)
	}

	// Fast path: related-panel NavigationKindDetail (cache hit) pushes the detail view
	// directly and dispatches the related-check fan-out as messages.RelatedCheckResult
	// leaves in the same cmd, with no intermediate trigger message and without
	// going through ResourcesLoadedMsg / NavigateMsg. Detect by collecting
	// those results directly from the returned cmd.
	if results := fullIntegrationCollectRelatedCheckResults(cmd); len(results) > 0 {
		res, ok := m.ActiveDetailResource()
		if !ok {
			t.Fatalf("related %q navigation dispatched a fan-out but did not land on a detail view", displayName)
		}
		return res, fullIntegrationApplyStartedAndCollectResults(t, m, results, targetType)
	}

	raw := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)
	var autoOpenCmd tea.Cmd
	*m, autoOpenCmd = fullIntegrationApplyMsg(*m, loaded)
	// The auto-open cmd may batch a Wave-2 ProbeEnrich task alongside the
	// navigation for issue-capable types — pick the Navigate out of the batch.
	navRaw := fullIntegrationExtractMsg(t, autoOpenCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.Navigate)
		return ok
	})
	nav, ok := navRaw.(messages.Navigate)
	if !ok {
		t.Fatalf("auto-open related %q returned %T, expected messages.Navigate", displayName, navRaw)
	}
	if nav.Resource == nil {
		t.Fatalf("auto-open related %q returned nil resource", displayName)
	}
	res := *nav.Resource

	var relatedCmd tea.Cmd
	*m, relatedCmd = fullIntegrationApplyMsg(*m, nav)
	return res, fullIntegrationRunRelatedChecksFromStartCmd(t, m, relatedCmd, targetType)
}

func fullIntegrationEnterRelatedList(t *testing.T, m *tui.Model, targetType, displayName string) {
	t.Helper()
	rel := fullIntegrationEnterFocusedRelated(t, m, targetType, displayName)
	var cmd tea.Cmd
	*m, cmd = fullIntegrationApplyMsg(*m, rel)
	if cmd == nil {
		return
	}
	raw := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	*m, _ = fullIntegrationApplyMsg(*m, raw)
}

func fullIntegrationEnterFocusedRelated(t *testing.T, m *tui.Model, targetType, displayName string) messages.RelatedNavigate {
	t.Helper()
	*m, _ = fullIntegrationApplyMsg(*m, fullIntegrationKeyPress("l"))
	var cmd tea.Cmd
	*m, cmd = fullIntegrationApplyMsg(*m, tea.KeyPressMsg{Code: tea.KeyEnter})
	raw := fullIntegrationRequireCmdMsg(t, cmd, "enter focused related "+displayName)
	rel, ok := raw.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("enter focused related %q returned %T, expected messages.RelatedNavigate", displayName, raw)
	}
	if rel.TargetType != targetType {
		t.Fatalf("focused related row target = %q, expected %q (%s)", rel.TargetType, targetType, displayName)
	}
	return rel
}

func fullIntegrationRunRelatedChecksFromStartCmd(t *testing.T, m *tui.Model, startCmd tea.Cmd, resourceType string) []messages.RelatedCheckResult {
	t.Helper()
	// The related-check fan-out is now dispatched directly by the
	// navigation flow itself — startCmd (may be a tea.BatchMsg carrying
	// other detail-load messages, e.g. EnrichDetailMsg, alongside them)
	// already carries the terminal messages.RelatedCheckResult leaves, with
	// no intermediate RelatedCheckStarted trigger message to find first.
	results := fullIntegrationCollectRelatedCheckResults(startCmd)
	return fullIntegrationApplyStartedAndCollectResults(t, m, results, resourceType)
}

func fullIntegrationApplyStartedAndCollectResults(t *testing.T, m *tui.Model, results []messages.RelatedCheckResult, resourceType string) []messages.RelatedCheckResult {
	t.Helper()
	if len(results) == 0 {
		t.Fatalf("related check for %s produced no RelatedCheckResultMsg", resourceType)
	}
	for _, result := range results {
		*m, _ = fullIntegrationApplyMsg(*m, result)
	}
	return results
}

func fullIntegrationCollectRelatedCheckResults(cmd tea.Cmd) []messages.RelatedCheckResult {
	var results []messages.RelatedCheckResult
	for _, msg := range fullIntegrationCollectCmdMessages(cmd) {
		if result, ok := msg.(messages.RelatedCheckResult); ok {
			results = append(results, result)
		}
	}
	return results
}

func fullIntegrationCollectCmdMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	return fullIntegrationCollectMessages(cmd())
}

func fullIntegrationCollectMessages(msg tea.Msg) []tea.Msg {
	switch v := msg.(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, cmd := range v {
			out = append(out, fullIntegrationCollectCmdMessages(cmd)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

// fullIntegrationAssertRelatedResults compares the app's related results
// against the oracle's expectations and returns the display names accepted as
// cold-cache unknowns: the oracle prefetches EVERY target cache before calling
// a checker, while the app prefetches only for defs registered with
// NeedsTargetCache — a no-prefetch checker answering -1 ("?") where the
// oracle computed a real count is the documented cold-cache contract, not a
// mismatch. The returned set lets the view assertion skip those rows.
func fullIntegrationAssertRelatedResults(t *testing.T, sourceType string, expected map[string]int, got []messages.RelatedCheckResult, context string) map[string]bool {
	t.Helper()
	prefetched := make(map[string]bool)
	for _, def := range resource.GetRelated(sourceType) {
		prefetched[def.DisplayName] = def.NeedsTargetCache
	}
	gotByName := make(map[string]resource.RelatedCheckResult, len(got))
	for _, result := range got {
		gotByName[result.DefDisplayName] = result.Result
	}
	coldUnknown := make(map[string]bool)
	for name, want := range expected {
		rr, ok := gotByName[name]
		if !ok {
			t.Fatalf("%s: missing related result %q; got %v", context, name, gotByName)
		}
		t.Logf("%s related result %s: actual=%d expected=%d", context, name, rr.Count, want)
		// Post-#58 a deferred/unknown/loading view result carries no
		// authoritative count — it is State, not Count==-1, that marks it. When
		// the def registers no target prefetch, the view could not resolve it
		// from its own cache while the oracle did (from its separately prefetched
		// cache): the documented cold-cache contract. Skip the strict view
		// assertion for it, exactly as the old Count==-1 path did.
		if rr.State != domain.RelatedResolved {
			if !prefetched[name] {
				coldUnknown[name] = true
				t.Logf("%s: related %q answered %v (cold cache, no prefetch registered); oracle computed %d from its own prefetched cache", context, name, rr.State, want)
				continue
			}
			t.Fatalf("%s: related result %q answered non-resolved state %v despite a registered prefetch, expected resolved %d", context, name, rr.State, want)
		}
		if rr.Count != want {
			t.Fatalf("%s: related result %q count = %d, expected %d; all results %v", context, name, rr.Count, want, gotByName)
		}
	}
	return coldUnknown
}

func fullIntegrationAssertRelatedCountsInView(t *testing.T, m tui.Model, sourceType string, expected map[string]int, coldUnknown map[string]bool, context string) {
	t.Helper()
	plain := fullIntegrationStripANSI(fullIntegrationViewContent(m))
	if !strings.Contains(plain, "RELATED") {
		t.Fatalf("%s: view does not contain RELATED panel:\n%s", context, plain)
	}
	for name, count := range expected {
		if coldUnknown[name] {
			t.Logf("%s related view %s: displayed=<?> (cold-cache unknown) expected=%d", context, name, count)
			continue
		}
		if fullIntegrationIsHiddenSelfPivotZero(sourceType, name, count) {
			t.Logf("%s related view %s: displayed=<hidden self-pivot zero> expected=%d", context, name, count)
			continue
		}
		want := name
		if count >= 0 {
			want = fmt.Sprintf("%s (%d)", name, count)
		}
		displayed, ok := fullIntegrationFindRelatedDisplayCount(plain, name)
		if ok {
			t.Logf("%s related view %s: displayed=%s expected=%d", context, name, displayed, count)
		} else {
			t.Logf("%s related view %s: displayed=<missing> expected=%d", context, name, count)
		}
		if !strings.Contains(plain, want) {
			t.Fatalf("%s: view missing related count %q:\n%s", context, want, plain)
		}
	}
}

func fullIntegrationIsHiddenSelfPivotZero(sourceType, displayName string, count int) bool {
	if sourceType == "" || count != 0 {
		return false
	}
	for _, def := range resource.GetRelated(sourceType) {
		if def.DisplayName == displayName && def.TargetType == sourceType {
			return true
		}
	}
	return false
}

func fullIntegrationAssertFrameContains(t *testing.T, m tui.Model, want string) {
	t.Helper()
	plain := fullIntegrationStripANSI(fullIntegrationViewContent(m))
	if !strings.Contains(plain, want) {
		t.Fatalf("view missing %q:\n%s", want, plain)
	}
}

func fullIntegrationFrameCount(name string, exp fullIntegrationCountExpectation) string {
	if rt := resource.FindResourceType(name); rt != nil && rt.ListTitle != "" {
		name = rt.ListTitle
	}
	if exp.truncated {
		return fmt.Sprintf("%s(%d+)", name, exp.count)
	}
	return fmt.Sprintf("%s(%d)", name, exp.count)
}

func fullIntegrationDetailContext(prefix string, res resource.Resource) string {
	if res.Name != "" {
		return fmt.Sprintf("%s [%s %q]", prefix, res.ID, res.Name)
	}
	return fmt.Sprintf("%s [%s]", prefix, res.ID)
}

func fullIntegrationExpectedDisplay(exp fullIntegrationCountExpectation) string {
	if exp.truncated {
		return fmt.Sprintf("%d+", exp.count)
	}
	return fmt.Sprintf("%d", exp.count)
}

func fullIntegrationFindMainMenuDisplayCount(plain string, rt resource.ResourceTypeDef) (string, bool) {
	re := regexp.MustCompile(regexp.QuoteMeta(rt.Name) + ` \((\d+\+?)\)`)
	m := re.FindStringSubmatch(plain)
	if len(m) != 2 {
		return "", false
	}
	return m[1], true
}

func fullIntegrationFindFrameDisplayCount(plain, resourceType string) (string, bool) {
	title := resourceType
	if rt := resource.FindResourceType(resourceType); rt != nil && rt.ListTitle != "" {
		title = rt.ListTitle
	}
	re := regexp.MustCompile(regexp.QuoteMeta(title) + `\((\d+\+?)\)`)
	m := re.FindStringSubmatch(plain)
	if len(m) != 2 {
		return "", false
	}
	return m[1], true
}

func fullIntegrationFindRelatedDisplayCount(plain, displayName string) (string, bool) {
	re := regexp.MustCompile(regexp.QuoteMeta(displayName) + `(?: \((\d+)\))?`)
	m := re.FindStringSubmatch(plain)
	if len(m) == 0 {
		return "", false
	}
	if len(m) >= 2 && m[1] != "" {
		return m[1], true
	}
	return "<unknown>", true
}

func fullIntegrationExtractMsg(t *testing.T, cmd tea.Cmd, pred func(tea.Msg) bool) tea.Msg {
	t.Helper()
	for _, msg := range fullIntegrationCollectCmdMessages(cmd) {
		if pred(msg) {
			return msg
		}
	}
	t.Fatalf("extractMsg: no message matched predicate")
	return nil
}

func fullIntegrationRequireCmdMsg(t *testing.T, cmd tea.Cmd, label string) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatalf("%s returned nil cmd", label)
	}
	msg := cmd()
	if msg == nil {
		t.Fatalf("%s command returned nil msg", label)
	}
	return msg
}

func fullIntegrationApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

func fullIntegrationViewContent(m tui.Model) string {
	return m.View().Content
}

func fullIntegrationKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

var fullIntegrationANSIRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func fullIntegrationStripANSI(s string) string {
	return fullIntegrationANSIRe.ReplaceAllString(s, "")
}
