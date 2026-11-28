// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// apigw_issue_enrichment.go — Wave 2 issue enrichment for the apigw resource type.
package aws

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// apigw canonical FindingCodes.
const (
	apigwCodeNoDeployedStages  domain.FindingCode = "apigw.no-deployed-stages"
	apigwCodeStageConfigIssues domain.FindingCode = "apigw.stage-config-issues"

	// CodeAPIGWNoAuthorizerPublic — a REST API reachable from the internet
	// with no authorizer and no scoped resource policy.
	CodeAPIGWNoAuthorizerPublic domain.FindingCode = "apigw.no-authorizer-public"
	// CodeAPIGWNoAuthorizer — any other API with no authorizer: a private
	// REST API, or an HTTP API, which has no private endpoint type.
	CodeAPIGWNoAuthorizer domain.FindingCode = "apigw.no-authorizer"
	// CodeAPIGWNoAccessLogs — a stage with no access log settings. One code
	// covers the REST and the HTTP lane so the badge counts the gap once.
	CodeAPIGWNoAccessLogs domain.FindingCode = "apigw.no-access-logs"
	// CodeAPIGWTracingOff — a REST stage with tracing switched off.
	CodeAPIGWTracingOff domain.FindingCode = "apigw.tracing-off"
	// CodeAPIGWStageVariableSecret — a REST stage variable whose value scans
	// as a credential. The value never leaves this package.
	CodeAPIGWStageVariableSecret domain.FindingCode = "apigw.stage-variable-secret" //nolint:gosec // G101 false positive: a finding code, not a credential
)

// EnrichAPIGatewayStage calls GetStages per API (cap EnrichmentCap)
// and returns a Finding for any API with stage-level throttling or access-log issues.
//
// Findings (severity "~" — informational):
//   - Any stage with DefaultRouteSettings.ThrottlingBurstLimit == 0 OR ThrottlingRateLimit == 0
//     → "no throttling configured (DoS risk)"
//   - Any stage with AccessLogSettings == nil → "access logs disabled"
//
// Findings are aggregated per API (one finding per API, covering all stages).
// Skip if clients.APIGatewayV2 == nil. Per-API errors → truncated.
func EnrichAPIGatewayStage(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		FieldUpdates: make(map[string]map[string]string),
		TruncatedIDs: make(map[string]string),
	}
	if clients.APIGatewayV2 == nil {
		return result, nil
	}
	v1, hasV1 := clients.APIGatewayV1.(apigwV1API)
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	resources = capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool {
		return apigwKind(r) != apigwKindREST || hasV1
	}, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		apiID := r.ID
		if apiID == "" {
			return
		}
		// The REST lane is a different API with different calls. An account
		// with no REST client still gets its HTTP APIs enriched.
		if apigwKind(r) == apigwKindREST {
			row, rowFailures := apigwRESTRow(ctx, clients, v1, r, ownAccount)
			mergeRowResult(&mu, &result, &failures, row, rowFailures)
			return
		}
		row, rowFailures := apigwHTTPRow(ctx, clients, r)
		mergeRowResult(&mu, &result, &failures, row, rowFailures)
	})
	return result, errors.Join(loopErr, AggregateFailures("authorizers and stages", failures, n))
}

// apigwRESTRow evaluates one REST API into a result of its own, which the
// caller merges under its lock.
func apigwRESTRow(ctx context.Context, clients *ServiceClients, v1 apigwV1API, r resource.Resource, ownAccount string) (IssueEnricherResult, []Failure) {
	row := newRowResult()
	var rowFailures []Failure
	apigwRESTFindings(ctx, clients, v1, &row, &rowFailures, r, ownAccount)
	return row, rowFailures
}

