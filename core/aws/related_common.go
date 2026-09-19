// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Generic helpers shared by every *_related.go checker.
package aws

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// errRawStructMissing marks the one reason a checker cannot read its own row
// that is not a failure: the row carries no RawStruct. Rows restored from the
// on-disk cache never do — it is deliberately not persisted (core/cache) — and
// the detail operation runs the checkers against the row as it stands, before
// enrichment refills it. Nothing was read and nothing failed, so the panel owes
// a "?".
var errRawStructMissing = errors.New("resource details have not been read yet")

// unreadZero is what a checker owes when it found nothing and could not read
// its source row. A disk-cache replay carries no RawStruct, so a zero there is
// a claim about a row nobody looked at — the checker had no filter to scan
// with, and "none" and "we have not looked" are not the same answer.
//
// Everything else passes through untouched: anything the checker did find, an
// error, an already-unknown, and a truncated zero — a truncated result means a
// real list was read with whatever filter the row's Fields could supply, and
// that lower bound is honest whether or not the RawStruct was there.
func unreadZero(res resource.Resource, r resource.RelatedCheckResult) resource.RelatedCheckResult {
	if res.RawStruct != nil || r.State() != domain.RelatedResolved || r.Count() != 0 || r.Truncated() {
		return r
	}
	return resource.UnknownRelated(r.TargetType())
}

// unreadZeroScanned is unreadZero for a checker that did read the target list.
// A zero over a population of none is proven whatever the source row could or
// could not say — no row existed for it to match — so only a zero over rows
// that were really there is a claim the source row has to back.
func unreadZeroScanned(res resource.Resource, scanned int, r resource.RelatedCheckResult) resource.RelatedCheckResult {
	if scanned == 0 {
		return r
	}
	return unreadZero(res, r)
}

// relatedFromErr turns the error a two-hop helper returned into the result the
// panel owes. It is the single place that knows "we have not read this row yet"
// is Unknown while every other error is Error.
func relatedFromErr(target string, err error) resource.RelatedCheckResult {
	if errors.Is(err, errRawStructMissing) {
		return resource.UnknownRelated(target)
	}
	return resource.ErrorRelated(target, err)
}

// assertStruct extracts a value of type T from an interface that may hold
// either T or *T. Used for RawStruct type assertions across related checkers.
// Falls back to searching v's exported anonymous (embedded) struct fields
// for a T — a detail enricher's wrapper (e.g. InstanceEnriched embedding
// ec2types.Instance) replaces RawStruct with itself, and every related
// checker asserting the raw SDK type must still resolve after that
// replacement.
func assertStruct[T any](v any) (T, bool) {
	if val, ok := v.(T); ok {
		return val, true
	}
	if p, ok := v.(*T); ok && p != nil {
		return *p, true
	}
	return findEmbeddedStruct[T](reflect.ValueOf(v))
}

// findEmbeddedStruct recursively searches rv's exported anonymous struct
// fields (pointer-deref'd) for a value of type T, returning the first match.
func findEmbeddedStruct[T any](rv reflect.Value) (T, bool) {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			var zero T
			return zero, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		var zero T
		return zero, false
	}
	rt := rv.Type()
	for i := range rt.NumField() {
		field := rt.Field(i)
		if !field.Anonymous || !field.IsExported() {
			continue
		}
		fv := rv.Field(i)
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				continue
			}
			fv = fv.Elem()
		}
		if fv.Kind() != reflect.Struct {
			continue
		}
		if val, ok := fv.Interface().(T); ok {
			return val, true
		}
		if val, ok := findEmbeddedStruct[T](fv); ok {
			return val, true
		}
	}
	var zero T
	return zero, false
}

// kmsRefFromField returns the KMS key reference a source field holds, for
// relatedRefs to read. A thin, cache-seeded source resource with no real
// encryption data can hand a checker a bare value that collides with the
// source's own resource-type short name (an s3 bucket's "KMSMasterKeyID"
// reading "s3"); no key ID, key ARN, alias name or alias ARN is ever a bare
// a9s type short name, so such a value is dropped ("") rather than handed on
// to DescribeKey/FetchByIDs.
func kmsRefFromField(raw, srcType string) string {
	if srcType != "" && raw == srcType {
		return ""
	}
	return raw
}

