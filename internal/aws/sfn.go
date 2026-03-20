package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("sfn", []string{"name", "type", "arn", "creation_date"})
	resource.Register("sfn", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchStepFunctions(ctx, c.SFN)
	})
}

// FetchStepFunctions calls the SFN ListStateMachines API and converts
// the response into a slice of generic Resource structs.
func FetchStepFunctions(ctx context.Context, api SFNListStateMachinesAPI) ([]resource.Resource, error) {
	output, err := api.ListStateMachines(ctx, &sfn.ListStateMachinesInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, sm := range output.StateMachines {
		name := ""
		if sm.Name != nil {
			name = *sm.Name
		}

		arn := ""
		if sm.StateMachineArn != nil {
			arn = *sm.StateMachineArn
		}

		smType := string(sm.Type)

		creationDate := ""
		if sm.CreationDate != nil {
			creationDate = sm.CreationDate.Format("2006-01-02 15:04:05")
		}

		// Build DetailData
		detail := map[string]string{
			"Name":          name,
			"ARN":           arn,
			"Type":          smType,
			"Creation Date": creationDate,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(sm, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":          name,
				"arn":           arn,
				"type":          smType,
				"creation_date": creationDate,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  sm,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
