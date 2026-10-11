package unit_test

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t576Run is one enricher pass over demo rows through the wrapped clients.
type t576Run struct {
	res   awsclient.IssueEnricherResult
	err   error
	calls []t576Call
}

func t576Enrich(t *testing.T, fn awsclient.IssueEnricherFunc, rows []resource.Resource, cache resource.ResourceCache, fail func(service, op string, in any) error) t576Run {
	t.Helper()
	clients := demo.NewServiceClients()
	inj := &t576Injector{fail: fail}
	t576WrapClients(clients, inj)
	res, err := fn(context.Background(), clients, rows, cache)
	return t576Run{res: res, err: err, calls: inj.calls}
}

// t576ReadsOf hands an enricher the cache entries it declares, as dispatch does.
func t576ReadsOf(cache resource.ResourceCache, reads []string) resource.ResourceCache {
	out := resource.ResourceCache{}
	for _, name := range reads {
		if e, ok := cache[name]; ok {
			out[name] = e
		}
	}
	return out
}

// t576Names reports whether a request names row r: by its ID, its name or its
// ARN, whole or as the last segment of an ARN.
func t576Names(leaves []string, r resource.Resource) bool {
	keys := []string{r.ID, r.Name, r.Fields["arn"]}
	for _, l := range leaves {
		for _, k := range keys {
			if k != "" && (l == k || strings.HasSuffix(l, "/"+k) || strings.HasSuffix(l, ":"+k)) {
				return true
			}
		}
	}
	return false
}

func t576CallKey(c t576Call) string {
	return c.Service + "." + c.Op + "(" + strings.Join(t576Leaves(c.In), ",") + ")"
}

// t576FailureAllowed lists (type, Service.Op) pairs whose refusal may break an
// invariant below, each with the AWS behaviour that makes it so.
var t576FailureAllowed = map[string]string{
	"cf/S3.HeadBucket": "HeadBucket answers 403 for a bucket that exists and is not the caller's, and 404 for one that does not exist, so an AccessDenied is an answer (the origin bucket exists), not a failed read",
}

