// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eb_pattern.go is the one reading of an EventBridge event pattern: whether a
// rule's pattern can match an event a service emits about one resource.
//
// "Event patterns have the same structure as the events they match. An event
// pattern either matches an event or it doesn't."
// https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html
// Values in a list match any one of them; the content filters are prefix,
// suffix, anything-but, equals-ignore-case, exists, numeric, cidr, wildcard,
// "" and null, and $or across fields.
// https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-create-pattern-operators.html
package aws

import (
	"encoding/json"
	"net/netip"
	"slices"
	"strings"

	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ebVerdict is what a pattern says about the events of one resource.
type ebVerdict int

const (
	ebNo ebVerdict = iota
	ebMatch
	// ebUnknown: the pattern could not be read — an operator EventBridge does
	// not document, a filter of the wrong shape, a field the service's events
	// do not carry — or it matches every resource of the type alike.
	ebUnknown
)

// ebEvent is one kind of event a service emits about a resource: the fields
// whose value the resource decides, and the fields the event carries with
// values it does not.
type ebEvent struct {
	// known maps a field path ("detail.repository-name") to its values.
	known map[string][]any
	// names are the known paths that say which resource the event is about.
	names map[string]bool
	// free are the paths the event carries with any value; a path ending in
	// "." covers everything below it.
	free []string
}

func (e ebEvent) isFree(path string) bool {
	for _, f := range e.free {
		if path == f || strings.HasSuffix(f, ".") && strings.HasPrefix(path, f) {
			return true
		}
	}
	return false
}

// ebEnvelope is the free part of every event's envelope.
// https://docs.aws.amazon.com/eventbridge/latest/ref/events-structure.html
var ebEnvelope = []string{"version", "id", "account", "time", "region"} //nolint:gochecknoglobals // static event envelope

// ebPatternMatches reads pattern against the kinds of event a resource emits.
// A pattern matches when one kind of event matches it through a field that
// names the resource; one that matches without naming it matches every
// resource of the type and is no answer about this one.
func ebPatternMatches(pattern string, events []ebEvent) ebVerdict {
	dec := json.NewDecoder(strings.NewReader(pattern))
	dec.UseNumber()
	var p map[string]any
	if err := dec.Decode(&p); err != nil {
		return ebUnknown
	}
	verdict := ebNo
	for _, ev := range events {
		switch v, named := ebObject(p, "", ev); {
		case v == ebMatch && named:
			return ebMatch
		case v != ebNo:
			verdict = ebUnknown
		}
	}
	return verdict
}

// ebObject reads the pattern object p at prefix: every field must match (and
// every branch of $or is one way to).
func ebObject(p map[string]any, prefix string, ev ebEvent) (ebVerdict, bool) {
	verdict, named := ebMatch, false
	for key, val := range p {
		var v ebVerdict
		var n bool
		switch sub := val.(type) {
		case map[string]any:
			v, n = ebObject(sub, prefix+key+".", ev)
		case []any:
			if key == "$or" {
				v, n = ebOr(sub, prefix, ev)
			} else {
				v, n = ebField(prefix+key, sub, ev)
			}
		default:
			v = ebUnknown
		}
		verdict = ebAnd(verdict, v)
		named = named || n
	}
	return verdict, named && verdict == ebMatch
}

// ebOr reads the branches of $or: any one matching is a match, and it names
// the resource only when every branch that matches does.
func ebOr(branches []any, prefix string, ev ebEvent) (ebVerdict, bool) {
	verdict, named, matched := ebNo, true, false
	for _, b := range branches {
		obj, ok := b.(map[string]any)
		if !ok {
			return ebUnknown, false
		}
		v, n := ebObject(obj, prefix, ev)
		switch v {
		case ebMatch:
			matched = true
			named = named && n
		case ebUnknown:
			if verdict == ebNo {
				verdict = ebUnknown
			}
		}
	}
	if matched {
		return ebMatch, named
	}
	return verdict, false
}

func ebAnd(a, b ebVerdict) ebVerdict {
	switch {
	case a == ebNo || b == ebNo:
		return ebNo
	case a == ebUnknown || b == ebUnknown:
		return ebUnknown
	}
	return ebMatch
}

// ebField reads the filters on one field: any one matching any of the
// event's values is a match.
func ebField(path string, filters []any, ev ebEvent) (ebVerdict, bool) {
	values, known := ev.known[path]
	if !known && !ev.isFree(path) {
		return ebUnknown, false
	}
	matched := false
	for _, f := range filters {
		m, valid := ebFilter(f, values)
		if !valid {
			return ebUnknown, false
		}
		matched = matched || m
	}
	switch {
	case !known:
		return ebMatch, false
	case matched:
		return ebMatch, ev.names[path]
	}
	return ebNo, false
}

// ebFilter reports whether filter f matches one of values, and whether f is a
// filter EventBridge documents in a shape it accepts.
func ebFilter(f any, values []any) (matched, valid bool) {
	switch x := f.(type) {
	case nil, string, json.Number, bool:
		return slices.ContainsFunc(values, func(v any) bool { return ebEqual(x, v) }), true
	case map[string]any:
		if len(x) != 1 {
			return false, false
		}
		for op, arg := range x {
			return ebOperator(op, arg, values)
		}
	}
	return false, false
}

func ebOperator(op string, arg any, values []any) (matched, valid bool) {
	switch op {
	case "prefix", "suffix":
		s, fold, ok := ebString(arg)
		if !ok {
			return false, false
		}
		test := strings.HasPrefix
		if op == "suffix" {
			test = strings.HasSuffix
		}
		return ebAnyString(values, func(v string) bool {
			if fold {
				return test(strings.ToLower(v), strings.ToLower(s))
			}
			return test(v, s)
		}), true
	case "equals-ignore-case":
		list, ok := ebStrings(arg)
		if !ok {
			return false, false
		}
		return ebAnyString(values, func(v string) bool {
			for _, s := range list {
				if strings.EqualFold(v, s) {
					return true
				}
			}
			return false
		}), true
	case "wildcard":
		s, ok := arg.(string)
		if !ok {
			return false, false
		}
		return ebAnyString(values, func(v string) bool { return ebWildcard(s, v) }), true
	case "exists":
		b, ok := arg.(bool)
		if !ok {
			return false, false
		}
		return (len(values) > 0) == b, true
	case "numeric":
		return ebNumeric(arg, values)
	case "cidr":
		s, ok := arg.(string)
		if !ok {
			return false, false
		}
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			return false, false
		}
		return ebAnyString(values, func(v string) bool {
			addr, err := netip.ParseAddr(v)
			return err == nil && prefix.Contains(addr)
		}), true
	case "anything-but":
		return ebAnythingBut(arg, values)
	}
	return false, false
}

