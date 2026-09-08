package unit

// wipfix_apigw_cursor_test.go — row 8: the merged API Gateway cursor must be
// able to say a lane FINISHED.
//
// FetchAPIGatewaysPageMerged walks two independent lanes: APIGateway V1's
// Position-based REST listing (capped per call at apigwV1PageCap) and
// APIGateway V2's NextToken listing. When V2 drains on the first call while
// V1 is still mid-walk, the encoded cursor carries a V1 position and an EMPTY
// V2 token — and an empty V2 token is also what "first page" means, so the
// next continuation restarts V2 from page one. Its rows are fetched twice and
// enter the accumulated result a second time.

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cursorV1Fake answers GetRestApis with one REST API per page and never
// reaches its terminal Position, so every call's walk is cut by apigwV1PageCap
// and hands back a resume position.
type cursorV1Fake struct {
	mu    sync.Mutex
	calls int
}

func (f *cursorV1Fake) GetRestApis(
	_ context.Context,
	_ *apigateway.GetRestApisInput,
	_ ...func(*apigateway.Options),
) (*apigateway.GetRestApisOutput, error) {
	f.mu.Lock()
	f.calls++
	n := f.calls
	f.mu.Unlock()
	return &apigateway.GetRestApisOutput{
		Items: []apigatewaytypes.RestApi{{
			Id:   aws.String("rest-" + strconv.Itoa(n)),
			Name: aws.String("acme-rest-" + strconv.Itoa(n)),
		}},
		Position: aws.String("v1-pos-" + strconv.Itoa(n)),
	}, nil
}

var _ awsclient.APIGatewayV1API = (*cursorV1Fake)(nil)

// cursorV2Fake serves ONE terminal page of HTTP APIs: no NextToken, so the V2
// lane is finished the moment it is called once. Every extra call is a re-walk
// of a lane that had already reported exhaustion.
type cursorV2Fake struct {
	awsclient.APIGatewayV2API
	mu    sync.Mutex
	calls int
}

func (f *cursorV2Fake) GetApis(
	_ context.Context,
	_ *apigatewayv2.GetApisInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetApisOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return &apigatewayv2.GetApisOutput{Items: []apigwv2types.Api{{
		ApiId:        aws.String("http-only"),
		Name:         aws.String("acme-http-only"),
		ProtocolType: apigwv2types.ProtocolTypeHttp,
		ApiEndpoint:  aws.String("https://http-only.execute-api.us-east-1.amazonaws.com"),
	}}}, nil
}

func countAPIGWIDs(seen map[string]int, res []resource.Resource) {
	for _, r := range res {
		seen[r.ID]++
	}
}

// TestFetchAPIGatewaysPageMerged_FinishedV2LaneIsNotRewalked pins row 8: after
// the V2 lane reports exhaustion, a continuation driven by the still-unfinished
// V1 lane must not fetch V2 again, and must not deliver its rows twice.
func TestFetchAPIGatewaysPageMerged_FinishedV2LaneIsNotRewalked(t *testing.T) {
	v1 := &cursorV1Fake{}
	v2 := &cursorV2Fake{}
	clients := &awsclient.ServiceClients{APIGatewayV1: v1, APIGatewayV2: v2}

	first, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if first.Pagination == nil || !first.Pagination.IsTruncated {
		t.Fatalf("first page must be truncated — the V1 lane is still mid-walk")
	}
	if v2.calls != 1 {
		t.Fatalf("GetApis called %d times on the first page, want 1", v2.calls)
	}

	second, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, first.Pagination.NextToken)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if second.Pagination == nil || !second.Pagination.IsTruncated {
		t.Fatalf("second page must still be truncated — the V1 lane is still mid-walk")
	}
	if !strings.Contains(second.Pagination.NextToken, "v1") {
		t.Errorf("continuation token %q must still carry the unfinished V1 position", second.Pagination.NextToken)
	}

	third, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, second.Pagination.NextToken)
	if err != nil {
		t.Fatalf("third page: %v", err)
	}

	if v2.calls != 1 {
		t.Errorf("GetApis called %d times across three continuations, want 1 — the V2 lane reported exhaustion on the first call and the cursor must say so", v2.calls)
	}
	seen := map[string]int{}
	countAPIGWIDs(seen, first.Resources)
	countAPIGWIDs(seen, second.Resources)
	countAPIGWIDs(seen, third.Resources)
	if n := seen["http-only"]; n != 1 {
		t.Errorf("the HTTP API %q was delivered %d times across three continuations, want 1 — a re-walked lane's rows enter the accumulated result again", "http-only", n)
	}
}
