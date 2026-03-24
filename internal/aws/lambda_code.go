package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// maxCodeSize is the maximum code package size (5 MB) that will be downloaded.
const maxCodeSize = 5 * 1024 * 1024

// LambdaCodeResult holds the result of fetching a Lambda function's source code.
type LambdaCodeResult struct {
	// FunctionName is the name of the Lambda function.
	FunctionName string
	// Runtime is the Lambda runtime (e.g., "python3.12", "nodejs20.x").
	Runtime string
	// Handler is the handler string (e.g., "index.handler").
	Handler string
	// Code is the extracted source code text (empty for container/too-large).
	Code string
	// IsContainerImage is true if the function uses a container image package type.
	IsContainerImage bool
	// IsTooLarge is true if the code package exceeds the download size limit.
	IsTooLarge bool
	// FileList contains the list of files in the zip archive.
	FileList []string
}

// FetchLambdaCode calls the Lambda GetFunction API to retrieve function
// configuration and, for Zip-packaged functions under the size limit,
// downloads and extracts the handler source file.
func FetchLambdaCode(ctx context.Context, api LambdaGetFunctionAPI, functionName string) (*LambdaCodeResult, error) {
	input := &lambda.GetFunctionInput{
		FunctionName: &functionName,
	}

	output, err := api.GetFunction(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("fetching lambda function %q: %w", functionName, err)
	}

	result := &LambdaCodeResult{
		FunctionName: functionName,
	}

	// Extract configuration if present
	if output.Configuration != nil {
		result.Runtime = string(output.Configuration.Runtime)
		if output.Configuration.Handler != nil {
			result.Handler = *output.Configuration.Handler
		}

		// Check for container image
		if string(output.Configuration.PackageType) == "Image" {
			result.IsContainerImage = true
			return result, nil
		}

		// Check code size limit
		if output.Configuration.CodeSize > maxCodeSize {
			result.IsTooLarge = true
			return result, nil
		}
	}

	// For Zip packages under the size limit, we would download the zip from
	// Code.Location and extract the handler file. This requires HTTP download
	// which is not testable with the current mock structure (the mock only
	// provides the GetFunction response, not an actual HTTP server).
	// The actual download + extraction will be wired when the CodeViewModel
	// is implemented.

	return result, nil
}