// ebAnythingBut matches a value that is none of arg: a value, a list of
// values, or a prefix, suffix, wildcard or equals-ignore-case filter.
func ebAnythingBut(arg any, values []any) (matched, valid bool) {
	switch x := arg.(type) {
	case string, json.Number:
		return slices.ContainsFunc(values, func(v any) bool { return !ebEqual(x, v) }), true
	case []any:
		for _, item := range x {
			switch item.(type) {
			case string, json.Number:
			default:
				return false, false
			}
		}
		return slices.ContainsFunc(values, func(v any) bool {
			for _, item := range x {
				if ebEqual(item, v) {
					return false
				}
			}
			return true
		}), true
	case map[string]any:
		if len(x) != 1 {
			return false, false
		}
		for op, inner := range x {
			switch op {
			case "prefix", "suffix", "wildcard", "equals-ignore-case":
			default:
				return false, false
			}
			return slices.ContainsFunc(values, func(v any) bool {
				m, _ := ebOperator(op, inner, []any{v})
				return !m
			}), ebValid(op, inner)
		}
	}
	return false, false
}

func ebValid(op string, arg any) bool {
	_, valid := ebOperator(op, arg, nil)
	return valid
}

// ebNumeric matches a number within every bound of arg: ["<", 5, ">=", 1].
func ebNumeric(arg any, values []any) (matched, valid bool) {
	list, ok := arg.([]any)
	if !ok || len(list) == 0 || len(list)%2 != 0 {
		return false, false
	}
	type bound struct {
		op string
		n  float64
	}
	var bounds []bound
	for i := 0; i < len(list); i += 2 {
		op, ok := list[i].(string)
		num, isNum := list[i+1].(json.Number)
		if !ok || !isNum {
			return false, false
		}
		n, err := num.Float64()
		if err != nil {
			return false, false
		}
		switch op {
		case "<", "<=", "=", ">", ">=":
		default:
			return false, false
		}
		bounds = append(bounds, bound{op, n})
	}
	return slices.ContainsFunc(values, func(v any) bool {
		num, ok := v.(json.Number)
		if !ok {
			return false
		}
		x, err := num.Float64()
		if err != nil {
			return false
		}
		for _, b := range bounds {
			if !(b.op == "<" && x < b.n || b.op == "<=" && x <= b.n || b.op == "=" && x == b.n ||
				b.op == ">" && x > b.n || b.op == ">=" && x >= b.n) {
				return false
			}
		}
		return true
	}), true
}

