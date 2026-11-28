// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_issue_enrichment.go — Wave 2 issue enrichment for the lambda
// resource type: the function's lifecycle state, who is allowed to invoke
// it, and whether it answers unauthenticated HTTP.
package aws

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// lambda Wave-2 canonical FindingCodes.
const (
	lambdaCodePublicPolicy       domain.FindingCode = "lambda.public-policy"
	lambdaCodeCrossAccountPolicy domain.FindingCode = "lambda.cross-account-policy"
	lambdaCodeFunctionURLPublic  domain.FindingCode = "lambda.function-url-public"
)

// EnrichLambdaPosture asks, per function (capped at EnrichmentCap), GetPolicy
// for the resource policy, ListFunctionUrlConfigs for unauthenticated function
// URLs, and GetFunction for its lifecycle state — ListFunctions returns none
// of State, StateReasonCode or LastUpdateStatus. All three are read-only.
//
// A function with no resource policy answers GetPolicy with
// ResourceNotFoundException — that is the healthy answer, not a failure and
// not a coverage gap, so it is neither recorded in TruncatedIDs nor folded
// into the composite error. GetFunction answering the same code means the
// function is gone.
//
// Who may invoke the function and through which endpoint is one join of the
// policy and the URL reads (lambdaExposure), so a failed policy read decides
// neither exposure finding; a failed URL read leaves the policy's own grants
// standing. Only the read that failed leaves the row uninspected.
func EnrichLambdaPosture(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:         make(map[string][]domain.Finding),
		AttentionDetails: make(map[string]map[domain.FindingCode]domain.AttentionDetail),
		TruncatedIDs:     make(map[string]string),
		FieldUpdates:     make(map[string]map[string]string),
	}
	if clients.Lambda == nil {
		return result, nil
	}
	api, ok := clients.Lambda.(LambdaPostureAPI)
	if !ok {
		return result, nil
	}

	targets := capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool { return r.ID != "" }, resourceIDsOf)

	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	const op = "function state, policy and URL posture"
	var mu sync.Mutex
	var failures []Failure
	loopErr := ForEachRow(ctx, &result, resourceIDs(targets), EnrichmentParallelism, func(i int) {
		r := targets[i]
		// Every read runs here, on this row's own goroutine; the mutex below
		// only records results, so no row waits on another's round trip.
		policy, policyErr := lambdaPolicy(ctx, api, r.ID, "")
		urls, urlErr := lambdaFunctionURLs(ctx, api, r.ID)
		urlPolicies, qualifierErrs := lambdaURLPolicies(ctx, api, r.ID, urls)
		cfg, getErr := lambdaGetConfiguration(ctx, api, r.ID)

		mu.Lock()
		defer mu.Unlock()
		if isLambdaNoPolicy(getErr) {
			// Gone between the list call and this one: a race, not a failure
			// to log, and nothing was inspected either.
			markUninspected(&result, r.ID, "GetFunction")
			return
		}
		class, state := catalog.LifecycleNormal, ""
		if cfg != nil {
			state = string(cfg.State)
			class = LifecycleClassOf("lambda", state)
			result.FieldUpdates[r.ID] = map[string]string{
				"state":              string(cfg.State),
				"last_update_status": string(cfg.LastUpdateStatus),
			}
			if f, ok := StateFinding("lambda", string(cfg.State)); ok {
				setWave2Finding(&result, r.ID, f.Code, nil, domain.HumanizeStatusPhrase(string(cfg.State)))
			}
			if cfg.LastUpdateStatus == lambdatypes.LastUpdateStatusFailed {
				setWave2Finding(&result, r.ID, CodeLambdaLastUpdateFailed, nil)
			}
		}
		read := func(doc string) policyReading { return readPolicy("lambda", doc, ownAccount, r.Fields["arn"]) }
		if policyErr == nil {
			urlPolicies[""] = policy
		}
		if policyErr == nil || isLambdaNoPolicy(policyErr) {
			lambdaExposure(&result, r.ID, read, urlPolicies, urls, urlErr == nil, class, state)
		}
		// Each read that failed is recorded: two calls refused for two
		// different actions are two grants the operator needs.
		for _, err := range append([]error{getErr, policyErr, urlErr}, qualifierErrs...) {
			if err != nil && !isLambdaNoPolicy(err) {
				MarkSkipped(&result, r.ID, &failures, err)
			}
		}
	})

	err := errors.Join(loopErr, AggregateFailures(op, failures, len(targets)))
	return result, err
}

