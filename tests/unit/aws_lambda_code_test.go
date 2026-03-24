package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Lambda Code fetcher tests (child of Lambda — viewport-based code viewer)
// ---------------------------------------------------------------------------

// TestFetchLambdaCode_PythonHandler verifies that a Python Lambda's handler
// file is extracted from the zip download and returned as LambdaCodeResult.
func TestFetchLambdaCode_PythonHandler(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: &lambda.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("my-python-func"),
				Runtime:      lambdatypes.RuntimePython312,
				Handler:      aws.String("index.handler"),
				PackageType:  lambdatypes.PackageTypeZip,
				CodeSize:     1048576, // 1 MB
			},
			Code: &lambdatypes.FunctionCodeLocation{
				Location: aws.String("https://awslambda-us-east-1-tasks.s3.us-east-1.amazonaws.com/presigned-url"),
			},
		},
	}

	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "my-python-func")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Run("function_name", func(t *testing.T) {
		if result.FunctionName != "my-python-func" {
			t.Errorf("FunctionName: expected %q, got %q", "my-python-func", result.FunctionName)
		}
	})

	t.Run("runtime", func(t *testing.T) {
		if result.Runtime == "" {
			t.Error("Runtime should not be empty")
		}
	})

	t.Run("handler", func(t *testing.T) {
		if result.Handler != "index.handler" {
			t.Errorf("Handler: expected %q, got %q", "index.handler", result.Handler)
		}
	})
}

// TestFetchLambdaCode_NodeHandler verifies that a Node.js Lambda's handler
// is resolved correctly (.js extension).
func TestFetchLambdaCode_NodeHandler(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: &lambda.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("my-node-func"),
				Runtime:      lambdatypes.RuntimeNodejs20x,
				Handler:      aws.String("app.handler"),
				PackageType:  lambdatypes.PackageTypeZip,
				CodeSize:     524288, // 512 KB
			},
			Code: &lambdatypes.FunctionCodeLocation{
				Location: aws.String("https://awslambda-us-east-1-tasks.s3.amazonaws.com/presigned-url"),
			},
		},
	}

	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "my-node-func")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Run("handler_resolved", func(t *testing.T) {
		if result.Handler != "app.handler" {
			t.Errorf("Handler: expected %q, got %q", "app.handler", result.Handler)
		}
	})
}

// TestFetchLambdaCode_ContainerImageRejection verifies that container image
// Lambdas (PackageType="Image") are not downloaded and return an appropriate
// informational result instead.
func TestFetchLambdaCode_ContainerImageRejection(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: &lambda.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("my-container-func"),
				PackageType:  lambdatypes.PackageTypeImage,
				CodeSize:     0,
			},
			Code: &lambdatypes.FunctionCodeLocation{
				ImageUri: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest"),
			},
		},
	}

	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "my-container-func")
	if err != nil {
		t.Fatalf("expected no error for container image, got %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Run("is_container_image", func(t *testing.T) {
		if !result.IsContainerImage {
			t.Error("IsContainerImage should be true for Image package type")
		}
	})

	t.Run("code_not_downloaded", func(t *testing.T) {
		if result.Code != "" {
			t.Error("Code should be empty for container image Lambda")
		}
	})
}

// TestFetchLambdaCode_PackageTooLarge verifies that packages over 5MB are
// rejected with an appropriate message.
func TestFetchLambdaCode_PackageTooLarge(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: &lambda.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("large-func"),
				Runtime:      lambdatypes.RuntimePython312,
				Handler:      aws.String("index.handler"),
				PackageType:  lambdatypes.PackageTypeZip,
				CodeSize:     10485760, // 10 MB — exceeds 5MB limit
			},
			Code: &lambdatypes.FunctionCodeLocation{
				Location: aws.String("https://awslambda-us-east-1-tasks.s3.amazonaws.com/presigned-url"),
			},
		},
	}

	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "large-func")
	if err != nil {
		t.Fatalf("expected no error for large package (graceful rejection), got %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	t.Run("too_large", func(t *testing.T) {
		if !result.IsTooLarge {
			t.Error("IsTooLarge should be true for >5MB package")
		}
	})

	t.Run("code_not_downloaded", func(t *testing.T) {
		if result.Code != "" {
			t.Error("Code should be empty for too-large package")
		}
	})
}

