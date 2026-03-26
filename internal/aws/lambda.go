package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("lambda", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLambdaFunctions(ctx, c.Lambda)
	})
	resource.RegisterFieldKeys("lambda", []string{"function_name", "runtime", "memory", "timeout", "handler", "last_modified", "code_size", "log_group", "package_type"})
}

// FetchLambdaFunctions calls the Lambda ListFunctions API and converts the
// response into a slice of generic Resource structs.
func FetchLambdaFunctions(ctx context.Context, api LambdaListFunctionsAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var marker *string

	for {
		output, err := api.ListFunctions(ctx, &lambda.ListFunctionsInput{
			Marker: marker,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching Lambda functions: %w", err)
		}

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
				codeSize = formatBytes(fn.CodeSize)
			}

			logGroup := "/aws/lambda/" + functionName
			if fn.LoggingConfig != nil && fn.LoggingConfig.LogGroup != nil && *fn.LoggingConfig.LogGroup != "" {
				logGroup = *fn.LoggingConfig.LogGroup
			}

			packageType := string(fn.PackageType)

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
					"log_group":     logGroup,
					"package_type":  packageType,
				},
				RawStruct: fn,
			}

			resources = append(resources, r)
		}

		if output.NextMarker == nil {
			break
		}
		marker = output.NextMarker
	}

	return resources, nil
}
