// row_identity_demo_bench_test.go — row identity over the demo bench.
//
// The demo fixtures are the only place every type, every child view and every
// related pivot run together through the real fetchers and checkers, so the
// class guards for "IDs are unique" and "a pivot's IDs open a row" live here.
package unit_test

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// ridReportDups fails once per ID that more than one row in rows carries.
func ridReportDups(t *testing.T, scope string, rows []resource.Resource) {
	t.Helper()
	count := map[string]int{}
	for _, r := range rows {
		count[r.ID]++
	}
	var dups []string
	for id, n := range count {
		if n > 1 {
			dups = append(dups, fmt.Sprintf("%q x%d", id, n))
		}
	}
	sort.Strings(dups)
	for _, d := range dups {
		t.Errorf("%s: duplicate row ID %s", scope, d)
	}
}

// ridCheckChildren drains every child view of every parent row the way the
// drill builds its context, and checks each child screen's rows. A child
// type's rows are unique per parent screen: the same log line in two streams
// is two screens, not a collision.
func ridCheckChildren(t *testing.T, clients *awsclient.ServiceClients, td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int) {
	t.Helper()
	if depth > 5 {
		return
	}
	for _, child := range td.Children {
		ctd := resource.GetChildType(child.ChildType)
		if ctd == nil || ctd.ChildFetcher == nil {
			continue
		}
		for i := range rows {
			dr := domain.Resource(rows[i])
			pctx := resource.ResolveChildContext(child, &dr, parentCtx)
			kids, err := unit.CollectAllPages(func(token string) (resource.FetchResult, error) {
				return ctd.ChildFetcher(context.Background(), clients, pctx, token)
			})
			if err != nil || len(kids) == 0 {
				continue
			}
			ridReportDups(t, fmt.Sprintf("%s under %s %q", child.ChildType, td.ShortName, rows[i].ID), kids)
			ridCheckChildren(t, clients, *ctd, kids, pctx, depth+1)
		}
	}
}

// TestDemoRowIdentity_NoTypeHasDuplicateIDs: every top-level type's demo rows,
// and every child screen reachable from them, carry unique IDs.
func TestDemoRowIdentity_NoTypeHasDuplicateIDs(t *testing.T) {
	clients := demo.NewServiceClients()
	for _, td := range resource.AllResourceTypes() {
		rows, ok := drainVisibilityFixtures(t, td, clients)
		if !ok {
			continue
		}
		ridReportDups(t, td.ShortName, rows)
		ridCheckChildren(t, clients, td, rows, nil, 0)
	}
}

// TestDemoRowIdentity_ECSSvcSameNameInTwoClustersBothList: the demo carries two
// ECS services with one name in two clusters, and the list shows both.
func TestDemoRowIdentity_ECSSvcSameNameInTwoClustersBothList(t *testing.T) {
	td := resource.FindResourceType("ecs-svc")
	if td == nil {
		t.Fatal("ecs-svc is not a registered type")
	}
	rows, _ := drainVisibilityFixtures(t, *td, demo.NewServiceClients())

	byName := map[string]map[string]string{}
	for _, r := range rows {
		if byName[r.Name] == nil {
			byName[r.Name] = map[string]string{}
		}
		byName[r.Name][r.Fields["cluster"]] = r.ID
	}
	var twins []string
	for name, clusters := range byName {
		if len(clusters) < 2 {
			continue
		}
		for _, id := range clusters {
			twins = append(twins, id)
		}
		t.Logf("demo service %q runs in %d clusters", name, len(clusters))
		break
	}
	if len(twins) == 0 {
		t.Fatal("no demo ECS service name is shared by two clusters")
	}

	c := newVisibilityListController(t, "ecs-svc")
	c.ApplyResourcesLoaded("ecs-svc", rows, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("no ecs-svc list on screen")
	}
	for _, id := range twins {
		n := 0
		for _, lr := range body.Rows {
			if lr.ResourceID == id {
				n++
			}
		}
		if n != 1 {
			t.Errorf("service %q shows as %d list rows, want 1", id, n)
		}
	}
}

// ridIdentityTargets are the types whose row ID carries a parent scope.
var ridIdentityTargets = []string{"ecs-svc", "codeartifact", "eb-rule", "policy"}

