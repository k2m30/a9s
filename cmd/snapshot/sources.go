package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	smithy "github.com/aws/smithy-go"
)

// outcomeNotFound is the word for an item a list named and a describe
// answered nothing for.
const outcomeNotFound = "not_found"

// outcomeNone is the word for a configuration AWS states does not exist.
const outcomeNone = "none"

// readOutcome is what one read answered for an item: "ok" (or a word of the
// read's own, such as "found"), outcomeNone, outcomeNotFound, or "error" with
// the code AWS returned.
type readOutcome struct {
	Outcome   string `json:"outcome"`
	ErrorCode string `json:"error_code,omitempty"`
}

// absentCodes are the errors by which an operation states that the
// configuration it reads does not exist: an answer, not a failure.
var absentCodes = map[string]string{
	"GetPublicAccessBlock":            "NoSuchPublicAccessBlockConfiguration",
	"GetBucketPolicyStatus":           "NoSuchBucketPolicy",
	"GetBucketLifecycleConfiguration": "NoSuchLifecycleConfiguration",
	"GetObjectLockConfiguration":      "ObjectLockConfigurationNotFoundError",
	"GetLoginProfile":                 "NoSuchEntity",
	"DescribeImageScanFindings":       "ScanNotFoundException",
}

// outcomeOf is the one place a read's error becomes a recorded word: a
// refused or failed read is "error" with its code, never an empty answer, and
// an error that states an absent configuration is outcomeNone.
func outcomeOf(err error) (outcome, code string) {
	if err == nil {
		return "ok", ""
	}
	if opErr, ok := errors.AsType[*smithy.OperationError](err); ok && absentCodes[opErr.Operation()] == apiErrorCode(err) {
		return outcomeNone, ""
	}
	return "error", apiErrorCode(err)
}

func readOf(err error) readOutcome {
	o, c := outcomeOf(err)
	return readOutcome{Outcome: o, ErrorCode: c}
}

func apiErrorCode(err error) string {
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		return apiErr.ErrorCode()
	}
	return err.Error()
}

// goneCodes are the codes a describe answers for an item deleted after the
// list named it, for the operations describeListed and the eb-rule bus walk
// send. Athena's InvalidRequestException also answers a request it could not
// parse, so there the item is gone only when the message carries the
// not-found sentence ("WorkGroup <name> is not found.").
var goneCodes = map[string][]string{
	"ResourceNotFoundException":               nil, // EKS, OpenSearch, Transfer, MWAA, DynamoDB, Kinesis, Backup, EventBridge
	"NotFoundException":                       nil, // KMS
	"NoSuchEntity":                            nil, // IAM
	"InvalidLaunchTemplateId.NotFound":        nil, // EC2 launch templates
	"ClusterNotFoundException":                nil, // ECS
	"QueueDoesNotExist":                       nil, // SQS
	"AWS.SimpleQueueService.NonExistentQueue": nil, // SQS, the legacy query error header
	"WAFNonexistentItemException":             nil, // WAFv2
	"NoSuchDistribution":                      nil, // CloudFront
	"InvalidRequestException":                 {"is not found"},
}

func isGone(err error) bool {
	apiErr, ok := errors.AsType[smithy.APIError](err)
	if !ok {
		return false
	}
	shapes, known := goneCodes[apiErr.ErrorCode()]
	return known && (len(shapes) == 0 || slices.ContainsFunc(shapes, func(s string) bool {
		return strings.Contains(apiErr.ErrorMessage(), s)
	}))
}

type listedRow[T any] struct {
	id   string
	item *T
	read readOutcome
}

// describeListed asks describe about ids, size at a time, and returns a row
// for every item an answer described and for every listed id an answer named
// nothing for (outcomeNotFound) or a failed call asked about ("error", so one
// item's failure never fails the type). An id the service reports gone, in
// the answer's gone list or as a not-found error, has no row: it was deleted
// after the list. An id listed twice is asked about once.
func describeListed[T any](ids []string, size int, describe func(batch []string) (found map[string]T, gone []string, err error)) []listedRow[T] {
	var rows []listedRow[T]
	var unique []string
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	for batch := range slices.Chunk(unique, size) {
		found, gone, err := describe(batch)
		if isGone(err) {
			continue
		}
		if err != nil {
			for _, id := range batch {
				rows = append(rows, listedRow[T]{id: id, read: readOf(err)})
			}
			continue
		}
		for _, id := range batch {
			item, ok := found[id]
			switch {
			case ok:
				rows = append(rows, listedRow[T]{id: id, item: &item, read: readOf(nil)})
				delete(found, id)
			case !slices.Contains(gone, id):
				rows = append(rows, listedRow[T]{id: id, read: readOutcome{Outcome: outcomeNotFound}})
			}
		}
		for _, id := range slices.Sorted(maps.Keys(found)) {
			item := found[id]
			rows = append(rows, listedRow[T]{id: id, item: &item, read: readOf(nil)})
		}
	}
	return rows
}

