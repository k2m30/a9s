package unit_test

import (
	"context"
	"go/ast"
	"go/token"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// A code two types declare is judged on each type's row by that type's own
// lifecycle and declaration: a value one type's table classes as non-live
// withholds a posture finding there and nowhere else.
func TestT2Dev_SharedCodeIsJudgedByEachTypesOwnDeclaration(t *testing.T) {
	declared := map[domain.FindingCode][]catalog.ResourceTypeDef{}
	for _, td := range slices.Concat(catalog.All(), catalog.AllChildren()) {
		for _, f := range td.Findings {
			declared[f.Code] = append(declared[f.Code], td)
		}
	}
	shared := 0
	for code, tds := range declared {
		if len(tds) < 2 {
			continue
		}
		shared++
		for _, td := range tds {
			i := slices.IndexFunc(td.Findings, func(f catalog.FindingDef) bool { return f.Code == code })
			def := td.Findings[i]
			if len(def.OnlyKinds)+len(def.NotKinds) > 0 {
				continue
			}
			for _, raw := range append([]string{""}, td.StateEnum...) {
				class, state := catalog.Lifecycle(td, raw)
				want := code == state || (len(def.AppliesInStates) == 0 || slices.Contains(def.AppliesInStates, raw)) &&
					(len(def.AppliesIn) == 0 && class.Live() || slices.Contains(def.AppliesIn, class))
				row := domain.Resource{ID: "row", Fields: map[string]string{td.StatusKey(): raw}}
				if got := catalog.AppliesToRow(td.ShortName, code, row); got != want {
					t.Errorf("%s on a %s row in %q: AppliesToRow = %v, its own declaration says %v", code, td.ShortName, raw, got, want)
				}
			}
		}
	}
	if shared == 0 {
		t.Fatal("no code is declared by two types; the catalog lost the case this guards")
	}
}

var t2UnfilledSlot = regexp.MustCompile(`<[^<>]*>`)

// Every phrase and detail a demo row renders, from either wave, is a whole
// sentence: no "<...>" slot of a declared wording is left for the reader.
func TestT2Dev_NoDemoFindingHoldsAnUnfilledSlot(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	for _, td := range resource.AllResourceTypes() {
		for _, r := range mergeWave2Findings(t, td, byType[td.ShortName], cache, clients) {
			for _, f := range r.Findings {
				for _, text := range []string{f.Phrase, f.Detail} {
					if slot := t2UnfilledSlot.FindString(text); slot != "" {
						t.Errorf("%s/%s %s renders %q with %s unfilled", td.ShortName, r.ID, f.Code, text, slot)
					}
				}
			}
		}
	}
}

// Every finding a demo row carries, from either wave, on a top-level row or
// on any child screen the drill reaches, is a code the row's own type
// declares, so the door judges it by that type's rules.
func TestT2Dev_EveryDemoFindingIsDeclaredOnItsRowsType(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	seen := map[string]bool{}
	checked := 0
	var walk func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int)
	walk = func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int) {
		own := catalog.FindAny(td.ShortName)
		if own == nil {
			t.Fatalf("%s is not in the catalog", td.ShortName)
		}
		for _, r := range mergeWave2Findings(t, td, rows, cache, clients) {
			for _, f := range r.Findings {
				checked++
				key := td.ShortName + " " + string(f.Code)
				if seen[key] || slices.ContainsFunc(own.Findings, func(d catalog.FindingDef) bool { return d.Code == f.Code }) {
					continue
				}
				seen[key] = true
				t.Errorf("%s/%s carries %s, which %s does not declare", td.ShortName, r.ID, f.Code, td.ShortName)
			}
		}
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
				if err == nil && len(kids) > 0 {
					walk(*ctd, kids, pctx, depth+1)
				}
			}
		}
	}
	for _, td := range resource.AllResourceTypes() {
		walk(td, byType[td.ShortName], nil, 0)
	}
	if checked < 500 {
		t.Fatalf("checked %d findings; the bench lost its rows", checked)
	}
}