// A failed read is one of two things on a row: data the row still gets from
// another read, or a row marked not inspected. Never a silent blank, and never
// the end of signals that come from reads that did not fail. Each registered
// list enricher runs on the demo bench once clean and once per operation it
// calls, that operation refused with AccessDenied:
//
//   - the failure is recorded: the enricher returns an error;
//   - every row a refused request named is marked not inspected;
//   - a field or finding the clean run set is set again, or its row is marked;
//   - a finding lost from a marked row belongs to a row the refused read was
//     about (or the read was account-wide);
//   - every request the clean run made that does not carry data only the
//     refused operation returned is still made — an enricher that stops at
//     the first refusal skips signals that never depended on it.
func TestT576_FailedEnricherReadIsMarkedAndHidesNoOtherSignal(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	parents := map[string]bool{}
	for _, td := range resource.AllResourceTypes() {
		parents[td.ShortName] = true
	}

	var violations, noRows []string
	allowedHit := map[string]bool{}
	pairs := 0
	for _, w := range awsclient.AllWave2() {
		rows := byType[w.ShortName]
		if len(rows) == 0 {
			if parents[w.ShortName] {
				noRows = append(noRows, w.ShortName)
			} else {
				t.Logf("%s: child type, its rows need a parent and are out of this guard's scope", w.ShortName)
			}
			continue
		}
		reads := t576ReadsOf(cache, w.Enricher.Reads)
		base := t576Enrich(t, w.Enricher.Fn, rows, reads, nil)
		if base.err != nil {
			t.Errorf("%s: clean demo run returned an error: %v", w.ShortName, base.err)
			continue
		}

		type op struct{ service, name string }
		var ops []op
		seen := map[op]bool{}
		for _, c := range base.calls {
			o := op{c.Service, c.Op}
			if !seen[o] {
				seen[o] = true
				ops = append(ops, o)
			}
		}

		for _, o := range ops {
			pairs++
			label := w.ShortName + "/" + o.service + "." + o.name
			got := t576Enrich(t, w.Enricher.Fn, rows, reads, func(service, name string, _ any) error {
				if service == o.service && name == o.name {
					return t576Denied(service, name)
				}
				return nil
			})
			var found []string
			report := func(format string, args ...any) {
				found = append(found, label+": "+fmt.Sprintf(format, args...))
			}

			var failed []t576Call
			made := map[string]bool{}
			for _, c := range got.calls {
				if c.Failed {
					failed = append(failed, c)
				} else {
					made[t576CallKey(c)] = true
				}
			}
			if len(failed) > 0 && got.err == nil {
				report("%d refused calls, enricher returned no error", len(failed))
			}

			marked := func(id string) bool {
				_, ok := got.res.TruncatedIDs[id]
				return ok
			}
			namedRows := map[string]bool{}
			accountWide := false
			for _, c := range failed {
				leaves := t576Leaves(c.In)
				named := false
				for _, r := range rows {
					if t576Names(leaves, r) {
						named = true
						namedRows[r.ID] = true
					}
				}
				if !named {
					accountWide = true
				}
			}
			for _, r := range rows {
				if namedRows[r.ID] && !marked(r.ID) {
					report("row %s: its %s was refused, row not marked not inspected", r.ID, o.name)
				}
			}

			for id, fields := range base.res.FieldUpdates {
				for k := range fields {
					if _, ok := got.res.FieldUpdates[id][k]; !ok && !marked(id) {
						report("row %s: field %s blank and row not marked", id, k)
					}
				}
			}
			// derived is what only the refused operation's answers carried;
			// otherAnswers is what the reads that still answered carried.
			derived := map[string]bool{}
			echoed := map[string]bool{}
			answered := map[string]bool{}
			for _, c := range base.calls {
				if c.Service == o.service && c.Op == o.name {
					for _, l := range t576Leaves(c.In) {
						echoed[l] = true
					}
				}
			}
			for _, c := range base.calls {
				if c.Service == o.service && c.Op == o.name {
					for _, l := range t576Leaves(c.Out) {
						answered[l] = true
						if !echoed[l] {
							derived[l] = true
						}
					}
				}
			}
			otherAnswers := map[string]string{}
			for _, c := range got.calls {
				if !c.Failed {
					for _, l := range t576Leaves(c.Out) {
						otherAnswers[l] = c.Op
					}
				}
			}

			for id, findings := range base.res.Findings {
				for _, f := range findings {
					if slices.ContainsFunc(got.res.Findings[id], func(g domain.Finding) bool { return g.Code == f.Code }) {
						continue
					}
					// A supporting row quoting a value only another read
					// returned names the read that raised the finding.
					evidence := ""
					for _, dr := range base.res.AttentionDetails[id][f.Code].Rows {
						if from, ok := otherAnswers[dr.Value]; ok && len(dr.Value) >= 4 && !answered[dr.Value] {
							evidence = fmt.Sprintf("%q from %s", dr.Value, from)
						}
					}
					switch {
					case !marked(id):
						report("row %s: finding %s lost and row not marked", id, f.Code)
					case !namedRows[id] && !accountWide:
						report("row %s: finding %s lost, but no refused %s call was about this row", id, f.Code, o.name)
					case evidence != "":
						report("row %s: finding %s lost, though its evidence %s still answered", id, f.Code, evidence)
					}
				}
			}

			for _, c := range base.calls {
				if c.Service == o.service && c.Op == o.name {
					continue
				}
				if slices.ContainsFunc(t576Leaves(c.In), func(l string) bool { return derived[l] }) {
					continue
				}
				if key := t576CallKey(c); !made[key] {
					report("independent request %s was not made", key)
					made[key] = true
				}
			}

			if reason, ok := t576FailureAllowed[w.ShortName+"/"+o.service+"."+o.name]; ok {
				allowedHit[w.ShortName+"/"+o.service+"."+o.name] = len(found) > 0
				for _, v := range found {
					t.Logf("allowed (%s): %s", reason, v)
				}
				continue
			}
			violations = append(violations, found...)
		}
	}

	for key := range t576FailureAllowed {
		if !allowedHit[key] {
			t.Errorf("%s is allowlisted but breaks no invariant; remove it", key)
		}
	}
	if len(noRows) > 0 {
		sort.Strings(noRows)
		t.Errorf("list enrichers with no demo rows to refuse reads under: %v", noRows)
	}
	if pairs < 90 {
		t.Fatalf("refused %d (enricher, operation) pairs; the bench lost its enrichers", pairs)
	}
	sort.Strings(violations)
	for _, v := range violations {
		t.Error(v)
	}
	t.Logf("%d (enricher, operation) pairs refused, %d violations", pairs, len(violations))
}
