package unit_test

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo"
)

// t576FailingClientsFile is the generated wrapper set: one type per demo fake
// whose every SDK-shaped method asks a t576Injector before answering, so a
// test can refuse any single operation of any service without a hand-written
// fake per service.
const t576FailingClientsFile = "t576_failing_clients_gen_test.go"

// t576Call is one SDK call a wrapped client saw.
type t576Call struct {
	Service, Op string
	In, Out     any
	Failed      bool
}

// t576Injector decides, per call, whether a wrapped client answers from its
// fake or refuses with fail's error, and records every call either way.
type t576Injector struct {
	mu    sync.Mutex
	calls []t576Call
	fail  func(service, op string, in any) error
}

func (i *t576Injector) before(service, op string, in any) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.fail == nil {
		return nil
	}
	err := i.fail(service, op, in)
	if err != nil {
		i.calls = append(i.calls, t576Call{Service: service, Op: op, In: in, Failed: true})
	}
	return err
}

func (i *t576Injector) after(service, op string, in, out any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.calls = append(i.calls, t576Call{Service: service, Op: op, In: in, Out: out})
}

// t576APIError is the error the SDK hands back for a refused call: the
// operation's name on the outside, the service's code inside.
func t576APIError(service, op, code, message string) error {
	return &smithy.OperationError{
		ServiceID:     service,
		OperationName: op,
		Err:           &smithy.GenericAPIError{Code: code, Message: message, Fault: smithy.FaultClient},
	}
}

func t576Denied(service, op string) error {
	return t576APIError(service, op, "AccessDeniedException",
		"User: arn:aws:iam::123456789012:user/example-readonly is not authorized to perform: "+op)
}

func t576Throttled(service, op string) error {
	return t576APIError(service, op, "ThrottlingException", "Rate exceeded")
}

// t576DenyOp refuses every call of op and answers the rest from the fakes.
func t576DenyOp(op string) func(service, o string, in any) error {
	return func(service, o string, _ any) error {
		if o == op {
			return t576Denied(service, o)
		}
		return nil
	}
}

// t576ThrottleAlways throttles every call of op. A wrapped fake has no SDK
// retryer, so each call ends as the throttle, as a real read does once the
// retryer's attempts run out.
func t576ThrottleAlways(op string) func(service, o string, in any) error {
	return func(service, o string, _ any) error {
		if o == op {
			return t576Throttled(service, o)
		}
		return nil
	}
}

