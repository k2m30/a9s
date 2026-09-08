// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_one_owner_test.go pins the "one fact, one owner" rows: a failure
// reaches the operator phrased once, through the one formatter; a result a
// lane has already discarded schedules no work; and the persisted "we have
// probed this type for issues" flag comes from a probe, not from the mere
// presence of rows.
//
// The three source-shape gates here exist because the duplication they pin is
// not observable from outside: two call sites computing one string identically
// look the same from every surface until one of them is edited. Each names the
// file and the expression, so the green transition is deleting the second
// copy — no test edit.
package unit

import (
	"go/format"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// wipfixDeniedErr is an AWS API error in the shape the SDK actually delivers:
// an operation wrapper whose own words repeat the service and operation the
// caller already knows, around the message the operator can act on.
func wipfixDeniedErr(op string) error {
	return &smithy.OperationError{
		ServiceID:     "Cost Explorer",
		OperationName: op,
		Err: &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User is not authorized to perform ce:GetCostAndUsage",
		},
	}
}

// --- row 9a: a screen shows the cause, not the chain -----------------------

// TestCostsScreen_FetchFailureShowsTheCauseNotTheChain pins that the costs
// screen's error line is the phrasing core/aws owns (aws.CauseOf), the same
// one every flash and menu row uses. Assigning err.Error() puts the SDK's
// service-and-operation preamble and the request id on a one-line surface,
// pushing the sentence the operator needs off the end.
func TestCostsScreen_FetchFailureShowsTheCauseNotTheChain(t *testing.T) {
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))

	// Answer the screen's own outstanding request, so the delivery matches
	// what the frame awaits and reaches the error branch.
	tasks := c.EnsureCostsFetch()
	if len(tasks) != 1 {
		t.Fatalf("the costs screen issued %d fetch task(s) on open, want 1", len(tasks))
	}
	payload, ok := tasks[0].Payload.(runtime.FetchCostsPayload)
	if !ok {
		t.Fatalf("costs fetch payload type = %T, want runtime.FetchCostsPayload", tasks[0].Payload)
	}

	err := wipfixDeniedErr("GetCostAndUsage")
	_ = c.ApplyCostsLoaded(messages.CostsLoaded{Query: payload.Query, Window: payload.Window, Err: err})

	costsBody := c.Snapshot().Body.Costs
	if costsBody == nil {
		t.Fatal("no costs body on the snapshot after a failed costs fetch")
	}
	want := awsclient.CauseOf(err)
	if costsBody.ErrorMsg != want {
		t.Errorf("costs screen error line = %q,\nwant %q — the one phrasing core/aws owns",
			costsBody.ErrorMsg, want)
	}
	if strings.Contains(costsBody.ErrorMsg, "operation error") {
		t.Errorf("costs screen error line = %q — it carries the SDK's wrapper preamble, "+
			"which is the caller's own words repeated back", costsBody.ErrorMsg)
	}
}

// --- row 9c: two failures, one sentence ------------------------------------

// TestHandleRelatedCheckResult_TwoFailuresFlashOnce pins that a related
// result which failed twice (the checker itself AND the by-ID lazy add)
// raises one flash. Two flashes race for one status line, so the operator
// sees whichever landed last and never learns there were two.
func TestHandleRelatedCheckResult_TwoFailuresFlashOnce(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:     "ec2",
		SourceResourceID: "i-0123456789abcdef0",
		DefDisplayName:   "Security Groups",
		Result:           resource.ErrorRelated("sg", wipfixDeniedErr("DescribeSecurityGroups")),
		LazyAddError:     wipfixDeniedErr("DescribeSecurityGroups"),
	})

	flashes := 0
	for _, in := range intents {
		if _, ok := in.(runtime.FlashIntent); ok {
			flashes++
		}
	}
	if flashes != 1 {
		t.Errorf("a related result carrying two failures produced %d flash intents, want 1 — "+
			"they share one status line, so the second silently replaces the first", flashes)
	}
}

