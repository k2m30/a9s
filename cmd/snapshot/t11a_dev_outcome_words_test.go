package main

import (
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"sync"
	"testing"

	smithy "github.com/aws/smithy-go"
)

var emptyOutcome = regexp.MustCompile(`"[a-z_]*outcome":""`)

// Every read a capture records carries a word, whether the reads after the
// list were answered or refused: an empty word states nothing about what AWS
// returned.
func TestT11aDev_NoCaptureRecordsAnEmptyOutcome(t *testing.T) {
	for name, capture := range captures {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			for mode, cfg := range map[string]answerFunc{
				"answered": func(service, _ string, in any) (any, error) {
					out, ok := sdkOutputTypes()[reflect.TypeOf(in)]
					if !ok {
						return nil, &smithy.GenericAPIError{Code: "UnknownOperationException"}
					}
					return synthesize(service, out), nil
				},
				"refused after the list": func(service, _ string, in any) (any, error) {
					mu.Lock()
					calls++
					first := calls == 1
					mu.Unlock()
					out, ok := sdkOutputTypes()[reflect.TypeOf(in)]
					if !first || !ok {
						return nil, &smithy.GenericAPIError{Code: "AccessDeniedException"}
					}
					return synthesize(service, out), nil
				},
			} {
				data, err := capture(context.Background(), answeringConfig(cfg))
				if err != nil {
					continue
				}
				raw, err := json.Marshal(data)
				if err != nil {
					t.Fatalf("%s: marshal: %v", mode, err)
				}
				if emptyOutcome.Match(raw) {
					t.Errorf("%s: the capture records an empty outcome: %s", mode, raw)
				}
			}
		})
	}
}

// truthy sets every boolean of v to true and every number to 1, so no false
// or 0 in a capture's record can have come from the answer it was built from.
func truthy(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			truthy(v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				truthy(v.Field(i))
			}
		}
	case reflect.Slice:
		for i := range v.Len() {
			truthy(v.Index(i))
		}
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Float64:
		v.SetFloat(1)
	}
}

func statedZero(path string, v any, found *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			statedZero(path+"."+k, e, found)
		}
	case []any:
		for _, e := range x {
			statedZero(path+"[]", e, found)
		}
	case bool:
		if !x {
			*found = append(*found, path)
		}
	case float64:
		if x == 0 {
			*found = append(*found, path)
		}
	}
}

// computedFromTheList are the facts the capture works out from the list answer
// itself, so a refused later read does not bear on them.
var computedFromTheList = map[string]string{
	"ct-events.truncated":                             "set only when LookupEvents names a page past the cap",
	"redshift.clusters[].deferred_maintenance_active": "the listed deferred maintenance windows compared with the clock",
}

// A fact derived from a read that failed is unknown, not its zero value: with
// the list answered (every boolean true, every number 1) and every read after
// it refused, a false or a 0 in the record states more than AWS answered.
func TestT11aDev_AFailedReadLeavesItsFactsUnknown(t *testing.T) {
	for name, capture := range captures {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			data, err := capture(context.Background(), answeringConfig(func(service, _ string, in any) (any, error) {
				mu.Lock()
				calls++
				first := calls == 1
				mu.Unlock()
				out, ok := sdkOutputTypes()[reflect.TypeOf(in)]
				if !first || !ok {
					return nil, &smithy.GenericAPIError{Code: "AccessDeniedException"}
				}
				answer := synthesize(service, out)
				truthy(reflect.ValueOf(answer))
				return answer, nil
			}))
			if err != nil {
				return
			}
			raw, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			var decoded any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			var found []string
			statedZero(name, decoded, &found)
			slices.Sort(found)
			for _, p := range slices.Compact(found) {
				if computedFromTheList[p] != "" {
					continue
				}
				t.Errorf("%s is stated as false or 0 after the read it comes from was refused", p)
			}
		})
	}
}
