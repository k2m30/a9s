// eb_rule_bus_walk_test.go — the EventBridge rule list spans every bus.
//
// ListRules answers for one bus and defaults to the default one, while a rule
// row is keyed by the bus it sits on. A fetcher that reads only the default
// bus therefore hands the screen a list that can never hold the row a pivot
// into a custom-bus rule names.
package unit_test

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ebBusFake answers ListRules for the named bus only, the way EventBridge
// does, and pages each bus's rules one at a time.
type ebBusFake struct {
	rulesByBus map[string][]ebtypes.Rule
	// noBusesNamed makes ListEventBuses answer without naming a bus.
	noBusesNamed bool
	busesAsked   int
	rulesAsked   []string
}

func (f *ebBusFake) ListEventBuses(_ context.Context, _ *eventbridge.ListEventBusesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
	f.busesAsked++
	var out eventbridge.ListEventBusesOutput
	if f.noBusesNamed {
		return &out, nil
	}
	for _, name := range slices.Sorted(maps.Keys(f.rulesByBus)) {
		out.EventBuses = append(out.EventBuses, ebtypes.EventBus{Name: aws.String(name)})
	}
	return &out, nil
}

func (f *ebBusFake) ListRules(_ context.Context, in *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	bus := "default"
	if in != nil && in.EventBusName != nil {
		bus = *in.EventBusName
	}
	f.rulesAsked = append(f.rulesAsked, bus)
	rules := f.rulesByBus[bus]
	if in != nil && in.NextToken != nil {
		return &eventbridge.ListRulesOutput{Rules: rules[1:]}, nil
	}
	if len(rules) > 1 {
		return &eventbridge.ListRulesOutput{Rules: rules[:1], NextToken: aws.String("page-2")}, nil
	}
	return &eventbridge.ListRulesOutput{Rules: rules}, nil
}

func ebRule(bus, name string) ebtypes.Rule {
	return ebtypes.Rule{
		Name:         aws.String(name),
		Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + bus + "/" + name),
		State:        ebtypes.RuleStateEnabled,
		EventBusName: aws.String(bus),
		EventPattern: aws.String(`{"source":["acme.orders"]}`),
	}
}

// TestEBRules_EveryBusIsListed: the rules of a bus beside the default one are
// on the list, each keyed by its own bus, and every page of each bus is read.
func TestEBRules_EveryBusIsListed(t *testing.T) {
	fake := &ebBusFake{rulesByBus: map[string][]ebtypes.Rule{
		"default":         {ebRule("default", "nightly-backup"), ebRule("default", "ec2-state-change")},
		"acme-orders-bus": {ebRule("acme-orders-bus", "order-placed-fanout")},
	}}

	out, err := awsclient.FetchEventBridgeRulesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchEventBridgeRulesPage: %v", err)
	}
	if fake.busesAsked != 1 {
		t.Errorf("ListEventBuses called %d times, want 1", fake.busesAsked)
	}
	for _, bus := range []string{"default", "acme-orders-bus"} {
		if !slices.Contains(fake.rulesAsked, bus) {
			t.Errorf("no ListRules call named the bus %q (called for %v)", bus, fake.rulesAsked)
		}
	}
	var ids []string
	for _, r := range out.Resources {
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	want := []string{"acme-orders-bus/order-placed-fanout", "default/ec2-state-change", "default/nightly-backup"}
	if !slices.Equal(ids, want) {
		t.Errorf("rows = %v, want %v", ids, want)
	}
	if out.Pagination.IsTruncated {
		t.Error("IsTruncated = true after a walk that read every page of every bus")
	}
}

// TestEBRules_CustomBusRuleIsOnTheDemoList: the demo carries a rule on a bus
// beside the default one, so the two-bus case is on a screen and not only in
// a fake.
func TestEBRules_CustomBusRuleIsOnTheDemoList(t *testing.T) {
	td := resource.FindResourceType("eb-rule")
	if td == nil {
		t.Fatal("eb-rule is not a registered type")
	}
	rows, ok := drainVisibilityFixtures(t, *td, demo.NewServiceClients())
	if !ok {
		t.Fatal("no eb-rule demo rows")
	}
	wantID := fixtures.EBCustomBus + "/" + fixtures.EBRuleOnCustomBus
	for _, r := range rows {
		if r.ID != wantID {
			continue
		}
		if r.Name != fixtures.EBRuleOnCustomBus {
			t.Errorf("row %q: Name = %q, want %q", r.ID, r.Name, fixtures.EBRuleOnCustomBus)
		}
		if got := r.Fields["event_bus"]; got != fixtures.EBCustomBus {
			t.Errorf("row %q: Fields[event_bus] = %q, want %q", r.ID, got, fixtures.EBCustomBus)
		}
		return
	}
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	t.Errorf("no demo eb-rule row %q; got %v", wantID, ids)
}

// TestEBRules_ReverseTargetPivotNamesTheBus: the pivot from a rule's target
// back to the rule opens the rule's own row. ListRuleNamesByTarget hands back
// a bare name for the bus it was asked about, and the demo's custom-bus rule
// is the row a default-bus reading would miss.
func TestEBRules_ReverseTargetPivotNamesTheBus(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	td := resource.FindResourceType("sqs")
	if td == nil {
		t.Fatal("sqs is not a registered type")
	}
	var def *resource.RelatedDef
	for i := range td.Related {
		if td.Related[i].TargetType == "eb-rule" {
			def = &td.Related[i]
		}
	}
	if def == nil || def.Checker == nil {
		t.Fatal("sqs has no eb-rule pivot")
	}

	wantID := fixtures.EBCustomBus + "/" + fixtures.EBRuleOnCustomBus
	for _, row := range byType["sqs"] {
		if row.ID != "order-processing-queue" {
			continue
		}
		got := def.Checker(context.Background(), clients, row, cache)
		if !slices.Contains(got.ResourceIDs(), wantID) {
			t.Errorf("sqs %q -> EventBridge Rules: IDs %v, want one of them %q", row.ID, got.ResourceIDs(), wantID)
		}
		return
	}
	t.Fatal("no demo sqs row \"order-processing-queue\"")
}

// TestEBRules_BusListNamingNothingFallsBackToTheDefaultBus: every account has
// a default bus, so an enumeration that names none of them has answered for
// nothing, and the rules ListRules gives without a bus are still the account's.
func TestEBRules_BusListNamingNothingFallsBackToTheDefaultBus(t *testing.T) {
	fake := &ebBusFake{rulesByBus: map[string][]ebtypes.Rule{
		"default": {ebRule("default", "nightly-backup")},
	}}
	fake.noBusesNamed = true

	out, err := awsclient.FetchEventBridgeRulesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchEventBridgeRulesPage: %v", err)
	}
	if len(out.Resources) != 1 || out.Resources[0].ID != "default/nightly-backup" {
		var ids []string
		for _, r := range out.Resources {
			ids = append(ids, r.ID)
		}
		t.Errorf("rows = %v, want [default/nightly-backup]", ids)
	}
}