// rowOf is the row describeListed returned for id, or, for an id the service
// reported gone, the outcomeNotFound row of an answer that named nothing.
func rowOf[T any](rows []listedRow[T], id string) listedRow[T] {
	if i := slices.IndexFunc(rows, func(r listedRow[T]) bool { return r.id == id }); i >= 0 {
		return rows[i]
	}
	return describeListed([]string{id}, 1, func([]string) (map[string]T, []string, error) { return nil, nil, nil })[0]
}

// readOne is a sub-read of one item through describeListed: a nil answer, or
// an error stating the item is gone, is outcomeNotFound.
func readOne[T any](get func() (*T, error)) (*T, readOutcome) {
	row := describeListed([]string{""}, 1, func([]string) (map[string]T, []string, error) {
		item, err := get()
		if isGone(err) {
			return nil, nil, nil
		}
		return describedAs("", item), nil, err
	})[0]
	return row.item, row.read
}

func describedAs[T any](id string, item *T) map[string]T {
	if item == nil {
		return nil
	}
	return map[string]T{id: *item}
}

// partialRead is embedded in the data of a type listed from several calls:
// the calls that failed while the others' rows were kept.
type partialRead struct {
	PartialErrors []string `json:"partial_errors,omitempty"`
}

func (p *partialRead) setPartial(e []string) { p.PartialErrors = e }

func (p partialRead) partial() []string { return p.PartialErrors }

type sourceResult struct {
	name string
	err  error
}

// settleSources applies the partial-success contract of a type listed from
// several calls: a source that failed is reported in partial while the others'
// rows are kept, and every source failing is an error.
func settleSources(label string, results ...sourceResult) (partial []string, err error) {
	for _, r := range results {
		if r.err != nil {
			partial = append(partial, fmt.Sprintf("%s: %v", r.name, r.err))
		}
	}
	if len(partial) == len(results) && len(results) > 0 {
		return partial, fmt.Errorf("%s: %s", label, strings.Join(partial, "; "))
	}
	return partial, nil
}

func rowCount(data any) int {
	v := reflect.ValueOf(data).Elem()
	n := 0
	for i := range v.NumField() {
		if f := v.Field(i); f.Kind() == reflect.Slice && v.Type().Field(i).IsExported() {
			n += f.Len()
		}
	}
	return n
}

// part is one list call of a type whose rows come from more than one.
type part[T any] struct {
	name string
	run  func(ctx context.Context, cfg aws.Config, data *T) error
}

// perParent plans one part per parent the list names, each reading that
// parent's children; a parent the service reports gone has no rows and no
// failure.
func perParent[T any](list func(context.Context, aws.Config) ([]string, error), run func(ctx context.Context, cfg aws.Config, parent string, d *T) error) func(context.Context, aws.Config) ([]part[T], error) {
	return func(ctx context.Context, cfg aws.Config) ([]part[T], error) {
		parents, err := list(ctx, cfg)
		parts := make([]part[T], 0, len(parents))
		for _, parent := range parents {
			parts = append(parts, part[T]{name: parent, run: func(ctx context.Context, cfg aws.Config, d *T) error {
				if childErr := run(ctx, cfg, parent, d); !isGone(childErr) {
					return childErr
				}
				return nil
			}})
		}
		return parts, err
	}
}

func fixed[T any](parts ...part[T]) func(context.Context, aws.Config) ([]part[T], error) {
	return func(context.Context, aws.Config) ([]part[T], error) { return parts, nil }
}

// composed declares a capture whose data is built by several list calls: plan
// names the parts (a list may have to be read to know them, and a plan whose
// list failed part way names the parts it read with the error), the parts run
// in order under the partial-success contract of settleSources (sources that
// all failed after reading rows have kept those rows, so they are partial),
// and finish, when not nil, completes the data from what the parts read.
func composed[T any, P interface {
	*T
	setPartial([]string)
}](label string, plan func(context.Context, aws.Config) ([]part[T], error), finish func(context.Context, aws.Config, *T) error) captureFunc {
	return func(ctx context.Context, cfg aws.Config) (any, error) {
		parts, planErr := plan(ctx, cfg)
		var results []sourceResult
		if planErr != nil {
			results = append(results, sourceResult{name: "list", err: planErr})
		}
		var data T
		for _, p := range parts {
			results = append(results, sourceResult{name: p.name, err: p.run(ctx, cfg, &data)})
		}
		partial, err := settleSources(label, results...)
		if err != nil && rowCount(&data) == 0 {
			return nil, err
		}
		if finish != nil {
			if err := finish(ctx, cfg, &data); err != nil {
				return nil, err
			}
		}
		P(&data).setPartial(partial)
		return data, nil
	}
}
