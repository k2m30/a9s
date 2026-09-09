package unit_test

// aws6_drill_context_entry_points_test.go — the screen owns its context, and
// every way into a child view carries it.
//
// A child list screen knows which parent it was opened for; that is what its
// ParentContext is. A grandchild's declaration reads from it by name
// ("@parent.bucket"), so a screen that never recorded its own context hands the
// resolver nothing and the drill opens onto an empty list with no AWS call and
// no error — the operator sees an empty folder that is not empty.
//
// The fix is that the screen always has one, not that the resolver guesses.
// Falling back to the selected row's field would put the same fact in two
// places: the row's "bucket" is the bucket the row was LISTED from, and after a
// cross-bucket navigation those two need not agree. The pin below locks the
// resolver against that fallback deliberately.

import (
	"context"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestChildListScreenAlwaysCarriesItsContext pins the Enter path: opening a
// child view records the context that opened it, and so does the grandchild.
// The resolver is then never handed a nil.
func TestChildListScreenAlwaysCarriesItsContext(t *testing.T) {
	c, bucket, folders := openDemoS3FolderList(t)

	got := c.GetListParentContext()
	if got == nil {
		t.Fatalf("the s3_objects screen for bucket %q has no parent context; a grandchild drill from "+
			"here reads \"@parent.bucket\" out of nothing", bucket)
	}
	if got["bucket"] != bucket {
		t.Errorf("the object listing's parent context bucket = %q, want %q", got["bucket"], bucket)
	}

	// The grandchild screen records its own, so a THIRD level reads a bucket
	// too. This is the step that fails when a path seeds the context only for
	// a screen it created and skips one that already exists.
	c.Apply(app.Action{Kind: app.ActionChildView, Arg: "enter"})
	deeper := c.GetListParentContext()
	if deeper == nil {
		t.Fatalf("drilling into prefix %q left the grandchild screen with no parent context",
			folders[0].ID)
	}
	if deeper["bucket"] != bucket {
		t.Errorf("the grandchild screen's bucket = %q, want %q — every drill carries the screen's "+
			"context forward", deeper["bucket"], bucket)
	}
	if deeper["prefix"] != folders[0].ID {
		t.Errorf("the grandchild screen's prefix = %q, want %q", deeper["prefix"], folders[0].ID)
	}
}

// TestNoRelatedPivotTargetsAChildView is why the related-resource lane needs no
// drill pin of its own: no registered pivot targets a child type, so that lane
// never lands on a screen a "@parent." declaration reads from.
//
// It is a guard, not an observation. The day a pivot does target a child type,
// this goes red and the context question has to be answered for that lane
// before it ships — which is cheaper than discovering it as a blank screen.
func TestNoRelatedPivotTargetsAChildView(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		for _, d := range resource.GetRelated(td.ShortName) {
			if resource.FindResourceType(d.TargetType) != nil {
				continue
			}
			if resource.GetChildType(d.TargetType) == nil {
				continue
			}
			t.Errorf("the %s pivot into %s lands on a child view; that lane builds its context from the "+
				"pivot's own filter and not from the screen, so a \"@parent.\" declaration on %s would "+
				"resolve nothing (core/app/navigate.go, NavigationKindEnterChildView)",
				td.ShortName, d.TargetType, d.TargetType)
		}
	}
}

