package unit_test

// t542_resolver_guard_test.go — one resolver per target type. Every type a
// related pivot or a navigable field points at reads the ARN of each of its
// rows back to that row's ID through its own RefToID, and every ID a related
// checker emits or a navigable field opens is one that resolver hands back
// unchanged. A checker, projector or field that builds an ID any other way —
// a raw ARN, a hand-cut segment — fails here on the first demo row that shows
// it, whichever type it is.

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// t542Targets is every type a catalog Related entry or NavigableField points at.
func t542Targets() []string {
	var out []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range resource.GetRelated(td.ShortName) {
			out = append(out, def.TargetType)
		}
		for _, nf := range resource.GetNavigableFields(td.ShortName) {
			out = append(out, nf.TargetType)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// t542RowARN is the row's own ARN as AWS returns it on the raw struct: the
// field named Arn/ARN, or <StructName>Arn/ARN (TrailARN, DBClusterArn,
// EnvironmentArn, …), and failing that Fields["arn"]. "" when it has none.
func t542RowARN(r resource.Resource) string {
	v := reflect.ValueOf(r.RawStruct)
	for v.Kind() == reflect.Pointer && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		name := v.Type().Name()
		for _, f := range []string{"Arn", "ARN", name + "Arn", name + "ARN"} {
			fv := v.FieldByName(f)
			if fv.IsValid() && fv.Kind() == reflect.Pointer && !fv.IsNil() && fv.Elem().Kind() == reflect.String {
				if a := fv.Elem().String(); arn.IsARN(a) {
					return a
				}
			}
		}
	}
	if a := r.Fields["arn"]; arn.IsARN(a) {
		return a
	}
	return ""
}

func TestResolverGuard_EveryTargetReadsItsRowsARNBackToTheID(t *testing.T) {
	b := newRefBench(t)
	var bad []string
	checked := 0
	for _, target := range t542Targets() {
		for _, r := range b.byType[target] {
			a := t542RowARN(r)
			// A list row's ARN is its own account's; one naming another
			// account is a fixture that cannot occur and names no local row.
			if parsed, err := arn.Parse(a); a == "" || a == r.ID || err == nil && parsed.AccountID != "" && parsed.AccountID != refAccount {
				continue
			}
			checked++
			if id, ok := resource.ResolveRef(target, a, b.rc(target)); !ok || id != r.ID {
				bad = append(bad, fmt.Sprintf("%s: ResolveRef(%q) = (%q, %v), want the row ID %q", target, a, id, ok, r.ID))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no target row carries an ARN beside its ID")
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d target rows whose ARN their type's resolver does not read back to the ID:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}

// resolvesToItself reports whether id is what target's resolver hands back
// for it against the loaded list: the canonical spelling, not a raw ARN or a
// cut that happens to open.
func resolvesToItself(b refBench, target, id string) bool {
	got, ok := resource.ResolveRef(target, id, b.rc(target))
	return ok && got == id
}

func TestResolverGuard_EmittedAndNavigableIDsAreTheResolversOwn(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	ctx := context.Background()
	var bad []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range resource.GetRelated(td.ShortName) {
			for _, res := range b.byType[td.ShortName] {
				for _, id := range def.Checker(ctx, clients, res, b.cache).ResourceIDs() {
					if !resolvesToItself(b, def.TargetType, id) {
						bad = append(bad, fmt.Sprintf("related %s → %s (%s): source %q emits %q", td.ShortName, def.TargetType, def.DisplayName, res.ID, id))
					}
				}
			}
		}
	}
	c := refDetailController(t, b)
	for _, td := range resource.AllResourceTypes() {
		for _, res := range b.byType[td.ShortName] {
			for _, f := range openDetail(c, td.ShortName, res) {
				if f.IsNavigable && !resolvesToItself(b, f.TargetType, navTarget(f)) {
					bad = append(bad, fmt.Sprintf("navigable %s.%s → %s: source %q opens %q", td.ShortName, f.Path, f.TargetType, res.ID, navTarget(f)))
				}
			}
			c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d IDs not read through their target type's resolver:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}

// Every id a related checker emits and every target a navigable field opens
// on the demo data names a row of its target type: one the loaded list holds,
// or one the type's by-ID fetch returns, as the panel's lazy add and Enter's
// drill read it (an AWS-managed policy or key is on no list). A resolver fixed
// point is not enough: a phantom that happens to look like an id of its type
// (a hand cut on any variable, a per-field Resolve returning the wrong
// segment) resolves to itself and opens nothing. A result read in another
// Region, and a field naming one, name rows of that Region's list, which the
// bench does not load. Three demo witnesses name a resource AWS no longer
// answers for, by design: a DeleteBucket event, an AssumeRole of a deleted
// role, and a role's attachment of a policy AWS has retired.
func TestResolverGuard_EmittedAndNavigableIDsNameLoadedRows(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	ctx := context.Background()
	gone := map[string]bool{
		demofixtures.CtEventDeletedBucket:                                  true,
		demofixtures.CtEventDeletedRole:                                    true,
		"arn:aws:iam::aws:policy/" + demofixtures.RetiredManagedPolicyName: true,
	}
	named := func(target, id string) bool {
		if b.has(target, id) || gone[id] {
			return true
		}
		fetch := resource.GetFetchByIDs(target)
		if fetch == nil {
			return false
		}
		rows, _ := fetch(ctx, clients, []string{id})
		return slices.ContainsFunc(rows, func(r resource.Resource) bool { return r.ID == id })
	}
	var bad []string
	for _, td := range resource.AllResourceTypes() {
		for _, def := range resource.GetRelated(td.ShortName) {
			for _, res := range b.byType[td.ShortName] {
				r := def.Checker(ctx, clients, res, b.cache)
				if r.Region() != "" && r.Region() != refRegion {
					continue
				}
				for _, id := range r.ResourceIDs() {
					if !named(def.TargetType, id) {
						bad = append(bad, fmt.Sprintf("related %s → %s (%s): source %q emits %q", td.ShortName, def.TargetType, def.DisplayName, res.ID, id))
					}
				}
			}
		}
	}
	c := refDetailController(t, b)
	for _, td := range resource.AllResourceTypes() {
		for _, res := range b.byType[td.ShortName] {
			for _, f := range openDetail(c, td.ShortName, res) {
				// A field's own Resolve is read directly: a phantom it returns
				// makes the field silently non-navigable, which no opened
				// target shows.
				for _, nf := range resource.GetNavigableFields(td.ShortName) {
					if nf.Resolve == nil || nf.FieldPath != f.Path {
						continue
					}
					value := strings.TrimPrefix(strings.TrimSpace(f.Value), "- ")
					if ref := nf.Resolve(res, value, b.byType[nf.TargetType]); ref != "" && !named(nf.TargetType, ref) {
						bad = append(bad, fmt.Sprintf("resolve %s.%s → %s: source %q reads %q", td.ShortName, f.Path, nf.TargetType, res.ID, ref))
					}
				}
				if region := resource.RefRegion(f.Value); !f.IsNavigable || region != "" && region != refRegion {
					continue
				}
				if !gone[res.ID] && !named(f.TargetType, navTarget(f)) {
					bad = append(bad, fmt.Sprintf("navigable %s.%s → %s: source %q opens %q", td.ShortName, f.Path, f.TargetType, res.ID, navTarget(f)))
				}
			}
			c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("%d IDs name no loaded row of their target type:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}
