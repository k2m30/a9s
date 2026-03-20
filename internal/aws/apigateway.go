package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("apigw", []string{"api_id", "name", "protocol", "endpoint", "description"})
	resource.Register("apigw", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAPIGateways(ctx, c.APIGatewayV2)
	})
}

// FetchAPIGateways calls the API Gateway V2 GetApis API and converts
// the response into a slice of generic Resource structs.
func FetchAPIGateways(ctx context.Context, api APIGatewayV2GetApisAPI) ([]resource.Resource, error) {
	output, err := api.GetApis(ctx, &apigatewayv2.GetApisInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, item := range output.Items {
		apiID := ""
		if item.ApiId != nil {
			apiID = *item.ApiId
		}

		name := ""
		if item.Name != nil {
			name = *item.Name
		}

		protocol := string(item.ProtocolType)

		endpoint := ""
		if item.ApiEndpoint != nil {
			endpoint = *item.ApiEndpoint
		}

		description := ""
		if item.Description != nil {
			description = *item.Description
		}

		createdDate := ""
		if item.CreatedDate != nil {
			createdDate = item.CreatedDate.Format("2006-01-02 15:04:05")
		}

		// Build DetailData
		detail := map[string]string{
			"API ID":       apiID,
			"Name":         name,
			"Protocol":     protocol,
			"Endpoint":     endpoint,
			"Description":  description,
			"Created Date": createdDate,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(item, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     apiID,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"api_id":      apiID,
				"name":        name,
				"protocol":    protocol,
				"endpoint":    endpoint,
				"description": description,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  item,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
