package unit_test

import (
	"context"
	"slices"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// One API's name is the prefix of another's, and a log group belongs to the
// API whose name it carries whole.
func TestRelated_APIGW_Logs_NameIsNotAPrefixMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "/aws/apigateway/orders", Fields: map[string]string{}},
			{ID: "/aws/apigateway/orders/access", Fields: map[string]string{}},
			{ID: "/aws/apigateway/orders-v2", Fields: map[string]string{}},
		}},
	}
	src := resource.Resource{
		ID:     "abc123",
		Name:   "orders",
		Fields: map[string]string{"api_id": "abc123", "name": "orders"},
	}

	result := apigwCheckerByTarget(t, "logs")(context.Background(), &awsclient.ServiceClients{}, src, cache)

	want := []string{"/aws/apigateway/orders", "/aws/apigateway/orders/access"}
	got := result.ResourceIDs()
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("ResourceIDs = %v, want %v", got, want)
	}
}
