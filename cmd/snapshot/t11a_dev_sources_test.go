package main

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	v2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

func TestT11aDev_CaptureEBRule_ReadsEveryBusPageAndKeepsRulesWhenABusFails(t *testing.T) {
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "ListEventBuses":
			if in.(*eventbridge.ListEventBusesInput).NextToken == nil {
				return &eventbridge.ListEventBusesOutput{EventBuses: []ebtypes.EventBus{{Name: aws.String("default")}}, NextToken: aws.String("p2")}, nil
			}
			return &eventbridge.ListEventBusesOutput{EventBuses: []ebtypes.EventBus{{Name: aws.String("partner")}, {Name: aws.String("broken")}}}, nil
		case "ListRules":
			switch aws.ToString(in.(*eventbridge.ListRulesInput).EventBusName) {
			case "default":
				return &eventbridge.ListRulesOutput{Rules: []ebtypes.Rule{{Name: aws.String("nightly"), EventBusName: aws.String("default")}}}, nil
			case "partner":
				return &eventbridge.ListRulesOutput{Rules: []ebtypes.Rule{{Name: aws.String("orders"), EventBusName: aws.String("partner")}}}, nil
			}
			return nil, fmt.Errorf("denied")
		case "ListTargetsByRule":
			return &eventbridge.ListTargetsByRuleOutput{}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureEBRule(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureEBRule: %v", err)
	}
	d := got.(ebRuleData)
	var names []string
	for _, r := range d.Rules {
		names = append(names, r.EventBusName+"/"+r.Name)
	}
	if !slices.Equal(names, []string{"default/nightly", "partner/orders"}) {
		t.Errorf("rules = %v", names)
	}
	if len(d.PartialErrors) != 1 {
		t.Errorf("partial errors = %v, want the broken bus only", d.PartialErrors)
	}
}

func TestT11aDev_CaptureAPIGW_KeepsOneSourceWhenTheOtherFails(t *testing.T) {
	cfg := answeringConfig(func(_, op string, in any) (any, error) {
		switch op {
		case "GetRestApis":
			return nil, fmt.Errorf("denied")
		case "GetApis":
			return &apigatewayv2.GetApisOutput{Items: []v2types.Api{{ApiId: aws.String("a1"), Name: aws.String("http")}}}, nil
		case "GetStages":
			return &apigatewayv2.GetStagesOutput{}, nil
		}
		return nil, fmt.Errorf("unexpected %s", op)
	})

	got, err := captureAPIGW(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureAPIGW: %v", err)
	}
	d := got.(apigwData)
	if len(d.V2APIs) != 1 || len(d.PartialErrors) != 1 {
		t.Errorf("apigw = %+v", d)
	}
}

func TestT11aDev_SettleSources_AllFailingIsAnError(t *testing.T) {
	_, err := settleSources("x", sourceResult{"a", fmt.Errorf("e1")}, sourceResult{"b", fmt.Errorf("e2")})
	if err == nil {
		t.Fatal("every source failing must be an error")
	}
}
