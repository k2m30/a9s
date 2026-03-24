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
// Lambda fetcher tests
// ---------------------------------------------------------------------------

func TestFetchLambdaFunctions_ParsesMultipleFunctions(t *testing.T) {
	mock := &mockLambdaListFunctionsClient{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{
				{
					FunctionName: aws.String("my-go-function"),
					Runtime:      lambdatypes.RuntimeGo1x,
					MemorySize:   aws.Int32(128),
					Timeout:      aws.Int32(30),
					Handler:      aws.String("bootstrap"),
					LastModified: aws.String("2025-01-15T10:00:00.000+0000"),
					CodeSize:     5242880,
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-go-function"),
					Role:         aws.String("arn:aws:iam::123456789012:role/lambda-exec-role"),
					Description:  aws.String("My Go Lambda function"),
					PackageType:  lambdatypes.PackageTypeZip,
					Architectures: []lambdatypes.Architecture{
						lambdatypes.ArchitectureArm64,
					},
				},
				{
					FunctionName: aws.String("my-python-function"),
					Runtime:      lambdatypes.RuntimePython312,
					MemorySize:   aws.Int32(256),
					Timeout:      aws.Int32(60),
					Handler:      aws.String("index.handler"),
					LastModified: aws.String("2025-02-20T12:30:00.000+0000"),
					CodeSize:     1048576,
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-python-function"),
					Role:         aws.String("arn:aws:iam::123456789012:role/lambda-exec-role"),
					Description:  aws.String("My Python Lambda function"),
					PackageType:  lambdatypes.PackageTypeZip,
					Architectures: []lambdatypes.Architecture{
						lambdatypes.ArchitectureX8664,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"function_name", "runtime", "memory", "timeout", "handler", "last_modified", "code_size"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first function
	r0 := resources[0]
	if r0.ID != "my-go-function" {
		t.Errorf("resource[0].ID: expected %q, got %q", "my-go-function", r0.ID)
	}
	if r0.Name != "my-go-function" {
		t.Errorf("resource[0].Name: expected %q, got %q", "my-go-function", r0.Name)
	}
	if r0.Status != "go1.x" {
		t.Errorf("resource[0].Status: expected %q, got %q", "go1.x", r0.Status)
	}
	if r0.Fields["function_name"] != "my-go-function" {
		t.Errorf("resource[0].Fields[\"function_name\"]: expected %q, got %q", "my-go-function", r0.Fields["function_name"])
	}
	if r0.Fields["runtime"] != "go1.x" {
		t.Errorf("resource[0].Fields[\"runtime\"]: expected %q, got %q", "go1.x", r0.Fields["runtime"])
	}
	if r0.Fields["memory"] != "128" {
		t.Errorf("resource[0].Fields[\"memory\"]: expected %q, got %q", "128", r0.Fields["memory"])
	}
	if r0.Fields["timeout"] != "30" {
		t.Errorf("resource[0].Fields[\"timeout\"]: expected %q, got %q", "30", r0.Fields["timeout"])
	}
	if r0.Fields["handler"] != "bootstrap" {
		t.Errorf("resource[0].Fields[\"handler\"]: expected %q, got %q", "bootstrap", r0.Fields["handler"])
	}
	if r0.Fields["last_modified"] != "2025-01-15T10:00:00.000+0000" {
		t.Errorf("resource[0].Fields[\"last_modified\"]: expected %q, got %q", "2025-01-15T10:00:00.000+0000", r0.Fields["last_modified"])
	}
	if r0.Fields["code_size"] != "5 MB" {
		t.Errorf("resource[0].Fields[\"code_size\"]: expected %q, got %q", "5 MB", r0.Fields["code_size"])
	}

	// Verify second function
	r1 := resources[1]
	if r1.ID != "my-python-function" {
		t.Errorf("resource[1].ID: expected %q, got %q", "my-python-function", r1.ID)
	}
	if r1.Fields["runtime"] != "python3.12" {
		t.Errorf("resource[1].Fields[\"runtime\"]: expected %q, got %q", "python3.12", r1.Fields["runtime"])
	}
	if r1.Fields["memory"] != "256" {
		t.Errorf("resource[1].Fields[\"memory\"]: expected %q, got %q", "256", r1.Fields["memory"])
	}
}

func TestFetchLambdaFunctions_ErrorResponse(t *testing.T) {
	mock := &mockLambdaListFunctionsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchLambdaFunctions_EmptyResponse(t *testing.T) {
	mock := &mockLambdaListFunctionsClient{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{},
		},
	}

	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// Lambda parent fetcher: new Fields required for child views
// ---------------------------------------------------------------------------

// TestLambdaFetcherPopulatesLogGroup verifies that the Lambda fetcher
// populates the "log_group" field. When LoggingConfig.LogGroup is set, use
// that value; otherwise default to "/aws/lambda/{FunctionName}".
func TestLambdaFetcherPopulatesLogGroup(t *testing.T) {
	mock := &mockLambdaListFunctionsClient{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{
				{
					FunctionName: aws.String("default-log-func"),
					Runtime:      lambdatypes.RuntimePython312,
					PackageType:  lambdatypes.PackageTypeZip,
					MemorySize:   aws.Int32(128),
					Timeout:      aws.Int32(30),
					Handler:      aws.String("index.handler"),
					LastModified: aws.String("2025-01-01T00:00:00.000+0000"),
					CodeSize:     1024,
					// No LoggingConfig → default log group
				},
				{
					FunctionName: aws.String("custom-log-func"),
					Runtime:      lambdatypes.RuntimePython312,
					PackageType:  lambdatypes.PackageTypeZip,
					MemorySize:   aws.Int32(256),
					Timeout:      aws.Int32(60),
					Handler:      aws.String("app.handler"),
					LastModified: aws.String("2025-02-01T00:00:00.000+0000"),
					CodeSize:     2048,
					LoggingConfig: &lambdatypes.LoggingConfig{
						LogGroup: aws.String("/custom/log/group"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("default_log_group", func(t *testing.T) {
		logGroup, ok := resources[0].Fields["log_group"]
		if !ok {
			t.Fatal("Fields missing key 'log_group'")
		}
		expected := "/aws/lambda/default-log-func"
		if logGroup != expected {
			t.Errorf("Fields[log_group]: expected %q, got %q", expected, logGroup)
		}
	})

	t.Run("custom_log_group", func(t *testing.T) {
		logGroup, ok := resources[1].Fields["log_group"]
		if !ok {
			t.Fatal("Fields missing key 'log_group'")
		}
		expected := "/custom/log/group"
		if logGroup != expected {
			t.Errorf("Fields[log_group]: expected %q, got %q", expected, logGroup)
		}
	})
}

// TestLambdaFetcherPopulatesPackageType verifies that the Lambda fetcher
// populates the "package_type" field with "Zip" or "Image".
func TestLambdaFetcherPopulatesPackageType(t *testing.T) {
	mock := &mockLambdaListFunctionsClient{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{
				{
					FunctionName: aws.String("zip-func"),
					Runtime:      lambdatypes.RuntimePython312,
					PackageType:  lambdatypes.PackageTypeZip,
					MemorySize:   aws.Int32(128),
					Timeout:      aws.Int32(30),
					Handler:      aws.String("index.handler"),
					LastModified: aws.String("2025-01-01T00:00:00.000+0000"),
					CodeSize:     1024,
				},
				{
					FunctionName: aws.String("image-func"),
					PackageType:  lambdatypes.PackageTypeImage,
					MemorySize:   aws.Int32(256),
					Timeout:      aws.Int32(60),
					LastModified: aws.String("2025-02-01T00:00:00.000+0000"),
					CodeSize:     0,
				},
			},
		},
	}

	resources, err := awsclient.FetchLambdaFunctions(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("zip_package_type", func(t *testing.T) {
		pkgType, ok := resources[0].Fields["package_type"]
		if !ok {
			t.Fatal("Fields missing key 'package_type'")
		}
		if pkgType != "Zip" {
			t.Errorf("Fields[package_type]: expected %q, got %q", "Zip", pkgType)
		}
	})

	t.Run("image_package_type", func(t *testing.T) {
		pkgType, ok := resources[1].Fields["package_type"]
		if !ok {
			t.Fatal("Fields missing key 'package_type'")
		}
		if pkgType != "Image" {
			t.Errorf("Fields[package_type]: expected %q, got %q", "Image", pkgType)
		}
	})
}
