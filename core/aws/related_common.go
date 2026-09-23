// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Generic helpers shared by every *_related.go checker.
package aws

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
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

// relatedResultTrunc is the result of a checker that searched: ids found,
// and whether the search stopped short. A truncated scan renders "(N+)" —
// 0+ and 10+ are the same case (N found so far, list truncated). A complete
// search that found nothing is a proven zero.
func relatedResultTrunc(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	if !truncated && !slices.ContainsFunc(ids, func(id string) bool { return id != "" }) {
		return resource.ProvenZero(target, "the checker's search")
	}
	return resource.KnownRelated(target, ids, truncated)
}

// heuristicResult is the result of a pivot that matches by a property every
// related resource must share with the source but unrelated ones may share
// too: the matches are candidates, and a complete scan that found none is a
// proven zero.
func heuristicResult(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	if !truncated && len(ids) == 0 {
		return resource.ProvenZero(target, "the checker's scan")
	}
	r := resource.HeuristicRelated(target, ids)
	if truncated {
		return r.PartialScan()
	}
	return r
}

// logGroupsNaming offers the log groups whose name carries name, as
// candidates. A log group's name is free text an operator chooses, and what
// binds one to a resource is written where no list response reaches — the
// awslogs-group option inside a task definition, a CloudWatch agent's
// configuration file — so carrying the source's name is a property an
// unrelated group may share too.
func logGroupsNaming(ctx context.Context, clients any, cache resource.ResourceCache, name string) resource.RelatedCheckResult {
	if name == "" {
		return resource.ProvenZero("logs", "the source's name")
	}
	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}
	var ids []string
	for _, logRes := range logList {
		if strings.Contains(logRes.ID, name) {
			ids = append(ids, logRes.ID)
		}
	}
	return heuristicResult("logs", ids, truncated)
}

// ecsTaskDefLogGroups is the logs pivot of a workload that runs a task
// definition: the awslogs-group option of every container of the definition,
// which is where that container's output goes and the link AWS records, read
// with one ecs:DescribeTaskDefinition. A definition nobody could read proves
// nothing either way, so the groups whose name carries the family are left as
// candidates.
func ecsTaskDefLogGroups(ctx context.Context, clients any, cache resource.ResourceCache, taskDefARN string) resource.RelatedCheckResult {
	family := taskDefFamily(taskDefARN)
	def, err := ecsTaskDefinition(ctx, clients, taskDefARN)
	// A refusal and a definition answered without a body are the same answer
	// here: nothing was read.
	// no finding: the candidate row below is what the operator is owed, and it already says the link was not read.
	if err != nil || def == nil {
		return logGroupsNaming(ctx, clients, cache, family)
	}
	var groups []string
	for _, container := range def.ContainerDefinitions {
		if container.LogConfiguration == nil || container.LogConfiguration.LogDriver != ecstypes.LogDriverAwslogs {
			continue
		}
		if g := container.LogConfiguration.Options["awslogs-group"]; g != "" {
			groups = append(groups, g)
		}
	}
	if len(groups) == 0 {
		return resource.ProvenZero("logs", "the definition's awslogs-group options")
	}
	logList, _, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}
	ids, lowerBound := listedRefs("logs", groups, refContext(clients, cache, "logs"), logList)
	return relatedResultTrunc("logs", ids, lowerBound)
}

// ecsTaskDefinition reads one task definition. errClientMissing stands for a
// session with no ECS client, which read nothing.
func ecsTaskDefinition(ctx context.Context, clients any, taskDefARN string) (*ecstypes.TaskDefinition, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.ECS == nil || taskDefARN == "" {
		return nil, errClientMissing
	}
	api, ok := c.ECS.(ECSDescribeTaskDefinitionAPI)
	if !ok {
		return nil, errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{TaskDefinition: &taskDefARN})
	})
	if err != nil {
		return nil, err
	}
	return out.TaskDefinition, nil
}

// alarmIDsByDimension is the resource→alarm pivot of every type an alarm can
// name: the alarms that name this row, read through the one match table the
// alarm→resource direction reads too. A row whose identity could not be read
// has nothing to match against — reported as a proven zero, or as unknown
// when the row arrived without its RawStruct. A nil alarm list means the
// alarm cache/fetcher gave no answer at all — reported as unknown, never as
// a proven zero.
// also, when given, is a second way an alarm names this row, for a type whose
// link to an alarm is not written in the alarm's dimensions.
func alarmIDsByDimension(ctx context.Context, clients any, cache resource.ResourceCache, source string, res resource.Resource, also ...func(cwtypes.MetricAlarm) bool) resource.RelatedCheckResult {
	spec, ok := AlarmMatchSpecFor(source)
	if !ok {
		return resource.UnknownRelated("alarm")
	}
	values, read := spec.alarmValuesOf(res)
	if !read {
		return resource.UnknownRelated("alarm")
	}
	if len(values) == 0 {
		return unreadZero(res, resource.ProvenZero("alarm", "the row's identity"))
	}

	alarmList, _, truncated, err := relatedListIn(ctx, clients, cache, "alarm", spec.metricsRegionOf(res))
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
		if spec.names(alarm, res) || anyMatch(also, alarm) {
			ids = append(ids, alarmRes.ID)
		}
	}
	result := relatedResultTrunc("alarm", ids, truncated)
	if spec.ValuesFromRawStruct {
		return unreadZeroScanned(res, len(alarmList), result)
	}
	return result
}

