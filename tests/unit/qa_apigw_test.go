package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// API Gateway V2 fetcher tests
// ---------------------------------------------------------------------------

func TestFetchAPIGateways_ParsesMultiple(t *testing.T) {
	now := time.Now()
	mock := &mockAPIGatewayV2Client{
		output: &apigatewayv2.GetApisOutput{
			Items: []apigwtypes.Api{
				{
					ApiId:        aws.String("abc123def4"),
					Name:         aws.String("my-http-api"),
					ProtocolType: apigwtypes.ProtocolTypeHttp,
					ApiEndpoint:  aws.String("https://abc123def4.execute-api.us-east-1.amazonaws.com"),
					Description:  aws.String("Production HTTP API"),
					CreatedDate:  &now,
				},
				{
					ApiId:        aws.String("xyz789ghi0"),
					Name:         aws.String("my-websocket-api"),
					ProtocolType: apigwtypes.ProtocolTypeWebsocket,
					ApiEndpoint:  aws.String("wss://xyz789ghi0.execute-api.us-east-1.amazonaws.com"),
					Description:  aws.String(""),
					CreatedDate:  &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchAPIGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first API
	r0 := resources[0]
	if r0.ID != "abc123def4" {
		t.Errorf("resource[0].ID: expected %q, got %q", "abc123def4", r0.ID)
	}
	if r0.Name != "my-http-api" {
		t.Errorf("resource[0].Name: expected %q, got %q", "my-http-api", r0.Name)
	}

	// Verify required fields
	requiredFields := []string{"api_id", "name", "protocol", "endpoint", "description"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	if r0.Fields["protocol"] != "HTTP" {
		t.Errorf("resource[0].Fields[\"protocol\"]: expected %q, got %q", "HTTP", r0.Fields["protocol"])
	}
	if r0.Fields["endpoint"] != "https://abc123def4.execute-api.us-east-1.amazonaws.com" {
		t.Errorf("resource[0].Fields[\"endpoint\"]: expected endpoint, got %q", r0.Fields["endpoint"])
	}

	// Verify second API (WebSocket)
	r1 := resources[1]
	if r1.Fields["protocol"] != "WEBSOCKET" {
		t.Errorf("resource[1].Fields[\"protocol\"]: expected %q, got %q", "WEBSOCKET", r1.Fields["protocol"])
	}
}

func TestFetchAPIGateways_RawStructPopulated(t *testing.T) {
	now := time.Now()
	mock := &mockAPIGatewayV2Client{
		output: &apigatewayv2.GetApisOutput{
			Items: []apigwtypes.Api{
				{
					ApiId:        aws.String("raw123"),
					Name:         aws.String("raw-api"),
					ProtocolType: apigwtypes.ProtocolTypeHttp,
					CreatedDate:  &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchAPIGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}
	api, ok := r.RawStruct.(apigwtypes.Api)
	if !ok {
		t.Fatalf("RawStruct should be apigwtypes.Api, got %T", r.RawStruct)
	}
	if api.ApiId == nil || *api.ApiId != "raw123" {
		t.Errorf("RawStruct.ApiId: expected %q", "raw123")
	}
}

func TestFetchAPIGateways_ErrorResponse(t *testing.T) {
	mock := &mockAPIGatewayV2Client{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchAPIGateways(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchAPIGateways_EmptyResponse(t *testing.T) {
	mock := &mockAPIGatewayV2Client{
		output: &apigatewayv2.GetApisOutput{
			Items: []apigwtypes.Api{},
		},
	}

	resources, err := awsclient.FetchAPIGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
