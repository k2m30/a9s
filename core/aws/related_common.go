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

	"github.com/k2m30/a9s/v3/core/resource"
)

// errRawStructMissing marks the one reason a checker cannot read its own row
// that is not a failure: the row carries no RawStruct. Rows restored from the
// on-disk cache never do — it is deliberately not persisted (core/cache) — and
// the detail operation runs the checkers against the row as it stands, before
// enrichment refills it. Nothing was read and nothing failed, so the panel owes
// a "?".
var errRawStructMissing = errors.New("resource details have not been read yet")

// relatedFromErr turns the error a two-hop helper returned into the result the
// panel owes. It is the single place that knows "we have not read this row yet"
// is Unknown while every other error is Error.
func relatedFromErr(target string, err error) resource.RelatedCheckResult {
	if errors.Is(err, errRawStructMissing) {
		return NotRead(target)
	}
	return ReadFailed(target, err)
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

// logGroupsNaming offers the log groups whose name carries name, as
// candidates. A log group's name is free text an operator chooses, and what
// binds one to a resource is written where no list response reaches — the
// awslogs-group option inside a task definition, a CloudWatch agent's
// configuration file — so carrying the source's name is a property an
// unrelated group may share too.
func logGroupsNaming(ctx context.Context, clients any, cache resource.ResourceCache, name string) resource.RelatedCheckResult {
	if name == "" {
		return relatedAnswer("logs", relatedRead{unread: true})
	}
	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return ReadFailed("logs", err)
	}
	if logList == nil {
		return NotRead("logs")
	}
	var ids []string
	for _, logRes := range logList {
		if textNames(logRes.ID, name) {
			ids = append(ids, logRes.ID)
		}
	}
	return candidatesResult("logs", ids, truncated)
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
	groups, whole := ecsContainerLogGroups(def, sessionRegion(clients))
	containers := relatedRead{unread: !whole}
	if len(groups) == 0 {
		return relatedAnswer("logs", containers)
	}
	logList, _, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if logList == nil {
		return relatedAnswer("logs", joinReads(containers, unreadBy(err)))
	}
	ids, lowerBound := listedRefs("logs", groups, refContext(clients, cache, "logs"), logList)
	return relatedAnswer("logs", joinReads(containers, relatedRead{ids: ids, partial: lowerBound}))
}

// ecsContainerLogGroups is the one reader of the log groups the containers of
// def write to, in region, and whether every container's destination could
// be read from def:
//   - the awslogs driver's awslogs-group, unless its awslogs-region names
//     another Region (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_awslogs.html);
//   - FireLens (awsfirelens), whose options become the log router's output
//     configuration (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/firelens-taskdef.html):
//     Fluent Bit's cloudwatch_logs output sends to its log_group_name
//     (https://docs.fluentbit.io/manual/data-pipeline/outputs/cloudwatch), and
//     other outputs to no log group. A log_group_template names its group
//     per record, and a container with no options routes through a
//     configuration file def does not hold: neither is readable here.
func ecsContainerLogGroups(def *ecstypes.TaskDefinition, region string) (groups []string, whole bool) {
	whole = true
	for _, container := range def.ContainerDefinitions {
		logs := container.LogConfiguration
		if logs == nil {
			continue
		}
		switch logs.LogDriver {
		case ecstypes.LogDriverAwslogs:
			if r := logs.Options["awslogs-region"]; r != "" && region != "" && r != region {
				continue
			}
			if g := logs.Options["awslogs-group"]; g != "" {
				groups = append(groups, g)
			}
		case ecstypes.LogDriverAwsfirelens:
			name := logs.Options["Name"]
			switch {
			case len(logs.Options) == 0:
				whole = false
			case name != "cloudwatch_logs" && name != "cloudwatch":
			case logs.Options["log_group_template"] != "" || logs.Options["log_group_name"] == "":
				whole = false
			case logs.Options["region"] != "" && region != "" && logs.Options["region"] != region:
			default:
				groups = append(groups, logs.Options["log_group_name"])
			}
		}
	}
	return groups, whole
}