// alarmRowsNaming is the alarm→target pivot of every type an alarm can name:
// the rows of target this alarm names, through the same match table the
// target's own alarm pivot reads, so the two ends cannot disagree.
// also, when given, is a second way the alarm names a row of target.
func alarmRowsNaming(ctx context.Context, clients any, cache resource.ResourceCache, target string, res resource.Resource, also ...func(resource.Resource) bool) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated(target)
	}
	spec, ok := AlarmMatchSpecFor(target)
	if !ok {
		return resource.UnknownRelated(target)
	}
	rows, truncated, err := relatedResourcesFor(ctx, clients, cache, target)
	if err != nil {
		return resource.ErrorRelated(target, err)
	}
	if rows == nil {
		return resource.UnknownRelated(target)
	}
	// An alarm watches the metrics of its own region, so a row whose metrics
	// are published elsewhere is none of this alarm's business.
	alarmRegion := arnRegionOf(aws.ToString(alarm.AlarmArn), "cloudwatch")
	var ids []string
	for _, row := range rows {
		if region := spec.metricsRegionOf(row); region != "" && alarmRegion != "" && region != alarmRegion {
			continue
		}
		if spec.names(alarm, row) || anyMatch(also, row) {
			ids = append(ids, row.ID)
		}
	}
	return relatedResultTrunc(target, ids, truncated)
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
// and typed → the asserting rows only. A row that carries no T — a row of
// another type, a row whose details could not be read — may be the one that
// matches, so dropping it makes the rows a subset and truncated true.
func cachedTypedRows[T any](cache resource.ResourceCache, shortName string) (rows []typedRow[T], truncated bool, ok bool) {
	entry, present := cache[shortName]
	if !present {
		return nil, false, false
	}
	if len(entry.Resources) == 0 {
		return nil, entry.IsTruncated, true
	}
	dropped := false
	for _, r := range entry.Resources {
		raw, asserted := assertStruct[T](r.RawStruct)
		if !asserted {
			dropped = true
			continue
		}
		rows = append(rows, typedRow[T]{ID: r.ID, Raw: raw})
	}
	if len(rows) == 0 {
		return nil, false, false
	}
	return rows, entry.IsTruncated || dropped, true
}

// lambdaEventSourceMappingLambdaCheck is shared by checkKinesisLambda and
// checkMSKLambda. Both pivots need the same mechanism: a stream/cluster ARN
// is the Lambda event source, and lambda:ListEventSourceMappings filtered by
// EventSourceArn (one call per open resource — the per-open call budget in
// docs/related-resources.md) is itself the authoritative mechanism —
// its FunctionArn values are the definitive answer,
// each read as the function name the lambda list is keyed by.
func lambdaEventSourceMappingLambdaCheck(ctx context.Context, clients any, eventSourceArn string, cache resource.ResourceCache) resource.RelatedCheckResult {
	return lambdaEventSourceMappingsNaming(ctx, clients, cache, lambda.ListEventSourceMappingsInput{EventSourceArn: aws.String(eventSourceArn)}, nil)
}

// lambdaEventSourceMappingsNaming reports the functions behind the event
// source mappings filter returns whose source keep accepts. A nil keep takes
// every mapping filter returned, for an event source whose ARN is the same
// one for as long as it exists.
func lambdaEventSourceMappingsNaming(ctx context.Context, clients any, cache resource.ResourceCache, filter lambda.ListEventSourceMappingsInput, keep func(eventSourceARN string) bool) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("lambda")
	}
	api, ok := c.Lambda.(LambdaListEventSourceMappingsAPI)
	if !ok {
		return resource.UnknownRelated("lambda")
	}

	mappings, complete, err := listEventSourceMappings(ctx, api, filter)
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}

	var functionArns []string
	for _, m := range mappings {
		if keep != nil && !keep(aws.ToString(m.EventSourceArn)) {
			continue
		}
		functionArns = append(functionArns, aws.ToString(m.FunctionArn))
	}
	ids, dropped := resolveRefs("lambda", functionArns, refContext(clients, cache, "lambda"))
	return relatedResultTrunc("lambda", ids, dropped || !complete)
}

// eventSourceARNs returns the EventSourceArn of every mapping whose source is
// an ARN of service ("sqs", "kinesis", "kafka").
func eventSourceARNs(mappings []lambdatypes.EventSourceMappingConfiguration, service string) []string {
	var arns []string
	for _, m := range mappings {
		if _, ok := ARNForService(aws.ToString(m.EventSourceArn), service); ok {
			arns = append(arns, *m.EventSourceArn)
		}
	}
	return arns
}

func anyMatch[T any](predicates []func(T) bool, v T) bool {
	for _, p := range predicates {
		if p(v) {
			return true
		}
	}
	return false
}
