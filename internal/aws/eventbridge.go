package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eb-rule", []string{"name", "state", "event_bus", "schedule", "description"})
	resource.Register("eb-rule", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEventBridgeRules(ctx, c.EventBridge)
	})
}

// FetchEventBridgeRules calls the EventBridge ListRules API and converts
// the response into a slice of generic Resource structs.
func FetchEventBridgeRules(ctx context.Context, api EventBridgeListRulesAPI) ([]resource.Resource, error) {
	output, err := api.ListRules(ctx, &eventbridge.ListRulesInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, rule := range output.Rules {
		name := ""
		if rule.Name != nil {
			name = *rule.Name
		}

		arn := ""
		if rule.Arn != nil {
			arn = *rule.Arn
		}

		state := string(rule.State)

		description := ""
		if rule.Description != nil {
			description = *rule.Description
		}

		eventBus := ""
		if rule.EventBusName != nil {
			eventBus = *rule.EventBusName
		}

		schedule := ""
		if rule.ScheduleExpression != nil {
			schedule = *rule.ScheduleExpression
		}

		eventPattern := ""
		if rule.EventPattern != nil {
			eventPattern = *rule.EventPattern
		}

		managedBy := ""
		if rule.ManagedBy != nil {
			managedBy = *rule.ManagedBy
		}

		roleArn := ""
		if rule.RoleArn != nil {
			roleArn = *rule.RoleArn
		}

		// Build DetailData
		detail := map[string]string{
			"Name":                name,
			"ARN":                 arn,
			"State":               state,
			"Description":         description,
			"Event Bus":           eventBus,
			"Schedule Expression": schedule,
			"Event Pattern":       eventPattern,
			"Managed By":          managedBy,
			"Role ARN":            roleArn,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(rule, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"name":        name,
				"state":       state,
				"description": description,
				"event_bus":   eventBus,
				"schedule":    schedule,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  rule,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