// ecsSecretRefs is the one classifier of the secret references def makes:
// every container's secrets and its log driver's secretOptions
// (https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Secret.html),
// and its private-registry credentials secret. A valueFrom is "either the full
// ARN of the AWS Secrets Manager secret or the full ARN of the parameter in
// the SSM Parameter Store", and a parameter in the task's Region may be named
// by its name alone; a Secrets Manager ARN may carry
// ":json-key:version-stage:version-id"
// (https://docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-envvar-secrets-manager.html),
// which the secrets resolver reads past.
func ecsSecretRefs(def *ecstypes.TaskDefinition) (secrets, params []string) {
	for _, c := range def.ContainerDefinitions {
		refs := slices.Clone(c.Secrets)
		if c.LogConfiguration != nil {
			refs = append(refs, c.LogConfiguration.SecretOptions...)
		}
		for _, s := range refs {
			v := aws.ToString(s.ValueFrom)
			_, isSSM := ARNForService(v, "ssm")
			switch {
			case isSecret(v):
				secrets = append(secrets, v)
			case isSSM || v != "" && !strings.HasPrefix(v, "arn:"):
				params = append(params, v)
			}
		}
		if c.RepositoryCredentials != nil && aws.ToString(c.RepositoryCredentials.CredentialsParameter) != "" {
			secrets = append(secrets, *c.RepositoryCredentials.CredentialsParameter)
		}
	}
	return secrets, params
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
		return NotRead("alarm")
	}
	res, err := spec.complete(ctx, clients, res)
	if err != nil {
		return ReadFailed("alarm", err)
	}
	values, read := spec.alarmValuesOf(res)
	if !read {
		return NotRead("alarm")
	}
	if len(values) == 0 {
		return unreadZero(res, foundNone("alarm", "the row's identity"))
	}

	alarmList, _, truncated, err := relatedListIn(ctx, clients, cache, "alarm", spec.metricsRegionOf(res))
	if err != nil {
		return ReadFailed("alarm", err)
	}
	if alarmList == nil {
		return NotRead("alarm")
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
		return NotRead(target)
	}
	spec, ok := AlarmMatchSpecFor(target)
	if !ok {
		return NotRead(target)
	}
	read := relatedRowsByID
	if spec.ValuesFromRawStruct {
		read = FetchRelatedTarget
	}
	rows, truncated, err := read(ctx, clients, cache, target)
	if err != nil {
		return ReadFailed(target, err)
	}
	if rows == nil {
		return NotRead(target)
	}
	// An alarm watches the metrics of its own region, so a row whose metrics
	// are published elsewhere is none of this alarm's business.
	alarmRegion := arnRegionOf(aws.ToString(alarm.AlarmArn), "cloudwatch")
	// A row completed by a call of its own is read only for an alarm that
	// could name one: any other names none, whatever the call would say.
	completing := spec.Complete != nil && spec.watchedBy(alarm)
	var ids []string
	var reads rowReads
	for _, row := range rows {
		if region := spec.metricsRegionOf(row); region != "" && alarmRegion != "" && region != alarmRegion {
			continue
		}
		if completing {
			completed, err := spec.complete(ctx, clients, row)
			if err != nil {
				reads.fail(row.ID, err)
				continue
			}
			row = completed
		}
		reads.read++
		if spec.names(alarm, row) || anyMatch(also, row) {
			ids = append(ids, row.ID)
		}
	}
	return reads.answer(target, "alarm-related: "+target, ids, truncated)
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
		return NotRead("lambda")
	}
	api, ok := c.Lambda.(LambdaListEventSourceMappingsAPI)
	if !ok {
		return NotRead("lambda")
	}

	mappings, complete, err := listEventSourceMappings(ctx, api, filter)
	if err != nil {
		return ReadFailed("lambda", err)
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
