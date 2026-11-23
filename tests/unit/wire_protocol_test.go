package unit

// Wire fakes answer an operation in the protocol the SDK client sent it in,
// so a service that moves protocol in an SDK release changes nothing here
// but this file.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/smithy-go/encoding/cbor"
)

func wireIsCBOR(req *http.Request) bool {
	return req.Header.Get("Smithy-Protocol") == "rpc-v2-cbor"
}

// WireRequest returns the operation and the decoded input of an AWS JSON,
// Smithy RPCv2 CBOR or AWS Query request; a query input is its form values.
func WireRequest(req *http.Request, body []byte) (op string, in map[string]any) {
	if target := req.Header.Get("X-Amz-Target"); target != "" {
		_, op, _ = strings.Cut(target, ".")
		_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body carries no input
		return op, in
	}
	if wireIsCBOR(req) {
		_, op, _ = strings.Cut(req.URL.Path, "/operation/")
		if v, err := cbor.Decode(body); err == nil {
			in, _ = wireFromCBOR(v).(map[string]any)
		}
		return op, in
	}
	form, err := url.ParseQuery(string(body))
	if err != nil || form.Get("Action") == "" {
		return "", nil
	}
	in = map[string]any{}
	for k := range form {
		in[k] = form.Get(k)
	}
	return form.Get("Action"), in
}

func WireAnswer(req *http.Request, out any) *http.Response {
	if wireIsCBOR(req) {
		return wireResponse(200, cbor.Encode(wireToCBOR(out)), true)
	}
	b, _ := json.Marshal(out) //nolint:errcheck // test-built maps always marshal
	return wireResponse(200, b, false)
}

func WireError(req *http.Request, status int, code, message string) *http.Response {
	if wireIsCBOR(req) {
		return wireResponse(status, cbor.Encode(cbor.Map{"__type": cbor.String(code), "message": cbor.String(message)}), true)
	}
	b, _ := json.Marshal(map[string]string{"__type": code, "message": message}) //nolint:errcheck // test-built maps always marshal
	resp := wireResponse(status, b, false)
	resp.Header.Set("X-Amzn-Errortype", code)
	return resp
}

// WireEmptyBody is the body of an answer with no output members, in the
// protocol req was sent in.
func WireEmptyBody(req *http.Request, body []byte) string {
	if wireIsCBOR(req) {
		return string(cbor.Encode(cbor.Map{}))
	}
	if req.Header.Get("X-Amz-Target") == "" {
		if op, _ := WireRequest(req, body); op != "" {
			return "<" + op + "Response><" + op + "Result/></" + op + "Response>"
		}
	}
	return `{}`
}

func wireResponse(status int, body []byte, isCBOR bool) *http.Response {
	header := http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}}
	if isCBOR {
		header = http.Header{"Content-Type": []string{"application/cbor"}, "Smithy-Protocol": []string{"rpc-v2-cbor"}}
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(bytes.NewReader(body))}
}

// wireToCBOR converts a JSON-shaped Go value to the CBOR tree the SDK
// decodes: whole numbers become integers, other numbers floats.
func wireToCBOR(v any) cbor.Value {
	b, _ := json.Marshal(v) //nolint:errcheck // test-built maps always marshal
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var x any
	_ = dec.Decode(&x) //nolint:errcheck // decodes what json.Marshal just wrote
	return wireCBORValue(x)
}

func wireCBORValue(x any) cbor.Value {
	switch x := x.(type) {
	case map[string]any:
		m := cbor.Map{}
		for k, v := range x {
			m[k] = wireCBORValue(v)
		}
		return m
	case []any:
		l := make(cbor.List, len(x))
		for i, v := range x {
			l[i] = wireCBORValue(v)
		}
		return l
	case string:
		return cbor.String(x)
	case bool:
		return cbor.Bool(x)
	case json.Number:
		if n, err := x.Int64(); err == nil {
			if n < 0 {
				return cbor.NegInt(-n)
			}
			return cbor.Uint(n)
		}
		f, _ := x.Float64() //nolint:errcheck // json.Number is always a valid float
		return cbor.Float64(f)
	}
	return &cbor.Nil{}
}

func wireFromCBOR(v cbor.Value) any {
	switch v := v.(type) {
	case cbor.Map:
		m := map[string]any{}
		for k, e := range v {
			m[k] = wireFromCBOR(e)
		}
		return m
	case cbor.List:
		l := make([]any, len(v))
		for i, e := range v {
			l[i] = wireFromCBOR(e)
		}
		return l
	case cbor.String:
		return string(v)
	case cbor.Bool:
		return bool(v)
	case cbor.Uint:
		return float64(v)
	}
	return nil
}

func TestWireToCBORRoundTrip(t *testing.T) {
	in := map[string]any{"name": "acme", "version": 3, "neg": -2, "ratio": 0.5, "ok": true, "tags": []string{"a"}, "nested": map[string]any{"k": "v"}}
	got, err := cbor.Decode(cbor.Encode(wireToCBOR(in)))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := got.(cbor.Map)
	if m["version"] != cbor.Uint(3) || m["neg"] != cbor.NegInt(2) || m["ratio"] != cbor.Float64(0.5) || m["ok"] != cbor.Bool(true) {
		t.Errorf("scalars = %#v", m)
	}
	back, _ := wireFromCBOR(got).(map[string]any)
	nested, _ := back["nested"].(map[string]any)
	if back["name"] != "acme" || nested["k"] != "v" {
		t.Errorf("round trip = %#v", back)
	}
}