// ridOpensOrReports reports whether id either opens a row of target — a row of
// the target's list, or, for a target whose rows are not all listed (inline
// policies are not), a row its FetchByIDs resolver returns under that exact ID
// — or makes the resolver report why it cannot. The demo's retired AWS-managed
// policy is the second kind by design; an ID that silently opens nothing is
// neither.
func ridOpensOrReports(ctx context.Context, clients *awsclient.ServiceClients, byType map[string][]resource.Resource, target, id string) bool {
	for _, r := range byType[target] {
		if r.ID == id {
			return true
		}
	}
	td := resource.FindResourceType(target)
	if td == nil || td.FetchByIDs == nil {
		return false
	}
	got, err := td.FetchByIDs(ctx, clients, []string{id})
	for _, r := range got {
		if r.ID == id {
			return true
		}
	}
	return err != nil
}

// TestDemoRowIdentity_RelatedIDsIntoScopedTypesOpenARow: every related row
// that points at an ECS service, CodeArtifact repository, EventBridge rule or
// IAM policy carries IDs that open a row of that type. A checker that still
// emits the bare name where the ID carries a parent shows a count and opens
// nothing.
func TestDemoRowIdentity_RelatedIDsIntoScopedTypesOpenARow(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	ctx := context.Background()

	isTarget := map[string]bool{}
	for _, tt := range ridIdentityTargets {
		isTarget[tt] = true
	}
	emitted := map[string]int{}
	for _, td := range resource.AllResourceTypes() {
		for _, def := range td.Related {
			if !isTarget[def.TargetType] || def.Checker == nil {
				continue
			}
			for _, row := range byType[td.ShortName] {
				res := def.Checker(ctx, clients, row, cache)
				for _, id := range res.ResourceIDs() {
					emitted[def.TargetType]++
					if !ridOpensOrReports(ctx, clients, byType, def.TargetType, id) {
						t.Errorf("%s %q -> %s: related ID %q opens no %s row and reports nothing",
							td.ShortName, row.ID, def.DisplayName, id, def.TargetType)
					}
				}
			}
		}
	}
	for _, tt := range ridIdentityTargets {
		if emitted[tt] == 0 {
			t.Errorf("no demo related row points at %s, so this guard cannot see it", tt)
		}
	}
}

// TestDemoRowIdentity_SourcePanelsDoNotDependOnTheIDShape: a scoped type's own
// related panel and wave-2 findings are the same whether the row's ID is the
// qualified one or the bare name. A checker or enricher that reads the ID as
// the name loses every pivot and finding the parent-qualified ID carries.
func TestDemoRowIdentity_SourcePanelsDoNotDependOnTheIDShape(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	ctx := context.Background()

	for _, short := range []string{"ecs-svc", "codeartifact", "eb-rule"} {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s is not a registered type", short)
		}
		rows := byType[short]
		if len(rows) == 0 {
			t.Errorf("%s: no demo rows", short)
			continue
		}
		bare := make([]resource.Resource, len(rows))
		for i, r := range rows {
			bare[i] = r
			bare[i].ID = r.Name
		}

		for i := range rows {
			for _, def := range td.Related {
				if def.Checker == nil {
					continue
				}
				a := def.Checker(ctx, clients, rows[i], cache)
				b := def.Checker(ctx, clients, bare[i], cache)
				ida := append([]string(nil), a.ResourceIDs()...)
				idb := append([]string(nil), b.ResourceIDs()...)
				sort.Strings(ida)
				sort.Strings(idb)
				if a.EffectiveState() != b.EffectiveState() || a.Count() != b.Count() || !reflect.DeepEqual(ida, idb) {
					t.Errorf("%s %q -> %s: with ID %q: state %v count %d IDs %v; with the bare name: state %v count %d IDs %v",
						short, rows[i].Name, def.DisplayName, rows[i].ID,
						a.EffectiveState(), a.Count(), ida, b.EffectiveState(), b.Count(), idb)
				}
			}
		}

		enr, ok := awsclient.Wave2EnricherFor(short)
		if !ok || enr.Fn == nil {
			continue
		}
		ra, errA := enr.Fn(ctx, clients, rows, cache)
		rb, errB := enr.Fn(ctx, clients, bare, cache)
		if (errA == nil) != (errB == nil) {
			t.Errorf("%s wave-2: error with qualified IDs %v, with bare names %v", short, errA, errB)
		}
		for i := range rows {
			ca := ridCodes(ra.Findings[rows[i].ID])
			cb := ridCodes(rb.Findings[bare[i].ID])
			if !reflect.DeepEqual(ca, cb) {
				t.Errorf("%s %q wave-2 findings: with ID %q %v, with the bare name %v", short, rows[i].Name, rows[i].ID, ca, cb)
			}
		}
	}
}

func ridCodes(fs []domain.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, string(f.Code))
	}
	sort.Strings(out)
	return out
}
