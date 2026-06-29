package app

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"gopkg.in/yaml.v3"
)

// resourceYAMLLines marshals r to plain YAML text (no ANSI coloring) and
// returns the individual lines. Mirrors the source that YAMLModel.RawContent
// uses — RawStruct when present, resource.Fields as fallback — so the
// content is equivalent to what the TUI YAML screen shows (minus syntax color).
//
// Returns a one-element slice with a "No YAML data available" notice when
// the resource carries neither RawStruct nor Fields.
func resourceYAMLLines(r resource.Resource) []string {
	var data []byte
	var err error
	if r.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(r.Fields) > 0 {
		data, err = yaml.Marshal(r.Fields)
	}
	if err != nil || len(data) == 0 {
		return []string{"  No YAML data available"}
	}
	raw := strings.TrimRight(string(data), "\n")
	return strings.Split(raw, "\n")
}

// resourceJSONLines marshals r to indented plain JSON text (no ANSI coloring)
// and returns the individual lines. Mirrors JSONModel.RawContent — RawStruct
// when present, resource.Fields as fallback.
//
// For the JSON case we also try a roundtrip through jsonyaml.TryJSONToYAMLLines
// to validate the JSON is well-formed; the actual output is the MarshalIndent
// string split by newline, which is always valid when MarshalIndent succeeds.
//
// Returns a one-element slice with a "No JSON data available" notice when the
// resource carries neither RawStruct nor Fields.
func resourceJSONLines(r resource.Resource) []string {
	var data []byte
	var err error
	if r.RawStruct != nil {
		data, err = json.MarshalIndent(r.RawStruct, "", "  ")
	} else if len(r.Fields) > 0 {
		data, err = json.MarshalIndent(r.Fields, "", "  ")
	}
	if err != nil || len(data) == 0 {
		return []string{"  No JSON data available"}
	}
	raw := strings.TrimRight(string(data), "\n")
	return strings.Split(raw, "\n")
}

