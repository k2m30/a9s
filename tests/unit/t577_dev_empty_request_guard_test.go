package unit_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t577EmptyStringFields names every top-level *string field of an SDK input
// that is set to "". An identifier sent empty is a request AWS rejects as
// invalid, never a read of the row it was meant for.
func t577EmptyStringFields(in any) []string {
	v := reflect.ValueOf(in)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	var out []string
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Type() == reflect.TypeFor[*string]() && !f.IsNil() && f.Elem().String() == "" {
			out = append(out, v.Type().Field(i).Name)
		}
	}
	return out
}

// No list enricher sends a request with an empty identifier on the demo bench,
// which carries rows whose describe call failed: a field only the describe
// returns is absent on such a row, and the row is marked rather than asked
// about with nothing.
func TestT577Dev_NoEnricherRequestCarriesAnEmptyIdentifier(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	for _, w := range awsclient.AllWave2() {
		rows := byType[w.ShortName]
		if len(rows) == 0 {
			continue
		}
		run := t576Enrich(t, w.Enricher.Fn, rows, t576ReadsOf(cache, w.Enricher.Reads), nil)
		for _, c := range run.calls {
			if empty := t577EmptyStringFields(c.In); len(empty) > 0 {
				t.Errorf("%s: %s.%s sent with empty %v", w.ShortName, c.Service, c.Op, empty)
			}
		}
	}
}

var t577ShortAnswerErrorRe = regexp.MustCompile(`(?i)(Errorf|errors\.New)\("[^"]*(\bnil\b[^"]*\b(response|answer|body)\b|returned nil|no [a-z ]+ returned|empty response)`)

// An answer that came back without the field a reader needs is an
// UnusableAnswerErr, which names the call and the field and is recorded
// through MarkUnusable; a hand-phrased error carries neither and reads as a
// refused call.
func TestT577Dev_ShortAnswersAreUnusableAnswerErr(t *testing.T) {
	err := filepath.WalkDir("../../core", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for i, line := range strings.Split(string(src), "\n") {
			if t577ShortAnswerErrorRe.MatchString(line) {
				t.Errorf("%s:%d: short answer phrased by hand, want UnusableAnswerErr: %s", path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var t577FailedForCountRe = regexp.MustCompile(`failed for (\d+) of (\d+)`)

// Every list fetcher, with each operation it calls refused in turn, states its
// failures as a subset of what it attempted: an aggregate counts a call's
// failures against that call's own attempts, never against another call's.
func TestT577Dev_FetcherFailureCountsAreSubsetsOfAttempts(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		fetch := resource.GetPaginatedFetcher(td.ShortName)
		if fetch == nil {
			continue
		}
		clean := &t576Injector{}
		clients := demo.NewServiceClients()
		t576WrapClients(clients, clean)
		_, _ = fetch(context.Background(), clients, "")
		ops := map[string]bool{}
		for _, c := range clean.calls {
			ops[c.Op] = true
		}
		for op := range ops {
			inj := &t576Injector{fail: t576DenyOp(op)}
			clients := demo.NewServiceClients()
			t576WrapClients(clients, inj)
			_, err := fetch(context.Background(), clients, "")
			if err == nil {
				continue
			}
			for _, m := range t577FailedForCountRe.FindAllStringSubmatch(err.Error(), -1) {
				n, _ := strconv.Atoi(m[1])
				total, _ := strconv.Atoi(m[2])
				if total == 0 || n > total {
					t.Errorf("%s with %s refused: %q in %q", td.ShortName, op, m[0], err)
				}
			}
		}
	}
}

// t577FetchUnit is one list a fetcher returns on the demo bench: a top-level
// type, or a child type under one demo parent row.
type t577FetchUnit struct {
	typ   string
	name  string
	fetch func(clients *awsclient.ServiceClients) ([]resource.Resource, error)
}

func t577FetchUnits(t *testing.T) []t577FetchUnit {
	t.Helper()
	var units []t577FetchUnit
	for _, td := range resource.AllResourceTypes() {
		fetch := resource.GetPaginatedFetcher(td.ShortName)
		if fetch == nil {
			continue
		}
		units = append(units, t577FetchUnit{typ: td.ShortName, name: td.ShortName, fetch: func(c *awsclient.ServiceClients) ([]resource.Resource, error) {
			res, err := fetch(context.Background(), c, "")
			return res.Resources, err
		}})
		parents, _ := fetch(context.Background(), demo.NewServiceClients(), "")
		for _, child := range td.Children {
			childFetch := resource.GetPaginatedChildFetcher(child.ChildType)
			if childFetch == nil {
				continue
			}
			for i := range min(3, len(parents.Resources)) {
				parent := parents.Resources[i]
				pctx := resource.ResolveChildContext(child, &parent, nil)
				units = append(units, t577FetchUnit{typ: child.ChildType, name: child.ChildType + " under " + parent.ID, fetch: func(c *awsclient.ServiceClients) ([]resource.Resource, error) {
					res, err := childFetch(context.Background(), c, pctx, "")
					return res.Resources, err
				}})
			}
		}
	}
	return units
}

// A per-item call that fails never removes the item a list call named: every
// fetcher, top-level or child, with each operation it calls refused in turn,
// still returns every row whose identity a refused request carried.
func TestT577Dev_RefusedDescribeNeverDropsAListedRow(t *testing.T) {
	for _, u := range t577FetchUnits(t) {
		clean := &t576Injector{}
		clients := demo.NewServiceClients()
		t576WrapClients(clients, clean)
		rows, _ := u.fetch(clients)
		if len(rows) == 0 {
			continue
		}
		ops := map[string]bool{}
		for _, c := range clean.calls {
			ops[c.Op] = true
		}
		for op := range ops {
			inj := &t576Injector{fail: t576DenyOp(op)}
			clients := demo.NewServiceClients()
			t576WrapClients(clients, inj)
			got, _ := u.fetch(clients)
			kept := map[string]bool{}
			for _, r := range got {
				kept[r.ID] = true
			}
			for _, r := range rows {
				if kept[r.ID] {
					continue
				}
				for _, c := range inj.calls {
					if !c.Failed || !t576Names(t576Leaves(c.In), r) {
						continue
					}
					t.Errorf("%s: %s refused for %s dropped the row", u.name, op, r.ID)
					break
				}
			}
		}
	}
}

var t577PageCapFailureKept = map[string]string{
	"kms.go": "the ListAliases failure is what marks the kms list a lower bound for alias references (RowsHeld); the cap needs a field-level lower bound before it can drop the failure",
}

// a9s's own page cap is a lower bound on what was read, never a failed AWS
// call: no site phrases it as one, apart from those listed above.
func TestT577Dev_PageCapIsNeverAFailedCall(t *testing.T) {
	re := regexp.MustCompile(`more than (%d|\d+) pages`)
	seen := map[string]bool{}
	err := filepath.WalkDir("../../core", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !re.Match(src) {
			return nil
		}
		if _, ok := t577PageCapFailureKept[filepath.Base(path)]; ok {
			seen[filepath.Base(path)] = true
			return nil
		}
		t.Errorf("%s: the page cap is phrased as a failure; keep what was read instead", path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for file := range t577PageCapFailureKept {
		if !seen[file] {
			t.Errorf("%s no longer phrases the cap as a failure; remove it from t577PageCapFailureKept", file)
		}
	}
}
