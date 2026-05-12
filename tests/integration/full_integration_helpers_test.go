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

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	if err != nil {
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
	fullIntegrationAssertRelatedResults(t, expectedRelated, relatedResults, detailContext)
	fullIntegrationAssertRelatedCountsInView(t, m, rt.ShortName, expectedRelated, detailContext)
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
	fullIntegrationAssertRelatedResults(t, expectedFirst, firstResults, sourceContext)
	fullIntegrationAssertRelatedCountsInView(t, *m, scenario.sourceType, expectedFirst, sourceContext)

	relatedResource, relatedResults := fullIntegrationEnterRelatedSingleDetail(t, m, scenario.firstTargetType, scenario.firstDisplayName)
	firstRelatedContext := fullIntegrationDetailContext(scenario.name+" first related detail", relatedResource)
	t.Logf("%s first related selected resource: id=%s name=%q", scenario.name, relatedResource.ID, relatedResource.Name)
	expectedRelated := fullIntegrationExpectedRelatedCounts(t, clients, scenario.firstTargetType, relatedResource)
	fullIntegrationAssertRelatedResults(t, expectedRelated, relatedResults, firstRelatedContext)
	fullIntegrationAssertRelatedCountsInView(t, *m, scenario.firstTargetType, expectedRelated, firstRelatedContext)

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
	fullIntegrationAssertRelatedResults(t, expectedReturn, returnResults, returnContext)
	fullIntegrationAssertRelatedCountsInView(t, *m, scenario.returnTargetType, expectedReturn, returnContext)
}

func fullIntegrationNewReadyModelWithClients(t *testing.T, profile, region string, clients *awsclient.ServiceClients) tui.Model {
	t.Helper()
	m := tui.New(profile, region, tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	m, _ = fullIntegrationApplyMsg(m, messages.ClientsReady{Clients: clients, Region: region})
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
		if err != nil {
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
	// directly and emits RelatedCheckStartedMsg without going through
	// ResourcesLoadedMsg / NavigateMsg. Detect by scanning for that message
	// in the returned cmd.
	for _, inner := range fullIntegrationCollectCmdMessages(cmd) {
		started, ok := inner.(messages.RelatedCheckStarted)
		if !ok {
			continue
		}
		res := started.SourceResource
		results := fullIntegrationApplyStartedAndCollectResults(t, m, started, targetType)
		return res, results
	}

	raw := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)
	var autoOpenCmd tea.Cmd
	*m, autoOpenCmd = fullIntegrationApplyMsg(*m, loaded)
	navRaw := fullIntegrationRequireCmdMsg(t, autoOpenCmd, "auto-open related "+displayName)
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
	// startCmd may be a tea.BatchMsg carrying other detail-load messages
	// (e.g. EnrichDetailMsg) alongside RelatedCheckStartedMsg — find the
	// related-check message regardless of where it sits in the batch.
	raw := fullIntegrationExtractMsg(t, startCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.RelatedCheckStarted)
		return ok
	})
	started, ok := raw.(messages.RelatedCheckStarted)
	if !ok {
		t.Fatalf("related check start for %s returned %T, expected messages.RelatedCheckStarted", resourceType, raw)
	}
	return fullIntegrationApplyStartedAndCollectResults(t, m, started, resourceType)
}

func fullIntegrationApplyStartedAndCollectResults(t *testing.T, m *tui.Model, started messages.RelatedCheckStarted, resourceType string) []messages.RelatedCheckResult {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = fullIntegrationApplyMsg(*m, started)
	results := fullIntegrationCollectRelatedCheckResults(cmd)
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

func fullIntegrationAssertRelatedResults(t *testing.T, expected map[string]int, got []messages.RelatedCheckResult, context string) {
	t.Helper()
	gotByName := make(map[string]int, len(got))
	for _, result := range got {
		gotByName[result.DefDisplayName] = result.Result.Count
	}
	for name, want := range expected {
		if gotCount, ok := gotByName[name]; !ok {
			t.Fatalf("%s: missing related result %q; got %v", context, name, gotByName)
		} else {
			t.Logf("%s related result %s: actual=%d expected=%d", context, name, gotCount, want)
			if gotCount != want {
				t.Fatalf("%s: related result %q count = %d, expected %d; all results %v", context, name, gotCount, want, gotByName)
			}
		}
	}
}

func fullIntegrationAssertRelatedCountsInView(t *testing.T, m tui.Model, sourceType string, expected map[string]int, context string) {
	t.Helper()
	plain := fullIntegrationStripANSI(fullIntegrationViewContent(m))
	if !strings.Contains(plain, "RELATED") {
		t.Fatalf("%s: view does not contain RELATED panel:\n%s", context, plain)
	}
	for name, count := range expected {
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