// TestHandleRelatedCheckResult_SingleFailureStillFlashes is the negative
// half: collapsing to one sentence must not collapse to none.
func TestHandleRelatedCheckResult_SingleFailureStillFlashes(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:     "ec2",
		SourceResourceID: "i-0123456789abcdef0",
		DefDisplayName:   "Security Groups",
		Result:           resource.ErrorRelated("sg", wipfixDeniedErr("DescribeSecurityGroups")),
	})
	flashes := 0
	for _, in := range intents {
		if _, ok := in.(runtime.FlashIntent); ok {
			flashes++
		}
	}
	if flashes != 1 {
		t.Errorf("a related result with one failure produced %d flash intents, want 1", flashes)
	}
}

// TestHandleRelatedCheckResult_SuccessFlashesNothing is the healthy
// counterpart.
func TestHandleRelatedCheckResult_SuccessFlashesNothing(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:     "ec2",
		SourceResourceID: "i-0123456789abcdef0",
		DefDisplayName:   "Security Groups",
		Result:           resource.KnownRelated("sg", []string{"sg-0123456789abcdef0"}, false),
	})
	for _, in := range intents {
		if f, ok := in.(runtime.FlashIntent); ok {
			t.Errorf("a successful related check flashed %q", f.Text)
		}
	}
}

// --- row 11: a discarded result schedules no work --------------------------

// TestHandleResourcesLoaded_SupersededResultDispatchesNoEnrichTask pins row
// 11: a list result whose sequence a later fetch has already replaced is
// dropped, and the Wave-2 list-open enrichment it would have triggered must
// go with it. Enriching rows nobody will render spends the operator's API
// budget on a screen that no longer exists.
func TestHandleResourcesLoaded_SupersededResultDispatchesNoEnrichTask(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	first := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	core.StampListFetchSeq(&first)
	second := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	core.StampListFetchSeq(&second)
	if !core.ListResultSuperseded("s3", first.ListSeq) {
		t.Fatalf("precondition: sequence %d is not reported superseded after %d was handed out",
			first.ListSeq, second.ListSeq)
	}

	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: "s3",
		Resources:    wipfixS3Rows(2),
		ListSeq:      first.ListSeq,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	for _, task := range tasks {
		t.Errorf("a superseded s3 result still dispatched task %v — "+
			"the result was discarded, so the work it schedules must be discarded with it", task.Key)
	}
}

// TestHandleResourcesLoaded_CurrentResultStillDispatchesEnrichTask is the
// negative half: the live result must keep its list-open enrichment.
func TestHandleResourcesLoaded_CurrentResultStillDispatchesEnrichTask(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	core.StampListFetchSeq(&req)

	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: "s3",
		Resources:    wipfixS3Rows(2),
		ListSeq:      req.ListSeq,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	if len(tasks) == 0 {
		t.Error("the current s3 result dispatched no task — the list-open Wave-2 enrichment is gone")
	}
}

// --- row 13: "probed" is a probe's answer, not a row count -----------------

// TestSaveAvailabilityFromRows_UnprobedTypeIsNotRecordedAsProbed pins row 13:
// a type whose rows carry only Wave-1 findings has never had its Wave-2
// enricher answer for it. Persisting it as "probed, 0 issues" makes the next
// session start with a confident zero badge for a question nobody asked.
func TestSaveAvailabilityFromRows_UnprobedTypeIsNotRecordedAsProbed(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", cfg)
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	pair := session.Pair{Profile: "example-readonly", Region: "us-east-1"}
	rows := []resource.Resource{{
		ID:   "a9s-example-bucket-a",
		Name: "a9s-example-bucket-a",
		Findings: []domain.Finding{{
			Code: "s3.no-versioning", Phrase: "versioning off",
			Severity: domain.SevWarn, Source: "wave1",
		}},
	}}
	s.RowStore.Observe("s3", rows, &domain.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	if err := core.SaveAvailabilityFromRows(pair); err != nil {
		t.Fatalf("SaveAvailabilityFromRows: %v", err)
	}

	body := wipfixReadTypeFile(t, cfg, "s3")
	if strings.Contains(body, "issues_known: true") {
		t.Errorf("the s3 type file records issues_known: true after a save driven only by "+
			"Wave-1 rows — no Wave-2 probe has answered for s3.\nfile:\n%s", body)
	}
}

// wipfixReadTypeFile returns the per-type cache file's text, or fails.
func wipfixReadTypeFile(t *testing.T, cfgFolder, shortName string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(cfgFolder, "cache", "*", shortName+".yaml"))
	if err != nil {
		t.Fatalf("globbing the cache dir: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("found %d cache files for %q under %s, want 1", len(matches), shortName, cfgFolder)
	}
	b, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("reading %s: %v", matches[0], err)
	}
	return string(b)
}

