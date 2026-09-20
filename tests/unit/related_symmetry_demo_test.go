package unit_test

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// symmetryOutOfScope are the types whose links are not a field read from two
// ends: a CloudWatch alarm matches on dimension name/value pairs, and a
// CloudTrail pivot matches on event records inside a time window.
var symmetryOutOfScope = map[string]bool{"alarm": true, "ct-events": true}

// relatedDefinite reports whether a result is a complete answer about the
// relation. A lookup that stopped short is a lower bound, one that searched
// nothing knows nothing, and heuristic matches are candidates that share a
// property rather than links AWS records: none of the three is a claim that a
// row is present or absent.
func relatedDefinite(r resource.RelatedCheckResult) bool {
	return r.Coverage() == domain.CoverageComplete
}

// mirrorPairsRequired are the pairs whose two directions read one AWS fact
// through one predicate (image URI, route target, ENI owner, mount-target
// file system, DNS name), so each must be declared Mirror on both ends.
var mirrorPairsRequired = [][2]string{
	{"ecr", "cb"}, {"ecr", "ecs-task"}, {"ecr", "lambda"},
	{"igw", "rtb"}, {"tgw", "rtb"}, {"nat", "rtb"},
	{"eni", "lambda"}, {"efs", "subnet"}, {"cf", "elb"}, {"cf", "r53"},
}

func relatedDefFor(src, target string) (resource.RelatedDef, bool) {
	for _, def := range resource.GetRelated(src) {
		if def.TargetType == target {
			return def, true
		}
	}
	return resource.RelatedDef{}, false
}

// TestRelatedSymmetry_DemoPairsListEachOther pins that a pair declared one
// relationship (RelatedDef.Mirror) is one relationship from both ends: row a
// of A lists row b of B exactly when b lists a. Two directions computing the
// same fact with different matching (substring against exact, blackhole
// routes counted against skipped) show an operator a link from one side that
// the other side denies. Pairs that are two relationships sharing a pivot
// (a secret's rotation function against the secrets a function's environment
// references) are not Mirror and are not held to it.
func TestRelatedSymmetry_DemoPairsListEachOther(t *testing.T) {
	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	// (a) Mirror is a property of the pair, so the two literals agree.
	type pair struct{ a, b string }
	var mirrors []pair
	for _, td := range types {
		for _, def := range resource.GetRelated(td.ShortName) {
			if !def.Mirror {
				continue
			}
			rev, ok := relatedDefFor(def.TargetType, td.ShortName)
			if !ok {
				t.Errorf("(a) %s -> %s is Mirror but %s registers no %s pivot", td.ShortName, def.TargetType, def.TargetType, td.ShortName)
				continue
			}
			if !rev.Mirror {
				t.Errorf("(a) %s -> %s is Mirror but %s -> %s is not", td.ShortName, def.TargetType, def.TargetType, td.ShortName)
				continue
			}
			if td.ShortName < def.TargetType {
				mirrors = append(mirrors, pair{td.ShortName, def.TargetType})
			}
		}
	}

	// (d) the pairs that read one fact through one predicate are declared so.
	for _, p := range mirrorPairsRequired {
		for _, dir := range [][2]string{{p[0], p[1]}, {p[1], p[0]}} {
			def, ok := relatedDefFor(dir[0], dir[1])
			if !ok {
				t.Errorf("(d) %s registers no %s pivot", dir[0], dir[1])
				continue
			}
			if !def.Mirror {
				t.Errorf("(d) %s -> %s is not declared Mirror; both ends read one fact through one predicate", dir[0], dir[1])
			}
		}
	}

	clients := demo.NewServiceClients()
	byType, cache := buildDemoTypeCache(t)
	ctx := context.Background()

	checked := 0
	for _, p := range mirrors {
		if symmetryOutOfScope[p.a] || symmetryOutOfScope[p.b] {
			continue
		}
		checked++
		defAB, _ := relatedDefFor(p.a, p.b)
		defBA, _ := relatedDefFor(p.b, p.a)
		rowsA, rowsB := byType[p.a], byType[p.b]

		ab := make(map[string]resource.RelatedCheckResult, len(rowsA))
		for _, ra := range rowsA {
			ab[ra.ID] = defAB.Checker(ctx, clients, ra, cache)
		}
		ba := make(map[string]resource.RelatedCheckResult, len(rowsB))
		for _, rb := range rowsB {
			ba[rb.ID] = defBA.Checker(ctx, clients, rb, cache)
		}

		// (b) a lists b exactly when b lists a, between two complete answers:
		// the side that lists must be claiming a link, and the silent side
		// must be claiming absence.
		links := 0
		var oneSided []string
		for _, ra := range rowsA {
			for _, rb := range rowsB {
				if !relatedDefinite(ab[ra.ID]) && !relatedDefinite(ba[rb.ID]) {
					continue
				}
				aListsB := relatedDefinite(ab[ra.ID]) && slices.Contains(ab[ra.ID].ResourceIDs(), rb.ID)
				bListsA := relatedDefinite(ba[rb.ID]) && slices.Contains(ba[rb.ID].ResourceIDs(), ra.ID)
				if aListsB && bListsA {
					links++
				}
				if aListsB && !bListsA && relatedDefinite(ba[rb.ID]) {
					oneSided = append(oneSided, fmt.Sprintf("%s %s lists %s %s; %s %s does not list it (lists %v)",
						p.a, ra.ID, p.b, rb.ID, p.b, rb.ID, ba[rb.ID].ResourceIDs()))
				}
				if bListsA && !aListsB && relatedDefinite(ab[ra.ID]) {
					oneSided = append(oneSided, fmt.Sprintf("%s %s lists %s %s; %s %s does not list it (lists %v)",
						p.b, rb.ID, p.a, ra.ID, p.a, ra.ID, ab[ra.ID].ResourceIDs()))
				}
			}
		}
		if len(oneSided) > 0 {
			t.Errorf("(b) %s <-> %s: %d one-sided links:\n  %s", p.a, p.b, len(oneSided), strings.Join(oneSided, "\n  "))
		}
		// (c) a pair with no link on the demo cannot fail (b).
		if links == 0 {
			t.Errorf("(c) %s <-> %s has no link both ends agree on in the demo fixtures, so its symmetry is unchecked", p.a, p.b)
		}
	}
	if checked == 0 {
		t.Errorf("(c) no Mirror pair is declared, so no symmetry is checked")
	}
}