// lambdaGetConfiguration reads the function's configuration through
// GetFunction. An answer without one is an error, never an empty state.
func lambdaGetConfiguration(ctx context.Context, api LambdaGetFunctionAPI, name string) (*lambdatypes.FunctionConfiguration, error) {
	out, err := api.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(name)})
	if err != nil {
		return nil, err
	}
	if out == nil || out.Configuration == nil {
		return nil, UnusableAnswerErr{Call: "GetFunction", Field: "configuration"}
	}
	return out.Configuration, nil
}

// lambdaPolicy reads the resource policy of the function, or of one of its
// aliases or versions when qualifier is set. A missing policy is the healthy
// answer, returned as ResourceNotFoundException.
func lambdaPolicy(ctx context.Context, api LambdaGetPolicyAPI, name, qualifier string) (string, error) {
	in := &lambda.GetPolicyInput{FunctionName: aws.String(name)}
	if qualifier != "" {
		in.Qualifier = aws.String(qualifier)
	}
	out, err := api.GetPolicy(ctx, in)
	if err != nil {
		return "", err
	}
	if out == nil {
		return "", nil
	}
	return aws.ToString(out.Policy), nil
}

func lambdaFunctionURLs(ctx context.Context, api LambdaListFunctionUrlConfigsAPI, name string) ([]lambdatypes.FunctionUrlConfig, error) {
	// ListFunctionUrlConfigs pages by Marker and NextMarker
	// (https://docs.aws.amazon.com/lambda/latest/api/API_ListFunctionUrlConfigs.html).
	configs, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]lambdatypes.FunctionUrlConfig, *string, error) {
		out, err := api.ListFunctionUrlConfigs(ctx, &lambda.ListFunctionUrlConfigsInput{FunctionName: aws.String(name), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		if out == nil {
			return nil, nil, UnusableAnswerErr{Call: "ListFunctionUrlConfigs", Field: "function URL configs"}
		}
		return out.FunctionUrlConfigs, out.NextMarker, nil
	})
	if err != nil {
		return nil, err
	}
	if !complete {
		return nil, errPageCap
	}
	return configs, nil
}

// lambdaURLQualifier is the alias a function URL belongs to, read off the
// URL's FunctionArn ("…:function:<name>:<alias>"); "" for a URL on the
// function itself.
func lambdaURLQualifier(u lambdatypes.FunctionUrlConfig) string {
	a, ok := ARNForService(aws.ToString(u.FunctionArn), "lambda")
	if !ok {
		return ""
	}
	if parts := strings.Split(a.Resource, ":"); len(parts) == 3 {
		return parts[2]
	}
	return ""
}

// lambdaURLPolicies reads the resource policy of each alias a URL belongs to,
// once per alias: a policy is per qualifier, so an alias URL is authorized by
// its alias's policy alone ("The scope of that permission depends on whether
// you apply it to an alias, a version, or the entire function"). An alias
// with no policy maps to "". The unqualified policy is the caller's own read.
func lambdaURLPolicies(ctx context.Context, api LambdaGetPolicyAPI, name string, urls []lambdatypes.FunctionUrlConfig) (map[string]string, []error) {
	policies := map[string]string{}
	var errs []error
	for _, u := range urls {
		q := lambdaURLQualifier(u)
		if _, seen := policies[q]; seen || q == "" {
			continue
		}
		doc, err := lambdaPolicy(ctx, api, name, q)
		if err != nil && !isLambdaNoPolicy(err) {
			errs = append(errs, err)
			continue
		}
		policies[q] = doc
	}
	return policies, errs
}