// A row's Findings are written by the emit door's own body alone: no other
// function in core/aws assigns the field, writes an element of it, takes its
// address, or sets it in a Resource literal.
func TestT2Dev_FindingsWrittenOnlyInsideTheDoorBody(t *testing.T) {
	const resourceType = "github.com/k2m30/a9s/v3/core/domain.Resource"
	door := []string{"emitWave1", "rebuiltWave1"}
	sites := map[string]bool{}
	for _, p := range t2LoadCoreAWS(t) {
		isFindings := func(e ast.Expr) bool {
			sel, ok := e.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Findings" {
				return false
			}
			s, ok := p.TypesInfo.Selections[sel]
			return ok && t2TypeName(s.Recv()) == resourceType
		}
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && slices.Contains(door, fd.Name.Name) {
					continue
				}
				ast.Inspect(d, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.AssignStmt:
						for _, lhs := range x.Lhs {
							for {
								idx, ok := lhs.(*ast.IndexExpr)
								if !ok {
									break
								}
								lhs = idx.X
							}
							if isFindings(lhs) {
								sites[t2Site(p, lhs.Pos())] = true
							}
						}
					case *ast.UnaryExpr:
						if x.Op == token.AND && isFindings(x.X) {
							sites[t2Site(p, x.Pos())] = true
						}
					case *ast.CompositeLit:
						tv, ok := p.TypesInfo.Types[x]
						if !ok || t2TypeName(tv.Type) != resourceType {
							return true
						}
						for _, el := range x.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Findings" {
									sites[t2Site(p, kv.Pos())] = true
								}
							}
						}
					}
					return true
				})
			}
		}
	}
	if len(sites) > 0 {
		list := slices.Sorted(maps.Keys(sites))
		t.Errorf("%d sites write a Resource's Findings outside the emit door's body:\n  %s", len(list), strings.Join(list, "\n  "))
	}
}

// A multi-Region replica takes its rotation from its primary key, so the
// demo replica, whose rotation status reads off, carries no rotation finding,
// and the classifier names it apart from a key that rotates on its own.
func TestT2Dev_KMSReplicaCarriesNoRotationFinding(t *testing.T) {
	for keyType, want := range map[kmstypes.MultiRegionKeyType]bool{kmstypes.MultiRegionKeyTypePrimary: true, kmstypes.MultiRegionKeyTypeReplica: false} {
		key := domain.Resource{ID: "mrk", RawStruct: kmstypes.KeyMetadata{
			KeyManager: kmstypes.KeyManagerTypeCustomer, KeySpec: kmstypes.KeySpecSymmetricDefault, Origin: kmstypes.OriginTypeAwsKms,
			MultiRegionConfiguration: &kmstypes.MultiRegionConfiguration{MultiRegionKeyType: keyType},
		}}
		if got := catalog.AppliesToRow("kms", "kms.rotation-disabled", key); got != want {
			t.Errorf("a multi-Region %s key: rotation-disabled applies = %v, want %v", keyType, got, want)
		}
	}
	byType, _ := buildVisibilityTypeCache(t)
	var rows []resource.Resource
	for _, r := range byType["kms"] {
		if r.ID == demofixtures.KMSReplicaKeyID {
			rows = append(rows, r)
		}
	}
	if len(rows) != 1 {
		t.Fatalf("the demo replica key is not on the bench (%d rows)", len(rows))
	}
	res := unit.Wave2Result(t, "kms", demo.NewServiceClients(), rows)
	if t1aHas(res.Findings[demofixtures.KMSReplicaKeyID], "kms.rotation-disabled") {
		t.Errorf("%s (replica): rotation-disabled stands; findings %v", demofixtures.KMSReplicaKeyID, t2Codes(res.Findings[demofixtures.KMSReplicaKeyID]))
	}
}

// Both enabled EventBridge rule states are live, so an enabled rule without
// targets is reported in either and a disabled one is not.
func TestT2Dev_EBRuleNoTargetsInBothEnabledStates(t *testing.T) {
	td := catalog.FindAny("eb-rule")
	if td == nil {
		t.Fatal("eb-rule is not in the catalog")
	}
	for raw, want := range map[string]bool{"ENABLED": true, "ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS": true, "DISABLED": false} {
		row := domain.Resource{ID: "rule", Fields: map[string]string{td.StatusKey(): raw}}
		if got := catalog.AppliesToRow("eb-rule", "eb-rule.no-targets", row); got != want {
			t.Errorf("eb-rule %s: no-targets applies = %v, want %v", raw, got, want)
		}
	}
}