// ebString reads the argument of prefix or suffix: a string, or
// {"equals-ignore-case": string}.
func ebString(arg any) (s string, fold, ok bool) {
	switch x := arg.(type) {
	case string:
		return x, false, true
	case map[string]any:
		if s, isStr := x["equals-ignore-case"].(string); isStr && len(x) == 1 {
			return s, true, true
		}
	}
	return "", false, false
}

func ebStrings(arg any) ([]string, bool) {
	switch x := arg.(type) {
	case string:
		return []string{x}, true
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

// ebWildcard matches v against a pattern where "*" is any run of characters
// and "\*" is a literal star.
func ebWildcard(pattern, v string) bool {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(pattern); i++ {
		switch {
		case pattern[i] == '\\' && i+1 < len(pattern) && pattern[i+1] == '*':
			cur.WriteByte('*')
			i++
		case pattern[i] == '*':
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(pattern[i])
		}
	}
	parts = append(parts, cur.String())
	if len(parts) == 1 {
		return v == parts[0]
	}
	if !strings.HasPrefix(v, parts[0]) {
		return false
	}
	v = v[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, mid := range parts[1 : len(parts)-1] {
		i := strings.Index(v, mid)
		if i < 0 {
			return false
		}
		v = v[i+len(mid):]
	}
	return len(v) >= len(last) && strings.HasSuffix(v, last)
}

func ebEqual(filter, value any) bool {
	switch f := filter.(type) {
	case nil:
		return value == nil
	case json.Number:
		n, ok := value.(json.Number)
		if !ok {
			return false
		}
		a, errA := f.Float64()
		b, errB := n.Float64()
		return errA == nil && errB == nil && a == b
	}
	return filter == value
}

func ebAnyString(values []any, test func(string) bool) bool {
	return slices.ContainsFunc(values, func(v any) bool {
		s, ok := v.(string)
		return ok && test(s)
	})
}

// ebRulesMatching reads the pattern of every loaded rule against the events
// of one resource: a rule whose pattern matches is counted, one it cannot
// read (or that matches every resource alike) leaves the answer a lower
// bound, and a rule with no pattern matches no event.
func ebRulesMatching(rules []resource.Resource, truncated bool, events []ebEvent) relatedRead {
	read := relatedRead{partial: truncated}
	for _, rule := range rules {
		pattern := rule.Fields["event_pattern"]
		if raw, ok := assertStruct[eventbridgetypes.Rule](rule.RawStruct); ok && raw.EventPattern != nil {
			pattern = *raw.EventPattern
		}
		if pattern == "" {
			continue
		}
		switch ebPatternMatches(pattern, events) {
		case ebMatch:
			read.ids = append(read.ids, rule.ID)
		case ebUnknown:
			read.partial = true
		}
	}
	return read
}

// ebStrs is the known values of a field.
func ebStrs(values ...string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// ecrEvents are the events ECR emits about a repository, from its samples:
// the repository is detail."repository-name" on every one, and resources
// holds its ARN on scan, pull-through-cache and replication events.
// https://docs.aws.amazon.com/AmazonECR/latest/userguide/ecr-eventbridge.html
func ecrEvents(name, arn string) []ebEvent {
	kind := func(detailType string, resources []any, free ...string) ebEvent {
		return ebEvent{
			known: map[string][]any{
				"source": ebStrs("aws.ecr"), "detail-type": ebStrs(detailType),
				"resources": resources, "detail.repository-name": ebStrs(name),
			},
			names: map[string]bool{"resources": len(resources) > 0, "detail.repository-name": true},
			free:  append(slices.Clone(ebEnvelope), free...),
		}
	}
	action := []string{"detail.result", "detail.image-digest", "detail.action-type", "detail.image-tag",
		"detail.target-storage-class", "detail.manifest-media-type", "detail.artifact-media-type", "detail.last-activated-at"}
	withARN := ebStrs(arn)
	return []ebEvent{
		kind("ECR Image Action", []any{}, action...),
		kind("ECR Referrer Action", []any{}, action...),
		kind("ECR Pull Through Cache Action", withARN, "detail.rule-version", "detail.sync-status",
			"detail.ecr-repository-prefix", "detail.upstream-registry-url", "detail.image-tag", "detail.image-digest"),
		kind("ECR Image Scan", withARN, "detail.scan-status", "detail.finding-severity-counts.",
			"detail.image-digest", "detail.image-tags"),
		kind("ECR Replication Action", withARN, "detail.result", "detail.image-digest", "detail.source-account",
			"detail.action-type", "detail.source-region", "detail.image-tag"),
	}
}

// s3Events are the events S3 sends EventBridge about a bucket: resources
// holds the bucket's ARN and detail.bucket.name its name.
// https://docs.aws.amazon.com/AmazonS3/latest/userguide/ev-events.html
func s3Events(bucket, arn string) []ebEvent {
	ev := ebEvent{
		known: map[string][]any{"source": ebStrs("aws.s3"), "detail.bucket.name": ebStrs(bucket)},
		names: map[string]bool{"resources": true, "detail.bucket.name": true},
		free: append(slices.Clone(ebEnvelope), "detail-type", "detail.version", "detail.event-version",
			"detail.bucket.aws-generated-tags.", "detail.object.", "detail.request-id", "detail.requester",
			"detail.source-ip-address", "detail.reason", "detail.deletion-type", "detail.restore-expiry-time",
			"detail.source-storage-class", "detail.destination-storage-class", "detail.destination-access-tier",
			"detail.object-annotation.", "detail.object-retention-event-data."),
	}
	ebKnownOrFree(&ev, "resources", arn)
	return []ebEvent{ev}
}

// ebKnownOrFree records the value of path, or, when the resource's own value
// is not known, the path as carried with a value this reading cannot decide.
func ebKnownOrFree(ev *ebEvent, path, value string) {
	if value == "" {
		ev.free = append(ev.free, path)
		return
	}
	ev.known[path] = ebStrs(value)
}

// ecsServiceEvents are the events ECS emits about a service: "ECS Service
// Action" names it in resources and its cluster in detail.clusterArn;
// "ECS Task State Change" names it in detail.group ("service:<name>"), with
// the task's own ARN in resources.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_events.html
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_task_events.html
func ecsServiceEvents(name, arn, clusterARN string) []ebEvent {
	action := ebEvent{
		known: map[string][]any{"source": ebStrs("aws.ecs"), "detail-type": ebStrs("ECS Service Action")},
		names: map[string]bool{"resources": true, "detail.clusterArn": true},
		free: append(slices.Clone(ebEnvelope), "detail.eventType", "detail.eventName", "detail.createdAt",
			"detail.capacityProviderArns", "detail.containerInstanceArns", "detail.reason"),
	}
	task := ebEvent{
		known: map[string][]any{
			"source": ebStrs("aws.ecs"), "detail-type": ebStrs("ECS Task State Change"),
			"detail.group": ebStrs("service:" + name),
		},
		names: map[string]bool{"detail.group": true, "detail.clusterArn": true},
		free: append(slices.Clone(ebEnvelope), "resources", "detail.attachments.", "detail.attributes.",
			"detail.availabilityZone", "detail.capacityProviderName", "detail.connectivity", "detail.connectivityAt",
			"detail.containerInstanceArn", "detail.containers.", "detail.cpu", "detail.createdAt",
			"detail.desiredStatus", "detail.enableExecuteCommand", "detail.ephemeralStorage.", "detail.launchType",
			"detail.lastStatus", "detail.memory", "detail.overrides.", "detail.platformVersion",
			"detail.pullStartedAt", "detail.pullStoppedAt", "detail.startedAt", "detail.startedBy",
			"detail.stoppingAt", "detail.stoppedAt", "detail.stoppedReason", "detail.stopCode", "detail.taskArn",
			"detail.taskDefinitionArn", "detail.updatedAt", "detail.version"),
	}
	ebKnownOrFree(&action, "resources", arn)
	ebKnownOrFree(&action, "detail.clusterArn", clusterARN)
	ebKnownOrFree(&task, "detail.clusterArn", clusterARN)
	return []ebEvent{action, task}
}
