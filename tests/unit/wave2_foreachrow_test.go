package unit_test

// A Wave 2 loop that stops at its deadline says
// which rows it never reached, and hands the error back.
//
// A row whose check never started is not a row that was inspected and found
// clean. Without a mark it renders exactly like one, and the issue count the
// enricher contributes reads as complete.

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func bkEmptyResult() awsclient.IssueEnricherResult {
	r, _ := awsclient.InFetcherWave2Sentinel(context.Background(), nil, nil, nil)
	return r
}

func bkIDs(prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%03d", prefix, i)
	}
	return ids
}

func TestForEachRow_SequentialStopMarksOnlyTheRowsNeverStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ids := bkIDs("acme-rule", 5)
	result := bkEmptyResult()

	var ran []int
	err := awsclient.ForEachRow(ctx, &result, ids, 1, func(i int) {
		ran = append(ran, i)
		if i == 2 {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("ForEachRow returned %v, want the context error", err)
	}
	if !slices.Equal(ran, []int{0, 1, 2}) {
		t.Errorf("fn ran for %v, want [0 1 2]", ran)
	}
	for _, id := range ids[:3] {
		if mark, ok := result.TruncatedIDs[id]; ok {
			t.Errorf("row %q was inspected but carries the mark %q", id, mark)
		}
	}
	for _, id := range ids[3:] {
		if got, ok := result.TruncatedIDs[id]; !ok || got != awsclient.CheckDeadline {
			t.Errorf("row %q was never started: TruncatedIDs = (%q, %v), want (%q, true)", id, got, ok, awsclient.CheckDeadline)
		}
	}
}

// Under parallelism which rows started is decided by the scheduler, so the
// pin is the partition: every row either ran or carries the deadline mark,
// never both and never neither.
func TestForEachRow_ParallelStopPartitionsStartedAndMarkedRows(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ids := bkIDs("acme-queue", 40)
	result := bkEmptyResult()

	var mu sync.Mutex
	ran := map[int]bool{}
	err := awsclient.ForEachRow(ctx, &result, ids, 4, func(i int) {
		mu.Lock()
		ran[i] = true
		mu.Unlock()
		if i == 5 {
			cancel()
		}
		time.Sleep(2 * time.Millisecond)
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("ForEachRow returned %v, want the context error", err)
	}
	marked := 0
	for i, id := range ids {
		mark, ok := result.TruncatedIDs[id]
		switch {
		case ran[i] && ok:
			t.Errorf("row %q ran and is also marked %q", id, mark)
		case !ran[i] && !ok:
			t.Errorf("row %q never ran and carries no mark — it renders as inspected-and-healthy", id)
		case !ran[i] && mark != awsclient.CheckDeadline:
			t.Errorf("row %q never ran: mark = %q, want %q", id, mark, awsclient.CheckDeadline)
		case !ran[i]:
			marked++
		}
	}
	if marked == 0 {
		t.Fatalf("every row ran (%d) — the stop was never observed", len(ran))
	}
}

func TestForEachRow_CompletedLoopMarksNothing(t *testing.T) {
	ids := bkIDs("acme-topic", 10)
	result := bkEmptyResult()

	var mu sync.Mutex
	ran := 0
	err := awsclient.ForEachRow(context.Background(), &result, ids, 4, func(int) {
		mu.Lock()
		ran++
		mu.Unlock()
	})

	if err != nil {
		t.Errorf("ForEachRow returned %v on a loop that finished", err)
	}
	if ran != len(ids) {
		t.Errorf("fn ran %d times, want %d", ran, len(ids))
	}
	if len(result.TruncatedIDs) != 0 {
		t.Errorf("a finished loop marked rows: %v", result.TruncatedIDs)
	}
}

// bkEBFake answers ListTargetsByRule from a per-rule table and records which
// rules it was asked about. A rule missing from the table has one target with
// a dead-letter queue, which is the healthy shape.
type bkEBFake struct {
	awsclient.EventBridgeAPI
	mu      sync.Mutex
	asked   map[string]bool
	targets map[string][]eventbridgetypes.Target
	errs    map[string]error
}

func (f *bkEBFake) ListTargetsByRule(_ context.Context, in *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	name := aws.ToString(in.Rule)
	f.mu.Lock()
	if f.asked == nil {
		f.asked = map[string]bool{}
	}
	f.asked[name] = true
	f.mu.Unlock()
	if err := f.errs[name]; err != nil {
		return nil, err
	}
	if t, ok := f.targets[name]; ok {
		return &eventbridge.ListTargetsByRuleOutput{Targets: t}, nil
	}
	return &eventbridge.ListTargetsByRuleOutput{Targets: []eventbridgetypes.Target{bkDLQTarget(name)}}, nil
}

func bkDLQTarget(rule string) eventbridgetypes.Target {
	return eventbridgetypes.Target{
		Id:               aws.String(rule + "-lambda"),
		Arn:              aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + rule),
		DeadLetterConfig: &eventbridgetypes.DeadLetterConfig{Arn: aws.String("arn:aws:sqs:us-east-1:123456789012:acme-eb-dlq")},
	}
}

func bkEBRule(name, state string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name, Type: "eb-rule",
		Fields: map[string]string{
			"name":      name,
			"state":     state,
			"event_bus": "default",
			"arn":       "arn:aws:events:us-east-1:123456789012:rule/" + name,
		},
	}
}