// t576Leaves is every non-empty string reachable from v, sorted: what a
// request names, or what a response hands back.
func t576Leaves(v any) []string {
	set := map[string]bool{}
	var walk func(reflect.Value, int)
	walk = func(rv reflect.Value, depth int) {
		if depth > 12 || !rv.IsValid() {
			return
		}
		switch rv.Kind() {
		case reflect.Pointer, reflect.Interface:
			if !rv.IsNil() {
				walk(rv.Elem(), depth+1)
			}
		case reflect.String:
			if s := rv.String(); s != "" {
				set[s] = true
			}
		case reflect.Struct:
			for i := range rv.NumField() {
				if rv.Type().Field(i).IsExported() {
					walk(rv.Field(i), depth+1)
				}
			}
		case reflect.Slice, reflect.Array:
			for i := range rv.Len() {
				walk(rv.Index(i), depth+1)
			}
		case reflect.Map:
			for _, k := range rv.MapKeys() {
				walk(k, depth+1)
				walk(rv.MapIndex(k), depth+1)
			}
		}
	}
	walk(reflect.ValueOf(v), 0)
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// t576GenerateFailingClients renders the wrapper set from the demo client set
// itself: every interface field holding a demo fake gets a wrapper carrying
// every SDK-shaped method of that fake, so a method reached by a narrow
// interface assertion is wrapped as well as the aggregate's.
func t576GenerateFailingClients() ([]byte, error) {
	clients := reflect.ValueOf(demo.NewServiceClients()).Elem()
	ct := clients.Type()
	imports := map[string]string{}
	alias := func(t reflect.Type) string {
		a := path.Base(t.PkgPath())
		imports[t.PkgPath()] = a
		return a + "." + t.Name()
	}
	var types, wraps bytes.Buffer
	var skipped []string
	for i := range ct.NumField() {
		f := ct.Field(i)
		if !f.IsExported() || f.Type.Kind() != reflect.Interface {
			continue
		}
		v := clients.Field(i)
		if v.IsNil() {
			continue
		}
		conc := v.Elem().Type()
		if conc.Kind() != reflect.Pointer || !strings.HasSuffix(conc.Elem().PkgPath(), "/core/demo/fakes") {
			continue
		}
		wrapper := "t576" + f.Name
		fmt.Fprintf(&types, "type %s struct {\n\tawsclient.%s\n\tfake any\n\tinj  *t576Injector\n}\n\n", wrapper, f.Type.Name())
		fmt.Fprintf(&wraps, "\tc.%s = %s{%s: c.%s, fake: c.%s, inj: inj}\n", f.Name, wrapper, f.Type.Name(), f.Name, f.Name)
		for m := range conc.NumMethod() {
			meth := conc.Method(m)
			mt := meth.Type
			if mt.NumIn() != 4 || mt.NumOut() != 2 || !mt.IsVariadic() ||
				mt.In(1).String() != "context.Context" || mt.In(2).Kind() != reflect.Pointer ||
				mt.Out(0).Kind() != reflect.Pointer || mt.Out(1).String() != "error" ||
				mt.In(3).Elem().Kind() != reflect.Func || mt.In(3).Elem().NumIn() != 1 {
				skipped = append(skipped, conc.String()+"."+meth.Name)
				continue
			}
			in := alias(mt.In(2).Elem())
			out := alias(mt.Out(0).Elem())
			opt := alias(mt.In(3).Elem().In(0).Elem())
			fmt.Fprintf(&types, "func (w %s) %s(ctx context.Context, in *%s, o ...func(*%s)) (*%s, error) {\n", wrapper, meth.Name, in, opt, out)
			fmt.Fprintf(&types, "\tif err := w.inj.before(%q, %q, in); err != nil {\n\t\treturn nil, err\n\t}\n", f.Name, meth.Name)
			fmt.Fprintf(&types, "\tres, err := w.fake.(interface {\n\t\t%s(context.Context, *%s, ...func(*%s)) (*%s, error)\n\t}).%s(ctx, in, o...)\n", meth.Name, in, opt, out, meth.Name)
			fmt.Fprintf(&types, "\tw.inj.after(%q, %q, in, res)\n\treturn res, err\n}\n\n", f.Name, meth.Name)
		}
	}
	if len(skipped) > 0 {
		return nil, fmt.Errorf("demo fake methods with no SDK call shape, which the wrappers cannot carry: %v", skipped)
	}
	var src bytes.Buffer
	src.WriteString("// Code generated by TestT576_FailingClientsGolden; DO NOT EDIT.\n// Regenerate with UPDATE_GOLDEN=1 go test ./tests/unit/ -run TestT576_FailingClientsGolden\n\npackage unit_test\n\nimport (\n\t\"context\"\n\n")
	paths := make([]string, 0, len(imports))
	for p := range imports {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		fmt.Fprintf(&src, "\t%s %q\n", imports[p], p)
	}
	src.WriteString("\n\tawsclient \"github.com/k2m30/a9s/v3/core/aws\"\n)\n\n")
	src.Write(types.Bytes())
	src.WriteString("// t576WrapClients puts a wrapper in front of every demo fake in c.\nfunc t576WrapClients(c *awsclient.ServiceClients, inj *t576Injector) {\n")
	src.Write(wraps.Bytes())
	src.WriteString("}\n")
	return format.Source(src.Bytes())
}

// The failure guard reaches every operation of every demo fake only while the
// wrapper set carries every method the fakes have; a fake method added since
// the last generation would answer unrefusable and unrecorded.
func TestT576_FailingClientsGolden(t *testing.T) {
	want, err := t576GenerateFailingClients()
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if werr := os.WriteFile(t576FailingClientsFile, want, 0o600); werr != nil {
			t.Fatal(werr)
		}
		return
	}
	got, err := os.ReadFile(t576FailingClientsFile)
	if err != nil {
		t.Fatalf("read %s: %v (run with UPDATE_GOLDEN=1 to create it)", t576FailingClientsFile, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale against the demo fakes; run UPDATE_GOLDEN=1 go test ./tests/unit/ -run TestT576_FailingClientsGolden", t576FailingClientsFile)
	}
}