// apigwHTTPRow evaluates one HTTP API — its stage walk and its authorizer
// verdict — into a result of its own; the caller holds its lock for the merge
// alone.
func apigwHTTPRow(ctx context.Context, clients *ServiceClients, r resource.Resource) (IssueEnricherResult, []Failure) {
	apiID := r.ID
	row := newRowResult()
	var rowFailures []Failure
	stages, complete, fetchErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigatewayv2types.Stage, *string, error) {
		out, err := clients.APIGatewayV2.GetStages(ctx, &apigatewayv2.GetStagesInput{
			ApiId:     aws.String(apiID),
			NextToken: token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	stagesTruncated := !complete

	stagesCountStr := resource.FormatExact(len(stages))
	if stagesTruncated || fetchErr != nil {
		stagesCountStr = resource.FormatTruncated(len(stages))
	}
	var rows []domain.DetailRow

	for _, stage := range stages {
		stageName := stage.StageName
		if stageName == nil {
			stageName = aws.String("(unnamed)")
		}

		if drs := stage.DefaultRouteSettings; drs != nil {
			noThrottle := (drs.ThrottlingBurstLimit != nil && *drs.ThrottlingBurstLimit == 0) ||
				(drs.ThrottlingRateLimit != nil && *drs.ThrottlingRateLimit == 0)
			if noThrottle {
				rows = append(rows, domain.DetailRow{
					Label: "Stage",
					Value: *stageName,
					Tier:  "~",
				})
				rows = append(rows, domain.DetailRow{
					Label: "Issue",
					Value: "no throttling configured (DoS risk)",
					Tier:  "~",
				})
			}
		}

		if stage.AccessLogSettings == nil {
			rows = append(rows, domain.DetailRow{
				Label: "Stage",
				Value: *stageName,
				Tier:  "~",
			})
			rows = append(rows, domain.DetailRow{
				Label: "Issue",
				Value: "access logs disabled",
				Tier:  "~",
			})
		}
	}

	// An API no stage deploys answers no caller, so whether its routes check
	// one is not an exposure; the no-deployed-stage finding says why.
	deployed := slices.ContainsFunc(stages, func(st apigatewayv2types.Stage) bool { return aws.ToString(st.DeploymentId) != "" })
	undeployed := !deployed && !stagesTruncated && fetchErr == nil
	if !undeployed {
		if authErr := apigwHTTPNoAuthorizer(ctx, clients, &row, r); authErr != nil {
			MarkSkipped(&row, r.ID, &rowFailures, authErr)
		}
	}

	// A stage-count page cap is not a coverage gap on the row: the "+" on
	// stages_count is where it is reported, and the authorizer verdict above
	// is a separate call the stage walk says nothing about. A walk refused on
	// a later page keeps the stages it read as a lower bound; one refused
	// before any stage answered has no count to state, and "0+" would only
	// restate the row's mark.
	if fetchErr != nil {
		MarkSkipped(&row, r.ID, &rowFailures, fetchErr)
	}
	if fetchErr == nil || len(stages) > 0 {
		row.FieldUpdates[apiID] = map[string]string{"stages_count": stagesCountStr}
	}

	if undeployed {
		apigwUndeployed(&row, r)
		return row, rowFailures
	}
	if len(rows) == 0 {
		return row, rowFailures
	}
	setWave2Finding(&row, apiID, apigwCodeStageConfigIssues, rows)
	return row, rowFailures
}

// apigwUndeployed raises the no-deployed-stage finding: no stage of the API
// carries a DeploymentId, so none serves a call. The phrase says there are
// none; the row says what kind of API is sitting undeployed, which the phrase
// cannot.
func apigwUndeployed(result *IssueEnricherResult, r resource.Resource) {
	setWave2Finding(result, r.ID, apigwCodeNoDeployedStages, []domain.DetailRow{{
		Label: "Protocol",
		Value: strings.ToLower(apigwKind(r)),
		Tier:  "~",
	}})
}

// apigwV1API is the set of REST calls the enricher needs. It is reached by
// type assertion off clients.APIGatewayV1 rather than by widening
// APIGatewayV1API, so a client or fake that implements only the aggregate
// still satisfies it. Same shape as CodeArtifactListPackagesAPI.
type apigwV1API interface {
	APIGatewayV1GetResourcesAPI
	APIGatewayV1GetStagesAPI
}

// apigwRESTFindings evaluates the REST posture rules for one API. A call that
// failed or a resource policy that does not parse marks the row rather than
// reporting an API whose posture it could not read as clean, and leaves the
// rules the other call answers running.
func apigwRESTFindings(ctx context.Context, clients *ServiceClients, api apigwV1API, result *IssueEnricherResult, failures *[]Failure, r resource.Resource, ownAccount string) {
	apiID := r.ID
	emit := func(code domain.FindingCode, rows ...domain.DetailRow) {
		setWave2Finding(result, apiID, code, rows)
	}
	stages, err := api.GetStages(ctx, &apigateway.GetStagesInput{RestApiId: aws.String(apiID)})
	// An API no stage deploys answers no caller, so whether its methods check
	// one is not an exposure; the no-deployed-stage finding says why.
	undeployed := err == nil && !slices.ContainsFunc(stages.Item, func(st apigwtypes.Stage) bool { return aws.ToString(st.DeploymentId) != "" })
	if !undeployed {
		apigwRESTAuthorizerFindings(ctx, clients, api, result, failures, r, ownAccount)
	}
	if err != nil {
		MarkSkipped(result, apiID, failures, err)
		return
	}
	if undeployed {
		apigwUndeployed(result, r)
	}
	for _, st := range stages.Item {
		name := aws.ToString(st.StageName)
		if st.AccessLogSettings == nil {
			emit(CodeAPIGWNoAccessLogs,
				domain.DetailRow{Label: "Stage", Value: name, Tier: "~"})
		}
		if !st.TracingEnabled {
			emit(CodeAPIGWTracingOff,
				domain.DetailRow{Label: "Stage", Value: name, Tier: "~"})
		}
		// Rows carry where and what kind, never the value itself.
		if rows := secretScanRows(st.Variables); len(rows) > 0 {
			emit(CodeAPIGWStageVariableSecret,
				append([]domain.DetailRow{{Label: "Stage", Value: name, Tier: "!"}}, rows...)...)
		}
	}
}

// apigwRESTAuthorizerFindings is the no-authorizer rule for one REST API. It
// stands apart from apigwRESTFindings so a refused method read can return
// early without taking the stage checks with it.
//
// The fact is each method's AuthorizationType: an authorizer the API defines
// guards only the methods that name it, so the count of authorizers says
// nothing about whether a caller is checked.
func apigwRESTAuthorizerFindings(ctx context.Context, clients *ServiceClients, api apigwV1API, result *IssueEnricherResult, failures *[]Failure, r resource.Resource, ownAccount string) {
	apiID := r.ID

	resources, complete, err := apigwRESTResources(ctx, clients, api, apiID)
	var opens []string
	for _, res := range resources {
		for verb, m := range res.ResourceMethods {
			if apigwOpenMethod(verb, aws.ToString(m.AuthorizationType)) {
				opens = append(opens, verb+" "+aws.ToString(res.Path))
			}
		}
	}
	open := len(opens) > 0
	if err == nil && !open && !complete {
		err = errPageCap
	}
	if err != nil {
		MarkSkipped(result, apiID, failures, err)
	}
	if !open {
		return
	}
	endpoint := strings.ToLower(r.Fields["endpoint"])
	// A method with no IAM authorization admits an anonymous caller only
	// when the resource policy grants it and no Deny matches, so the API is
	// scoped unless it has no policy or an unscoped wildcard grant survives.
	policy := readPolicy("apigw", apigwRESTPolicy(r), ownAccount, "")
	switch {
	case policy.markUnread(result, apiID):
	case !policy.absent() && !policy.public():
	case endpoint == "":
		// The endpoint type is unknown, and the common contract's nil rule
		// makes an unread field unknown rather than misconfigured.
	case endpoint == "private":
		setWave2Finding(result, apiID, CodeAPIGWNoAuthorizer, []domain.DetailRow{
			apigwOpenRow("Open methods", opens, "~"),
			{Label: "Endpoint", Value: endpoint, Tier: "~"}})
	default:
		setWave2Finding(result, apiID, CodeAPIGWNoAuthorizerPublic, []domain.DetailRow{
			apigwOpenRow("Open methods", opens, "!"),
			{Label: "Endpoint", Value: endpoint, Tier: "!"}})
	}
}

func apigwOpenRow(label string, opens []string, tier string) domain.DetailRow {
	return domain.DetailRow{Label: label, Value: strconv.Itoa(len(opens)), Tier: tier}
}

// apigwOpenMethod reports whether a method or route lets a caller through
// unchecked: its AuthorizationType is NONE. An OPTIONS method answers the
// browser's CORS preflight, which carries no credentials, so it is not an
// open method.
func apigwOpenMethod(verb, authorizationType string) bool {
	return authorizationType == "NONE" && !strings.EqualFold(verb, "OPTIONS")
}

// apigwRESTPolicy returns the API's resource policy document, which the
// fetcher keeps on the retained RestApi.
func apigwRESTPolicy(r resource.Resource) string {
	api, ok := r.RawStruct.(apigwtypes.RestApi)
	if !ok {
		return ""
	}
	return aws.ToString(api.Policy)
}

// apigwHTTPNoAuthorizer evaluates the no-authorizer rule for one HTTP or
// WebSocket (v2) API from its routes' AuthorizationType. There is no private
// endpoint type on v2, so an unauthorized one is always the warn code. A
// WebSocket API authorizes the connection on its $connect route alone, and a
// WebSocket API without one accepts every connection.
func apigwHTTPNoAuthorizer(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, r resource.Resource) error {
	api, ok := clients.APIGatewayV2.(APIGatewayV2GetRoutesAPI)
	if !ok {
		return errClientMissing
	}
	// GetRoutes pages by NextToken
	// (https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-routes.html).
	routes, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigatewayv2types.Route, *string, error) {
		out, err := api.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{ApiId: aws.String(r.ID), NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	websocket := apigwKind(r) == "WEBSOCKET"
	connect := false
	var opens []string
	for _, rt := range routes {
		key := aws.ToString(rt.RouteKey)
		if websocket && key != "$connect" {
			continue
		}
		connect = connect || key == "$connect"
		if verb, _, _ := strings.Cut(key, " "); apigwOpenMethod(verb, string(rt.AuthorizationType)) {
			opens = append(opens, key)
		}
	}
	switch {
	case len(opens) > 0:
	case err != nil:
		return err
	case !complete:
		return errPageCap
	case !websocket || connect:
		return nil
	default:
		opens = []string{"$connect"}
	}
	setWave2Finding(result, r.ID, CodeAPIGWNoAuthorizer, []domain.DetailRow{apigwOpenRow("Open routes", opens, tierOf(CodeAPIGWNoAuthorizer))})
	return err
}
