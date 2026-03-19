package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("ssm", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSSMParameters(ctx, c.SSM)
	})
	resource.RegisterFieldKeys("ssm", []string{"name", "type", "version", "last_modified", "description"})
}

// FetchSSMParameters calls the SSM DescribeParameters API and converts the
// response into a slice of generic Resource structs.
func FetchSSMParameters(ctx context.Context, api SSMDescribeParametersAPI) ([]resource.Resource, error) {
	output, err := api.DescribeParameters(ctx, &ssm.DescribeParametersInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, param := range output.Parameters {
		paramName := ""
		if param.Name != nil {
			paramName = *param.Name
		}

		paramType := string(param.Type)

		version := ""
		if param.Version != 0 {
			version = fmt.Sprintf("%d", param.Version)
		}

		lastModified := ""
		if param.LastModifiedDate != nil {
			lastModified = param.LastModifiedDate.Format("2006-01-02T15:04:05Z07:00")
		}

		description := ""
		if param.Description != nil {
			description = *param.Description
		}

		detail := map[string]string{
			"Name":          paramName,
			"Type":          paramType,
			"Version":       version,
			"Last Modified": lastModified,
			"Description":   description,
		}

		if param.LastModifiedUser != nil {
			detail["Last Modified By"] = *param.LastModifiedUser
		}

		if param.KeyId != nil {
			detail["KMS Key ID"] = *param.KeyId
		}

		detail["Tier"] = string(param.Tier)
		detail["Data Type"] = ""
		if param.DataType != nil {
			detail["Data Type"] = *param.DataType
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(param, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     paramName,
			Name:   paramName,
			Status: "",
			Fields: map[string]string{
				"name":          paramName,
				"type":          paramType,
				"version":       version,
				"last_modified": lastModified,
				"description":   description,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  param,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