// --- source-shape gates ----------------------------------------------------

// wipfixCountOccurrences returns how many of the named files contain expr, and
// the list of those that do.
func wipfixCountOccurrences(t *testing.T, expr string, roots ...string) []string {
	t.Helper()
	var hits []string
	for _, root := range roots {
		err := filepath.WalkDir(wipfixRepoPath(t, root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for i, line := range strings.Split(string(b), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(line, expr) {
					hits = append(hits, path+":"+strconv.Itoa(i+1))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	return hits
}

// wipfixRepoPath resolves a repo-relative path from tests/unit.
func wipfixRepoPath(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join("..", "..", rel)
}

// TestOneOwner_BulkIntentForwardHasOneSeam pins row 10: the terminal host
// applies "forward the whole slice minus the banner flashes, then re-emit
// each banner flash through handleFlash" in two places. Two copies of one
// rule drift; the seam is dispatchHandlerResult.
func TestOneOwner_BulkIntentForwardHasOneSeam(t *testing.T) {
	hits := wipfixCountOccurrences(t, "ApplyIntents(withoutBannerFlashes(", "internal/tui")
	if len(hits) > 1 {
		t.Errorf("the banner-flash-withholding bulk forward appears at %d sites, want 1:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// TestOneOwner_RelatedRowErrorTextHasOneProducer pins row 9's middle part: the
// related panel's per-row error text is computed from one fact
// (RelatedCheckResult.Err) at two sites — the result lane and the cache
// replay. The panel shows only whether an error exists, so the two copies
// cannot be told apart from any surface until one is changed.
func TestOneOwner_RelatedRowErrorTextHasOneProducer(t *testing.T) {
	hits := wipfixCountOccurrences(t, "if err := entry.Result.Err(); err != nil", "core/app")
	hits = append(hits, wipfixCountOccurrences(t, "if err := result.Result.Err(); err != nil", "core/app")...)
	if len(hits) > 1 {
		t.Errorf("the related row's error text is derived at %d sites, want 1:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// TestOneOwner_CanonicalShortNameHasOneHelper pins row 14: canonShortName is
// the canonicaliser, and the inline `FindResourceType(x); td != nil` idiom it
// replaces is still spelled out across four packages. Either every site goes
// through the helper or the helper goes — one shape, decided once.
func TestOneOwner_CanonicalShortNameHasOneHelper(t *testing.T) {
	helper := wipfixCountOccurrences(t, "func canonShortName(", "core/runtime")
	inline := wipfixCountOccurrences(t, "FindResourceType(",
		"core/app", "core/aws", "core/cache", "core/runtime")
	kept := inline[:0]
	for _, h := range inline {
		if strings.Contains(h, "handlers_resources.go") {
			continue // canonShortName's own body lives here
		}
		kept = append(kept, h)
	}
	inline = kept

	// Either resolution the row allows is green: every site routes through the
	// helper, or the helper is gone and the inline idiom is the shape.
	if len(helper) == 0 {
		return
	}
	if len(inline) > 0 {
		t.Errorf("canonShortName is the canonicaliser and the idiom it replaces is still spelled "+
			"inline at %d site(s) — route them through it, or delete it and keep the idiom, "+
			"decided once:\n  %s", len(inline), strings.Join(inline, "\n  "))
	}
}

// TestOneOwner_ProfileFetchFailurePhrasedOnce pins row 21: the TUI's profile
// fetch must not phrase a failed local config read a second way beside the
// controller's own flash. Both go through the one extraction.
func TestOneOwner_ProfileFetchFailurePhrasedOnce(t *testing.T) {
	hits := wipfixCountOccurrences(t, "messages.Flash{Text: err.Error()", "internal/tui")
	if len(hits) > 0 {
		t.Errorf("a raw error chain is flashed verbatim at %d site(s) — the words the operator "+
			"reads come from aws.MessageOf, wherever the failure was raised:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// TestNoStaleFetcherKeysInHandBuiltS3Rows pins row 26: the hand-built s3 rows
// in core/app/list_test.go describe a fetcher shape that no longer exists.
// A fixture that lies about the fetcher is the next reader's wrong mental
// model.
func TestNoStaleFetcherKeysInHandBuiltS3Rows(t *testing.T) {
	b, err := os.ReadFile(wipfixRepoPath(t, "core/app/list_test.go"))
	if err != nil {
		t.Fatalf("reading core/app/list_test.go: %v", err)
	}
	if strings.Contains(string(b), "bucket_name") {
		t.Error("core/app/list_test.go still builds s3 rows with a \"bucket_name\" field — " +
			"no s3 fetcher has written that key since the aws5 change")
	}
}

// TestCostsAccessDeniedCommentAsksTheClassifier pins row 24: the comment
// beside the costs refusal spells an AWS exception name by hand while the
// code asks the classifier for it, so the two can disagree silently.
func TestCostsAccessDeniedCommentAsksTheClassifier(t *testing.T) {
	b, err := os.ReadFile(wipfixRepoPath(t, "core/app/costs_state.go"))
	if err != nil {
		t.Fatalf("reading core/app/costs_state.go: %v", err)
	}
	for i, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") && strings.Contains(line, "AccessDeniedException") {
			t.Errorf("core/app/costs_state.go:%d names AccessDeniedException in a comment while the "+
				"code beside it asks the classifier: %s", i+1, strings.TrimSpace(line))
		}
	}
}

// TestListBodyMarkerColIsNamedForWhatItIs pins row 22: MarkerCol elects the
// identity column and marks nothing. Nothing reads its serialised name, so
// the rename costs nothing and the misleading name costs every reader.
func TestListBodyMarkerColIsNamedForWhatItIs(t *testing.T) {
	hits := wipfixCountOccurrences(t, "MarkerCol", "core/app", "core/runtime", "internal/tui")
	if len(hits) > 0 {
		t.Errorf("MarkerCol is still spelled at %d site(s) — it elects the identity column and "+
			"marks nothing:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

// TestRelatedColdFetchRoundtripTestIsGofmtClean pins row 32.
func TestRelatedColdFetchRoundtripTestIsGofmtClean(t *testing.T) {
	path := wipfixRepoPath(t, "core/app/related_cold_fetch_roundtrip_test.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		t.Fatalf("formatting %s: %v", path, err)
	}
	if string(src) != string(formatted) {
		t.Errorf("core/app/related_cold_fetch_roundtrip_test.go is not gofmt clean")
	}
}

// TestCrossViewCacheSeed_IsListOnly pins row 12: the shared per-type resource
// cache is the type's global population. A filtered or child result carries a
// narrower set under the same type name, and an unstamped one carries no
// ordering claim at all — neither may seed it, or every not-yet-visited view
// of that type reads a subset as the whole.
func TestCrossViewCacheSeed_IsListOnly(t *testing.T) {
	for _, lane := range []messages.FetchProvenance{
		messages.FetchProvenanceFilteredList,
		messages.FetchProvenanceChild,
	} {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		s := session.New()
		s.Profile = "example-readonly"
		s.Region = "us-east-1"
		core := runtime.New(s, nil)

		// ListSeq 0: a drill carries no ordering claim of its own.
		intents, _ := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
			ResourceType: "s3",
			Resources:    wipfixS3Rows(1),
			ListSeq:      0,
			Provenance:   lane,
		})
		for _, in := range intents {
			if p, ok := in.(runtime.PatchResourceCache); ok {
				t.Errorf("an unstamped %v result seeded the cross-view cache for %q with %d row(s) — "+
					"that is a subset of the type, not its population",
					lane, p.ResourceType, len(p.Entry.Resources))
			}
		}
	}
}

// TestTUILoadMoreStampsTheScreensLane pins row 28: the "m" key builds its own
// messages.LoadMore rather than going through the controller's action, so it
// must stamp the lane the controller's own owner reports. Left unstamped it
// falls back to deriving the lane from the drill's (empty) context maps and
// claims to be the type's canonical list, which the delivery gate then
// refuses on the drill's own screen.
func TestTUILoadMoreStampsTheScreensLane(t *testing.T) {
	src := wipfixReadRepoFile(t, "internal/tui/views/resourcelist.go")
	if !strings.Contains(src, "GetListLane()") {
		t.Error("internal/tui/views/resourcelist.go builds messages.LoadMore without reading " +
			"GetListLane() — the one owner of a list screen's lane")
	}
}

// TestTUIRefreshActiveListReadsTheLaneOwner pins row 29: Ctrl+R on a
// client-side related drill goes through refreshActiveList, whose fetch is
// stamped canonical unconditionally. The refresh is then refused on the
// drill's own screen and can land on the list beneath it.
func TestTUIRefreshActiveListReadsTheLaneOwner(t *testing.T) {
	src := wipfixReadRepoFile(t, "internal/tui/runtime_adapter_navigate.go")
	idx := strings.Index(src, "func (m Model) refreshActiveList")
	if idx < 0 {
		t.Fatal("refreshActiveList is not in internal/tui/runtime_adapter_navigate.go")
	}
	end := idx + 2500
	if end > len(src) {
		end = len(src)
	}
	if !strings.Contains(src[idx:end], "GetListLane()") {
		t.Error("refreshActiveList dispatches its refresh without reading GetListLane() — " +
			"a Ctrl+R on a related drill is stamped as the type's canonical list")
	}
}

// wipfixReadRepoFile returns a repo-relative file's text.
func wipfixReadRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(wipfixRepoPath(t, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// wipfixFuncBody returns the text of the named top-level function in a
// repo-relative file, from its signature to the next one.
func wipfixFuncBody(t *testing.T, rel, signature string) string {
	t.Helper()
	src := wipfixReadRepoFile(t, rel)
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("%s does not declare %s", rel, signature)
	}
	rest := src[start+len(signature):]
	if end := strings.Index(rest, "\nfunc "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// TestOneOwner_SupersessionIsCheckedOncePerMessage pins the first half of row
// 36. The runtime's handler is the owner of re-entry verification: it is the
// seam the terminal host and every direct caller go through, so it stamps the
// canonical short name onto the message and drops a superseded one. The
// controller then reads what it was handed. Asking the same two questions a
// second time at the controller's door is a second answer to each, and a
// second place to get either wrong.
func TestOneOwner_SupersessionIsCheckedOncePerMessage(t *testing.T) {
	owner := wipfixFuncBody(t, "core/runtime/handlers_resources.go",
		"func (c *Core) HandleResourcesLoaded(")
	if !strings.Contains(owner, "ListResultSuperseded(") {
		t.Error("Core.HandleResourcesLoaded does not check ListResultSuperseded — it is the " +
			"one seam that verifies re-entry, and a direct caller has nothing else to rely on")
	}
	if !strings.Contains(owner, "FindResourceType(") && !strings.Contains(owner, "CanonicalShortName(") {
		t.Error("Core.HandleResourcesLoaded does not canonicalise the resource type — an " +
			"alias-opened list must key the gen guard, the reseed and the task scope by the " +
			"same short name everything else uses")
	}

	if strings.Contains(wipfixReadRepoFile(t, "core/app/handle.go"), "ListResultSuperseded(") {
		t.Error("core/app/handle.go re-checks ListResultSuperseded for a message the runtime " +
			"has already dropped on that answer — the controller reads what it is handed")
	}
	screenLookup := wipfixFuncBody(t, "core/app/handle.go",
		"func (c *Controller) findResourceListScreen(")
	if strings.Contains(screenLookup, "FindResourceType(") ||
		strings.Contains(screenLookup, "CanonicalShortName(") {
		t.Error("findResourceListScreen canonicalises the resource type a second time — " +
			"the message arrives carrying the canonical short name the runtime stamped")
	}
}

// TestOneOwner_TheFilterChainHasOneCaller pins the second half of row 36: the
// list body's build filters and sorts the rows once and memoises the result.
// A title or an accessor that runs the same chain again pays for it on every
// call and can disagree with what is on screen — the drill-count defect this
// task already fixed was exactly that disagreement.
func TestOneOwner_TheFilterChainHasOneCaller(t *testing.T) {
	hits := wipfixCountOccurrences(t, "c.applyListFilters(", "core/app")
	var outside []string
	for _, h := range hits {
		if strings.Contains(h, "list_body.go") {
			outside = append(outside, h)
		}
	}
	if len(outside) > 1 {
		t.Errorf("the build's filter chain is re-run at %d sites in core/app/list_body.go, "+
			"want 1 (the build itself) — the title and the accessors read the memo:\n  %s",
			len(outside), strings.Join(outside, "\n  "))
	}
}