func relatedResult(target string, ids []string) resource.RelatedCheckResult {
	return resource.KnownRelated(target, ids, false)
}

// relatedResultTrunc is relatedResult with the truncation flag carried through
// UNIFORMLY for any count. A truncated scan renders "(N+)" — the "+" means the
// target list is truncated, so navigating shows "m for more". 0+ and 10+ are the
// same case (N found so far, list truncated), not two: there is no special
// zero-truncated result.
func relatedResultTrunc(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	return resource.KnownRelated(target, ids, truncated)
}

// alarmIDsByDimension is the shared body of every check*Alarm function whose
// match rule is "one CloudWatch dimension name/value pair, exact equality,
// first match wins". namespace, when non-empty, additionally restricts
// matches to alarms in that AWS/* namespace (the sqs/cb/mwaa pattern); pass
// "" to skip the namespace guard (the dbi pattern). An empty dimValue means
// the caller had nothing to match against — reported as a proven zero, not
// unknown. A nil alarm list means the alarm cache/fetcher gave no answer at
// all — reported as unknown, never as a proven zero.
func alarmIDsByDimension(ctx context.Context, clients any, cache resource.ResourceCache, namespace, dimName, dimValue string) resource.RelatedCheckResult {
	if dimValue == "" {
		return resource.KnownRelated("alarm", nil, false)
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		if namespace != "" && (alarm.Namespace == nil || *alarm.Namespace != namespace) {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == dimName && d.Value != nil && *d.Value == dimValue {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("alarm", ids, truncated)
}

// typedRow pairs a cached Resource's ID with its RawStruct already asserted
// to T, so callers of cachedTypedRows never re-assert.
type typedRow[T any] struct {
	ID  string
	Raw T
}

// cachedTypedRows reads the shortName entry directly from cache — it never
// fetches. Tri-state contract: cache absent →
// (nil, false, false) = unknown; entry present but zero rows assert to T
// (disk-seeded, no RawStruct) → (nil, false, false) = unknown; entry present
// and typed → the asserting rows only (non-asserting rows are dropped).
func cachedTypedRows[T any](cache resource.ResourceCache, shortName string) (rows []typedRow[T], truncated bool, ok bool) {
	entry, present := cache[shortName]
	if !present {
		return nil, false, false
	}
	if len(entry.Resources) == 0 {
		return nil, entry.IsTruncated, true
	}
	for _, r := range entry.Resources {
		if raw, asserted := assertStruct[T](r.RawStruct); asserted {
			rows = append(rows, typedRow[T]{ID: r.ID, Raw: raw})
		}
	}
	if len(rows) == 0 {
		return nil, false, false
	}
	return rows, entry.IsTruncated, true
}

// lambdaEventSourceMappingLambdaCheck is shared by checkKinesisLambda and
// checkMSKLambda. Both pivots need the same mechanism: a stream/cluster ARN
// is the Lambda event source, and lambda:ListEventSourceMappings filtered by
// EventSourceArn (one call per open resource — the per-open call budget in
// docs/related-resources.md) is itself the authoritative mechanism —
// its FunctionArn values are the definitive answer,
// each read as the function name the lambda list is keyed by.
func lambdaEventSourceMappingLambdaCheck(ctx context.Context, clients any, eventSourceArn string, cache resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("lambda")
	}
	api, ok := c.Lambda.(LambdaListEventSourceMappingsAPI)
	if !ok {
		return resource.UnknownRelated("lambda")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.ListEventSourceMappingsOutput, error) {
		return api.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
			EventSourceArn: aws.String(eventSourceArn),
		})
	})
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}

	var functionArns []string
	for _, m := range out.EventSourceMappings {
		if m.FunctionArn != nil && *m.FunctionArn != "" {
			functionArns = append(functionArns, *m.FunctionArn)
		}
	}
	return relatedRefs("lambda", functionArns, refContext(clients, cache, "lambda"))
}

// eventSourceARNs returns the EventSourceArn of every mapping whose source is
// the service marked by service (":sqs:", ":kinesis:", ":kafka:").
func eventSourceARNs(mappings []lambdatypes.EventSourceMappingConfiguration, service string) []string {
	var arns []string
	for _, m := range mappings {
		if m.EventSourceArn != nil && strings.Contains(*m.EventSourceArn, service) {
			arns = append(arns, *m.EventSourceArn)
		}
	}
	return arns
}
