package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("lambda", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLambdaFunctions(ctx, c.Lambda)
	})
}

// FetchLambdaFunctions calls the Lambda ListFunctions API and converts the
// response into a slice of generic Resource structs.
func FetchLambdaFunctions(ctx context.Context, api LambdaListFunctionsAPI) ([]resource.Resource, error) {
	output, err := api.ListFunctions(ctx, &lambda.ListFunctionsInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, fn := range output.Functions {
		functionName := ""
		if fn.FunctionName != nil {
			functionName = *fn.FunctionName
		}

		runtime := string(fn.Runtime)

		memory := ""
		if fn.MemorySize != nil {
			memory = fmt.Sprintf("%d", *fn.MemorySize)
		}

		timeout := ""
		if fn.Timeout != nil {
			timeout = fmt.Sprintf("%d", *fn.Timeout)
		}

		handler := ""
		if fn.Handler != nil {
			handler = *fn.Handler
		}

		lastModified := ""
		if fn.LastModified != nil {
			lastModified = *fn.LastModified
		}

		codeSize := ""
		if fn.CodeSize != 0 {
			codeSize = fmt.Sprintf("%d", fn.CodeSize)
		}

		detail := map[string]string{
			"Function Name": functionName,
			"Runtime":       runtime,
			"Memory (MB)":   memory,
			"Timeout (s)":   timeout,
			"Handler":       handler,
			"Last Modified": lastModified,
			"Code Size":     codeSize,
		}

		if fn.FunctionArn != nil {
			detail["ARN"] = *fn.FunctionArn
		}

		if fn.Role != nil {
			detail["Role"] = *fn.Role
		}

		if fn.Description != nil {
			detail["Description"] = *fn.Description
		}

		detail["Package Type"] = string(fn.PackageType)
		detail["Architecture"] = ""
		if len(fn.Architectures) > 0 {
			detail["Architecture"] = string(fn.Architectures[0])
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(fn, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     functionName,
			Name:   functionName,
			Status: runtime,
			Fields: map[string]string{
				"function_name": functionName,
				"runtime":       runtime,
				"memory":        memory,
				"timeout":       timeout,
				"handler":       handler,
				"last_modified": lastModified,
				"code_size":     codeSize,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  fn,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