// lambdaInvokeURLAction is the action a function URL call is authorized by.
// "If you delete your function URL with auth type NONE, Lambda doesn't
// automatically delete the associated resource-based policy", and the grant
// reaches nothing without a URL.
const lambdaInvokeURLAction = "lambda:InvokeFunctionUrl"

// lambdaExposure is the one join of the function's resource policies and its
// URLs, raising both exposure findings so one fact is one finding. policies
// holds the document of the function ("", absent when the function has no
// policy) and of each alias a URL belongs to; an alias missing from it is one
// whose read failed, and its URLs are undecided. Each URL is judged by the policy of its own qualifier:
//   - a public URL grant (functionURLGrants) with a URL whose AuthType is NONE
//     is lambda.function-url-public: anyone on the internet can call it;
//   - a public grant that reaches the function directly is
//     lambda.public-policy, and so is a public URL grant on the function's
//     own URL when that URL requires signed requests, which any AWS account
//     can sign;
//   - a public URL grant with no URL raises nothing.
//
// urlsRead is false when the URL read failed: the URL grants are then
// undecided, and the failed read marks the row.
func lambdaExposure(result *IssueEnricherResult, id string, read func(doc string) policyReading, policies map[string]string, urls []lambdatypes.FunctionUrlConfig, urlsRead bool, class catalog.LifecycleClass, state string) {
	urlGrantOf := map[string]bool{}
	for q, doc := range policies {
		urlGrantOf[q], _ = read(doc).functionURLGrants()
	}
	var open *lambdatypes.FunctionUrlConfig
	signed := false
	for i, u := range urls {
		q := lambdaURLQualifier(u)
		if !urlGrantOf[q] {
			continue
		}
		switch {
		case u.AuthType == lambdatypes.FunctionUrlAuthTypeNone:
			if open == nil {
				open = &urls[i]
			}
		case q == "":
			signed = true
		}
	}
	if open != nil && catalog.Applies("lambda", lambdaCodeFunctionURLPublic, class, state, "") {
		rows := []domain.DetailRow{{Label: "Endpoint auth", Value: "none", Tier: "!"}}
		if q := lambdaURLQualifier(*open); q != "" {
			rows = append(rows, domain.DetailRow{Label: "Alias", Value: q, Tier: "!"})
		}
		if open.Cors != nil && slices.Contains(open.Cors.AllowOrigins, "*") {
			rows = append(rows, domain.DetailRow{Label: "Allowed origins", Value: "any", Tier: "!"})
		}
		setWave2Finding(result, id, lambdaCodeFunctionURLPublic, rows)
	}
	doc, hasPolicy := policies[""]
	if !hasPolicy {
		return
	}
	p := read(doc)
	if p.public() {
		urlGrant, otherGrant := p.functionURLGrants()
		ownNoneURL := slices.ContainsFunc(urls, func(u lambdatypes.FunctionUrlConfig) bool {
			return lambdaURLQualifier(u) == "" && u.AuthType == lambdatypes.FunctionUrlAuthTypeNone
		})
		p = p.publicOnlyIf(otherGrant || (urlGrant && urlsRead && signed && !ownNoneURL))
	}
	p.raiseIn(result, id, class, state)
}

// isLambdaNoPolicy reports the wire code Lambda answers when the thing asked
// for is absent: "no resource policy / no URL config" (healthy) from GetPolicy
// and ListFunctionUrlConfigs, "the function no longer exists" from
// GetFunction.
func isLambdaNoPolicy(err error) bool {
	return ErrCodeIs(err, "ResourceNotFoundException")
}