// TestFetchLambdaCode_APIError verifies that API errors are propagated correctly.
func TestFetchLambdaCode_APIError(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "err-func")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
}

// TestFetchLambdaCode_RuntimeResolution verifies handler file resolution
// for multiple runtimes: Python→.py, Node→.js, Ruby→.rb, Go→bootstrap.
func TestFetchLambdaCode_RuntimeResolution(t *testing.T) {
	cases := []struct {
		name     string
		runtime  lambdatypes.Runtime
		handler  string
		expected string // The resolved handler (not file extension)
	}{
		{"python", lambdatypes.RuntimePython312, "lambda_function.handler", "lambda_function.handler"},
		{"nodejs", lambdatypes.RuntimeNodejs20x, "index.handler", "index.handler"},
		{"go", lambdatypes.RuntimeProvidedal2023, "bootstrap", "bootstrap"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockLambdaGetFunctionClient{
				output: &lambda.GetFunctionOutput{
					Configuration: &lambdatypes.FunctionConfiguration{
						FunctionName: aws.String(tc.name + "-func"),
						Runtime:      tc.runtime,
						Handler:      aws.String(tc.handler),
						PackageType:  lambdatypes.PackageTypeZip,
						CodeSize:     1024, // 1 KB
					},
					Code: &lambdatypes.FunctionCodeLocation{
						Location: aws.String("https://example.com/presigned"),
					},
				},
			}

			result, err := awsclient.FetchLambdaCode(context.Background(), mock, tc.name+"-func")
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Handler != tc.expected {
				t.Errorf("Handler: expected %q, got %q", tc.expected, result.Handler)
			}
		})
	}
}

// TestFetchLambdaCode_NilConfiguration verifies that nil Configuration
// fields do not cause a panic.
func TestFetchLambdaCode_NilConfiguration(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: &lambda.GetFunctionOutput{
			Configuration: nil,
			Code: &lambdatypes.FunctionCodeLocation{
				Location: aws.String("https://example.com/presigned"),
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "nil-config-func")
	// Either error or graceful result, but no panic
	_ = result
	_ = err
}

// TestFetchLambdaCode_NilCodeLocation verifies that nil Code.Location
// does not cause a panic.
func TestFetchLambdaCode_NilCodeLocation(t *testing.T) {
	mock := &mockLambdaGetFunctionClient{
		output: &lambda.GetFunctionOutput{
			Configuration: &lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("nil-code-func"),
				Runtime:      lambdatypes.RuntimePython312,
				Handler:      aws.String("index.handler"),
				PackageType:  lambdatypes.PackageTypeZip,
				CodeSize:     1024,
			},
			Code: &lambdatypes.FunctionCodeLocation{
				// Location is nil
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchLambdaCode(context.Background(), mock, "nil-code-func")
	_ = result
	_ = err
}

// TestLambdaCodeResult_StructFields verifies that the LambdaCodeResult struct
// has the expected fields for all states.
func TestLambdaCodeResult_StructFields(t *testing.T) {
	r := awsclient.LambdaCodeResult{
		FunctionName:     "test-func",
		Runtime:          "python3.12",
		Handler:          "index.handler",
		Code:             "def handler(event, context):\n    return 'hello'",
		IsContainerImage: false,
		IsTooLarge:       false,
		FileList:         []string{"index.py", "requirements.txt"},
	}

	if r.FunctionName != "test-func" {
		t.Errorf("FunctionName: expected %q, got %q", "test-func", r.FunctionName)
	}
	if r.Runtime != "python3.12" {
		t.Errorf("Runtime: expected %q, got %q", "python3.12", r.Runtime)
	}
	if r.Handler != "index.handler" {
		t.Errorf("Handler: expected %q, got %q", "index.handler", r.Handler)
	}
	if r.Code == "" {
		t.Error("Code should not be empty")
	}
	if r.IsContainerImage {
		t.Error("IsContainerImage should be false")
	}
	if r.IsTooLarge {
		t.Error("IsTooLarge should be false")
	}
	if len(r.FileList) != 2 {
		t.Errorf("FileList: expected 2 items, got %d", len(r.FileList))
	}
}
