package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// A read's bound is declared at its call site: every page, or only the first
// (newest-first lists and MaxResults 1 probes).
const (
	allPages = 0
	onePage  = 1
)

// resumePairs are the (output, input) members an AWS list call resumes
// through, in the order they are tried: an output member sent back in the
// input member. A member already paired is not paired again, so an output
// that echoes the request's Marker and carries NextMarker (Route 53
// ListHostedZones) resumes through NextMarker.
var resumePairs = [][2]string{
	{"NextToken", "NextToken"},
	{"NextMarker", "NextMarker"},
	{"NextMarker", "Marker"},
	{"Marker", "Marker"},
	{"Position", "Position"},
	{"NextRecordName", "StartRecordName"},
	{"NextRecordType", "StartRecordType"},
	{"NextRecordIdentifier", "StartRecordIdentifier"},
}

// pageAll calls call with in until the answer names no further page, or until
// pages pages have been read (allPages reads them all), and returns the items
// read. A page that fails ends the walk with its error and the items of the
// pages before it. The SDK paginators and pageAll are the collector's two
// walks; pageAll is the only code that reads or sets a token by hand.
func pageAll[I, O, T any](ctx context.Context, in *I, call func(context.Context, *I) (*O, error), items func(*O) []T, pages int) ([]T, error) {
	resume, unresolved := resumeFields(reflect.TypeFor[I](), reflect.TypeFor[O]())
	if len(resume) == 0 || len(unresolved) > 0 {
		panic(fmt.Sprintf("pageAll: %s and %s: no resume pair for %v", reflect.TypeFor[I](), reflect.TypeFor[O](), unresolved))
	}
	input := reflect.ValueOf(in).Elem()

	var all []T
	for read := 1; ; read++ {
		out, err := call(ctx, in)
		if err != nil {
			return all, err
		}
		all = append(all, items(out)...)
		output := reflect.ValueOf(out).Elem()
		more, advanced := false, false
		for from, to := range resume {
			next := output.FieldByName(from)
			more = more || next.Kind() == reflect.Pointer && !next.IsNil() && next.Elem().String() != ""
			advanced = advanced || !reflect.DeepEqual(next.Interface(), input.FieldByName(to).Interface())
			input.FieldByName(to).Set(next)
		}
		if !more || read == pages {
			return all, nil
		}
		if !advanced {
			return all, fmt.Errorf("pageAll: %s answered the cursor it was sent", reflect.TypeFor[O]())
		}
	}
}

// resumeFields maps each output member that names the next page to the input
// member it is sent back in, from resumePairs, and returns the output's
// token-like members it leaves unpaired. IsTruncated and an echo of a paired
// input member are answered by the pair. A further page exists while a
// *string member of the pairs is set: Route 53 sets NextRecordName whenever
// the list is truncated.
func resumeFields(in, out reflect.Type) (resume map[string]string, unresolved []string) {
	resume = map[string]string{}
	fed := map[string]bool{}
	for _, pair := range resumePairs {
		fo, outOK := out.FieldByName(pair[0])
		fi, inOK := in.FieldByName(pair[1])
		if outOK && inOK && fo.Type == fi.Type && resume[pair[0]] == "" && !fed[pair[1]] {
			resume[pair[0]] = pair[1]
			fed[pair[1]] = true
		}
	}
	for fo := range out.Fields() {
		name := fo.Name
		tokenLike := strings.HasPrefix(name, "Next") || strings.HasSuffix(name, "Marker") || strings.HasPrefix(name, "LastEvaluated") ||
			name == "Position" || name == "ContinuationToken" || name == "PaginationToken" || name == "StartingToken" || name == "IsTruncated"
		answered := resume[name] != "" || fed[name] || name == "IsTruncated" && len(resume) > 0
		if tokenLike && !answered {
			unresolved = append(unresolved, name)
		}
	}
	return resume, unresolved
}