// TestEveryParentSourcedGrandchildDrillReadsItsParent is the sweep row 21 asks
// for, over every child view whose declaration reads from a grandparent rather
// than over the S3 one alone.
func TestEveryParentSourcedGrandchildDrillReadsItsParent(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, _ := buildVisibilityTypeCache(t)

	chains := parentSourcedChains(t)
	if len(chains) == 0 {
		t.Fatal("no child view declares a \"@parent.\" source, so nothing exercises this at all")
	}

	for _, ch := range chains {
		t.Run(ch.parentType+"/"+ch.childType, func(t *testing.T) {
			c, wantCtx, ok := openGrandchildDrill(t, clients, byType, ch)
			if !ok {
				t.Skipf("the demo bench has no %s row that opens a %s listing with a drillable row",
					ch.rootType, ch.parentType)
			}
			got := c.GetListParentContext()
			if got == nil {
				t.Fatalf("drilling %s -> %s left the screen with no parent context", ch.parentType, ch.childType)
			}
			for k, v := range wantCtx {
				if got[k] != v {
					t.Errorf("grandchild context[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// TestResolverDoesNotFallBackToTheRowsField locks the decision the fix must NOT
// take. The reviewer proposed reading the selected row's own field when the
// parent context is missing; that makes the row a second source for a fact the
// screen owns, and the two disagree the moment a row is reached from somewhere
// other than the bucket it names.
func TestResolverDoesNotFallBackToTheRowsField(t *testing.T) {
	child := s3ObjectsSelfChild(t)

	row := domain.Resource{
		ID:     "landing/",
		Fields: map[string]string{"bucket": "acme-bucket-the-row-remembers", "kind": "folder"},
	}
	got := resource.ResolveChildContext(child, &row, nil)

	if got["bucket"] != "" {
		t.Errorf("with no parent context the resolver produced bucket %q from the row's own field; "+
			"\"@parent.bucket\" names the SCREEN's context, and a row-field fallback is a second "+
			"source for one fact", got["bucket"])
	}
	// The sources that do not depend on a parent still resolve, so refusing the
	// fallback is not refusing to work.
	if got["prefix"] != row.ID {
		t.Errorf("the \"ID\" source resolved to %q, want %q", got["prefix"], row.ID)
	}
}

// TestNoChildListScreenHasANilContext is the sweep behind the three above: every
// child type the demo bench can open is opened, and none of the screens ends up
// without a context for a grandchild to read.
func TestNoChildListScreenHasANilContext(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)

	checked := 0
	for _, td := range resource.AllResourceTypes() {
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}
		for _, child := range td.Children {
			ctd := resource.GetChildType(child.ChildType)
			if ctd == nil || ctd.ChildFetcher == nil {
				continue
			}
			c := newVisibilityListController(t, td.ShortName)
			c.ApplyResourcesLoaded(td.ShortName, rows[:1], nil, false)
			if _, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: child.Key}); len(tasks) == 0 {
				continue
			}
			checked++
			if c.GetListParentContext() == nil {
				t.Errorf("%s -> %s: the child screen carries no parent context, so a grandchild "+
					"declaration reading \"@parent.\" from it resolves nothing",
					td.ShortName, child.ChildType)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no child view opened on the demo bench, so this sweep checked nothing")
	}
}

// parentSourcedChain names a grandchild drill whose declaration reads from a
// grandparent: the top-level type it starts at, the child listing it drills
// through, and the grandchild that needs the inherited value.
type parentSourcedChain struct {
	rootType   string
	rootChild  resource.ChildViewDef
	parentType string
	childType  string
	child      resource.ChildViewDef
}

// parentSourcedChains finds every registered child view declaring a "@parent."
// source, together with the top-level type whose drill reaches it.
func parentSourcedChains(t *testing.T) []parentSourcedChain {
	t.Helper()
	var out []parentSourcedChain
	for _, ptd := range resource.AllChildTypes() {
		for _, ch := range ptd.Children {
			usesParent := false
			for _, src := range ch.ContextKeys {
				if strings.HasPrefix(src, "@parent.") {
					usesParent = true
				}
			}
			if !usesParent {
				continue
			}
			for _, rtd := range resource.AllResourceTypes() {
				for _, rc := range rtd.Children {
					if rc.ChildType != ptd.ShortName {
						continue
					}
					out = append(out, parentSourcedChain{
						rootType: rtd.ShortName, rootChild: rc,
						parentType: ptd.ShortName, childType: ch.ChildType, child: ch,
					})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].parentType+out[i].childType < out[j].parentType+out[j].childType
	})
	return out
}

// openGrandchildDrill drives root list -> child listing -> grandchild drill for
// one chain, and returns the controller plus the context the resolver produces
// for that drill. Reports false when the demo bench offers no row to drill.
func openGrandchildDrill(
	t *testing.T,
	clients *awsclient.ServiceClients,
	byType map[string][]resource.Resource,
	ch parentSourcedChain,
) (*app.Controller, map[string]string, bool) {
	t.Helper()
	ptd := resource.GetChildType(ch.parentType)
	if ptd == nil || ptd.ChildFetcher == nil {
		return nil, nil, false
	}

	for _, root := range byType[ch.rootType] {
		dr := domain.Resource(root)
		rootCtx := resource.ResolveChildContext(ch.rootChild, &dr, nil)
		res, err := ptd.ChildFetcher(context.Background(), clients, rootCtx, "")
		if err != nil || len(res.Resources) == 0 {
			continue
		}
		var drillable []resource.Resource
		for _, r := range res.Resources {
			if ch.child.DrillCondition == nil || ch.child.DrillCondition(domain.Resource(r)) {
				drillable = append(drillable, r)
			}
		}
		if len(drillable) == 0 {
			continue
		}

		c := newVisibilityListController(t, ch.rootType)
		c.ApplyResourcesLoaded(ch.rootType, []resource.Resource{root}, nil, false)
		if _, tasks := c.Apply(app.Action{Kind: app.ActionChildView, Arg: ch.rootChild.Key}); len(tasks) == 0 {
			continue
		}
		c.ApplyResourcesLoaded(ch.parentType, drillable, nil, false)
		c.Apply(app.Action{Kind: app.ActionChildView, Arg: ch.child.Key})

		selected := domain.Resource(drillable[0])
		return c, resource.ResolveChildContext(ch.child, &selected, rootCtx), true
	}
	return nil, nil, false
}
