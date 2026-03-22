package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/k2m30/a9s/v3/internal/resource"
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
		return nil, fmt.Errorf("fetching EventBridge rules: %w", err)
	}

	var resources []resource.Resource

	for _, rule := range output.Rules {
		name := ""
		if rule.Name != nil {
			name = *rule.Name
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
			RawStruct:  rule,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