// TestEBRuleDeadline_RulesNeverAskedAreMarkedAndTheErrorSurfaces: the Wave 2
// deadline passes while the rule loop is still scheduling.
// A rule whose ListTargetsByRule was never sent has no answer, so it must say
// so; a rule that was asked has one and must not.
func TestEBRuleDeadline_RulesNeverAskedAreMarkedAndTheErrorSurfaces(t *testing.T) {
	rules := make([]resource.Resource, 0, 20)
	for _, id := range bkIDs("acme-orders-rule", 20) {
		rules = append(rules, bkEBRule(id, "ENABLED"))
	}
	fake := &bkEBFake{}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	res, err := awsclient.EnrichEventBridgeRuleTargets(ctx, &awsclient.ServiceClients{EventBridge: fake}, rules, nil)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("enricher returned %v, want an error carrying context.DeadlineExceeded — the deadline was swallowed", err)
	}
	unasked := 0
	for _, r := range rules {
		mark, marked := res.TruncatedIDs[r.ID]
		if fake.asked[r.ID] {
			if marked {
				t.Errorf("rule %q was asked and answered, yet is marked %q", r.ID, mark)
			}
			continue
		}
		unasked++
		if mark != "stopped at the Wave 2 deadline" {
			t.Errorf("rule %q was never asked: TruncatedIDs = (%q, %v), want (%q, true)",
				r.ID, mark, marked, "stopped at the Wave 2 deadline")
		}
		if len(res.Findings[r.ID]) != 0 {
			t.Errorf("rule %q was never asked yet carries findings %v", r.ID, res.Findings[r.ID])
		}
	}
	if unasked == 0 {
		t.Fatal("every rule was asked despite an expired deadline — the stop was never observed")
	}
	if !res.Truncated {
		t.Error("result.Truncated = false: the issue count reads complete although rules were never checked")
	}
}

// TestWave2Deadline_EveryEnricherAccountsForRowsItNeverReached runs each
// registered enricher twice over the demo rows: once with time to finish and
// once past its deadline. Every finding the full run raised must, in the late
// run, either be raised again or sit on a row marked not inspected. A finding
// that simply disappears is a row the late run claims is clean.
func TestWave2Deadline_EveryEnricherAccountsForRowsItNeverReached(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	for _, w := range awsclient.AllWave2() {
		rows := byType[w.ShortName]
		if len(rows) == 0 {
			continue
		}
		t.Run(w.ShortName, func(t *testing.T) {
			rowIDs := make(map[string]bool, len(rows))
			for _, r := range rows {
				rowIDs[r.ID] = true
			}
			full, _ := w.Enricher.Fn(context.Background(), clients, rows, cache) //nolint:errcheck // the full run's error is not under test

			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			late, _ := w.Enricher.Fn(ctx, clients, rows, cache) //nolint:errcheck // the late run is judged by its rows

			for id, fs := range full.Findings {
				if !rowIDs[id] {
					continue
				}
				for _, f := range fs {
					if slices.ContainsFunc(late.Findings[id], func(g domain.Finding) bool { return g.Code == f.Code }) {
						continue
					}
					if _, marked := late.TruncatedIDs[id]; marked {
						continue
					}
					t.Errorf("%s: after the deadline row %q lost %s and carries no not-inspected mark", w.ShortName, id, f.Code)
				}
			}
		})
	}
}

// TestWave2Loops_NoCallerDiscardsTheLoopError: the loop's error is how a
// stop reaches the result. A caller that drops it reports a stopped loop as a
// finished one.
func TestWave2Loops_NoCallerDiscardsTheLoopError(t *testing.T) {
	loops := map[string]bool{"ForEachParallel": true, "ForEachRow": true}
	isLoop := func(e ast.Expr) bool {
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			return loops[fn.Name]
		case *ast.SelectorExpr:
			return loops[fn.Sel.Name]
		}
		return false
	}

	var offenders []string
	for _, dir := range []string{"core", "internal", "cmd"} {
		root, err := filepath.Abs(filepath.Join("..", "..", dir))
		if err != nil {
			t.Fatalf("resolve %s: %v", dir, err)
		}
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.ExprStmt:
					if isLoop(s.X) {
						offenders = append(offenders, fset.Position(s.Pos()).String())
					}
				case *ast.AssignStmt:
					if len(s.Rhs) == 1 && isLoop(s.Rhs[0]) && len(s.Lhs) == 1 {
						if id, ok := s.Lhs[0].(*ast.Ident); ok && id.Name == "_" {
							offenders = append(offenders, fset.Position(s.Pos()).String())
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("%d call(s) discard the Wave 2 loop error:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
}
