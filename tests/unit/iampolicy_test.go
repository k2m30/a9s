package unit

import (
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/iampolicy"
)

func mustParse(t *testing.T, doc string) iampolicy.Document {
	t.Helper()
	d, err := iampolicy.Parse(doc)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", doc, err)
	}
	return d
}

func sorted(ss []string) []string {
	out := make([]string, len(ss))
	copy(out, ss)
	sort.Strings(out)
	return out
}

func strSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Parse ---

func TestParse_Robustness(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"truncated json", "{", true},
		{"empty object", "{}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := iampolicy.Parse(tt.doc)
			if tt.wantErr && err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", tt.doc)
			}
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Parse(%q) unexpected error: %v", tt.doc, err)
				}
				if len(d.Statement) != 0 {
					t.Fatalf("Parse(%q) expected zero statements, got %d", tt.doc, len(d.Statement))
				}
			}
		})
	}
}

func TestParse_OddShapesDoNotPanic(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"principal is a number", `{"Statement":[{"Effect":"Allow","Principal":42,"Action":"s3:GetObject","Resource":"*"}]}`},
		{"action is an object", `{"Statement":[{"Effect":"Allow","Principal":"*","Action":{"x":1},"Resource":"*"}]}`},
		{"statement is a bare string", `{"Statement":"abc"}`},
		{"resource is null", `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":null}]}`},
		{"nested array in action", `{"Statement":[{"Effect":"Allow","Principal":"*","Action":[["s3:GetObject"]],"Resource":"*"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Parse(%q) panicked: %v", tt.doc, r)
				}
			}()
			iampolicy.Parse(tt.doc) //nolint:errcheck // only checking for panics here
		})
	}
}

func TestParse_URLEncoded(t *testing.T) {
	plain := `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*"}]}`
	encoded := `%7B%22Statement%22%3A%5B%7B%22Effect%22%3A%22Allow%22%2C%22Principal%22%3A%22%2A%22%2C%22Action%22%3A%22s3%3AGetObject%22%2C%22Resource%22%3A%22%2A%22%7D%5D%7D`

	dPlain := mustParse(t, plain)
	dEncoded := mustParse(t, encoded)

	exPlain := iampolicy.Evaluate(dPlain, "123456789012")
	exEncoded := iampolicy.Evaluate(dEncoded, "123456789012")

	if exPlain.Public != exEncoded.Public {
		t.Fatalf("Public mismatch: plain=%v encoded=%v", exPlain.Public, exEncoded.Public)
	}
	if !exEncoded.Public {
		t.Fatalf("expected URL-decoded document to evaluate Public=true")
	}
}

func TestParse_StatementObjectVsArray_SameVerdict(t *testing.T) {
	obj := `{"Statement":{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*"}}`
	arr := `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*"}]}`

	dObj := mustParse(t, obj)
	dArr := mustParse(t, arr)

	exObj := iampolicy.Evaluate(dObj, "123456789012")
	exArr := iampolicy.Evaluate(dArr, "123456789012")

	if !exObj.Public || !exArr.Public {
		t.Fatalf("expected both forms Public=true, got obj=%v arr=%v", exObj.Public, exArr.Public)
	}
	if !strSliceEqual(sorted(exObj.PublicActions), sorted(exArr.PublicActions)) {
		t.Fatalf("PublicActions mismatch: obj=%v arr=%v", exObj.PublicActions, exArr.PublicActions)
	}
}

// --- Evaluate: Public / Conditioned ---

func TestEvaluate_PublicAndConditioned(t *testing.T) {
	tests := []struct {
		name          string
		doc           string
		wantPublic    bool
		wantCond      bool
		wantPubAction []string // nil means "do not check"
	}{
		{
			name:          "wildcard principal string, array statement",
			doc:           `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*"}]}`,
			wantPublic:    true,
			wantPubAction: []string{"s3:GetObject"},
		},
		{
			name:          "AWS wildcard principal, object statement, multiple actions",
			doc:           `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*"}}`,
			wantPublic:    true,
			wantPubAction: []string{"sqs:ReceiveMessage", "sqs:SendMessage"},
		},
		{
			name:       "restrictive SourceAccount condition scopes it",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}}`,
			wantPublic: false,
			wantCond:   true,
		},
		{
			name:       "restrictive condition with array of values, still restrictive",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"StringEquals":{"aws:SourceAccount":["123456789012","210987654321"]}}}}`,
			wantPublic: false,
			wantCond:   true,
		},
		{
			name:       "wildcard SourceArn value is not restrictive",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"ArnLike":{"aws:SourceArn":"*"}}}}`,
			wantPublic: true,
		},
		{
			name:       "Bool SecureTransport condition is not restrictive",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"Bool":{"aws:SecureTransport":"true"}}}}`,
			wantPublic: true,
		},
		{
			name:       "SourceIp 0.0.0.0/0 is not restrictive",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"IpAddress":{"aws:SourceIp":"0.0.0.0/0"}}}}`,
			wantPublic: true,
		},
		{
			name:       "SourceIp scoped range is restrictive",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"IpAddress":{"aws:SourceIp":"10.0.0.0/8"}}}}`,
			wantPublic: false,
			wantCond:   true,
		},
		{
			name:       "PrincipalOrgID condition is restrictive",
			doc:        `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":["sqs:SendMessage","sqs:ReceiveMessage"],"Resource":"*","Condition":{"StringEquals":{"aws:PrincipalOrgID":"o-abc123"}}}}`,
			wantPublic: false,
			wantCond:   true,
		},
		{
			name:       "Deny with wildcard principal contributes nothing",
			doc:        `{"Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"*"}]}`,
			wantPublic: false,
			wantCond:   false,
		},
		{
			name:          "NotPrincipal is treated as public",
			doc:           `{"Statement":[{"Effect":"Allow","NotPrincipal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"s3:GetObject","Resource":"*"}]}`,
			wantPublic:    true,
			wantPubAction: []string{"s3:GetObject"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := mustParse(t, tt.doc)
			ex := iampolicy.Evaluate(d, "123456789012")
			if ex.Public != tt.wantPublic {
				t.Errorf("Public = %v, want %v", ex.Public, tt.wantPublic)
			}
			if ex.Conditioned != tt.wantCond {
				t.Errorf("Conditioned = %v, want %v", ex.Conditioned, tt.wantCond)
			}
			if tt.wantPublic && tt.wantCond {
				t.Errorf("test case invalid: Public and Conditioned both true")
			}
			if tt.wantPubAction != nil {
				got := sorted(ex.PublicActions)
				want := sorted(tt.wantPubAction)
				if !strSliceEqual(got, want) {
					t.Errorf("PublicActions = %v, want %v", got, want)
				}
			}
			if !tt.wantPublic && ex.PublicActions != nil {
				t.Errorf("PublicActions = %v, want nil when !Public", ex.PublicActions)
			}
		})
	}
}

func TestEvaluate_PrincipalWithBothAWSAndService_NoFalseWildcard(t *testing.T) {
	doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"999888777666","Service":"ec2.amazonaws.com"},"Action":"s3:GetObject","Resource":"*"}}`
	d := mustParse(t, doc)
	ex := iampolicy.Evaluate(d, "123456789012")
	if ex.Public {
		t.Errorf("Public = true, want false: a non-wildcard AWS principal plus a Service entry must not be treated as public")
	}
	if ex.Conditioned {
		t.Errorf("Conditioned = true, want false")
	}
	want := []string{"999888777666"}
	if !strSliceEqual(sorted(ex.CrossAccount), want) {
		t.Errorf("CrossAccount = %v, want %v", ex.CrossAccount, want)
	}
}

// --- Evaluate: CrossAccount ---

func TestEvaluate_CrossAccount(t *testing.T) {
	doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:root","arn:aws:iam::210987654321:role/x","999888777666"]},"Action":"s3:GetObject","Resource":"*"}}`

	t.Run("excludes own account", func(t *testing.T) {
		d := mustParse(t, doc)
		ex := iampolicy.Evaluate(d, "123456789012")
		want := []string{"210987654321", "999888777666"}
		if !strSliceEqual(sorted(ex.CrossAccount), want) {
			t.Errorf("CrossAccount = %v, want %v", ex.CrossAccount, want)
		}
	})

	t.Run("empty ownAccount reports every named account", func(t *testing.T) {
		d := mustParse(t, doc)
		ex := iampolicy.Evaluate(d, "")
		want := []string{"123456789012", "210987654321", "999888777666"}
		if !strSliceEqual(sorted(ex.CrossAccount), want) {
			t.Errorf("CrossAccount = %v, want %v", ex.CrossAccount, want)
		}
	})

	t.Run("service principals contribute nothing", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}}`)
		ex := iampolicy.Evaluate(d, "123456789012")
		if len(ex.CrossAccount) != 0 {
			t.Errorf("CrossAccount = %v, want empty", ex.CrossAccount)
		}
	})
}

// --- AllowsAction / IsAdmin / PrivilegeEscalation ---

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func TestIsAdmin_And_PrivilegeEscalation(t *testing.T) {
	t.Run("Action:* Resource:* is admin and OverPermissiveIAM", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":"*","Resource":"*"}}`)
		if !d.IsAdmin() {
			t.Errorf("IsAdmin() = false, want true")
		}
		combos := d.PrivilegeEscalation()
		if len(combos) == 0 {
			t.Fatalf("PrivilegeEscalation() empty, want non-empty")
		}
		if !containsStr(combos, "OverPermissiveIAM") {
			t.Errorf("PrivilegeEscalation() = %v, want to contain OverPermissiveIAM", combos)
		}
	})

	t.Run("iam:* allows iam:CreateAccessKey and matches its combo", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":"iam:*","Resource":"*"}}`)
		if !d.AllowsAction("iam:CreateAccessKey") {
			t.Errorf("AllowsAction(iam:CreateAccessKey) = false, want true")
		}
		combos := d.PrivilegeEscalation()
		if !containsStr(combos, "OverPermissiveIAM") {
			t.Errorf("PrivilegeEscalation() = %v, want to contain OverPermissiveIAM", combos)
		}
		if !containsStr(combos, "iam:CreateAccessKey") {
			t.Errorf("PrivilegeEscalation() = %v, want to contain iam:CreateAccessKey", combos)
		}
	})

	t.Run("PassRole+CreateLambda+Invoke combo present, PassRole+EC2 absent", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":["iam:PassRole","lambda:CreateFunction","lambda:InvokeFunction"],"Resource":"*"}}`)
		combos := d.PrivilegeEscalation()
		if !containsStr(combos, "PassRole+CreateLambda+Invoke") {
			t.Errorf("PrivilegeEscalation() = %v, want to contain PassRole+CreateLambda+Invoke", combos)
		}
		if containsStr(combos, "PassRole+EC2") {
			t.Errorf("PrivilegeEscalation() = %v, want NOT to contain PassRole+EC2", combos)
		}
	})

	t.Run("iam:Put* glob allows PutUserPolicy, not CreateUser, matches IAMPut", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":"iam:Put*","Resource":"*"}}`)
		if !d.AllowsAction("iam:PutUserPolicy") {
			t.Errorf("AllowsAction(iam:PutUserPolicy) = false, want true")
		}
		if d.AllowsAction("iam:CreateUser") {
			t.Errorf("AllowsAction(iam:CreateUser) = true, want false")
		}
		if !containsStr(d.PrivilegeEscalation(), "IAMPut") {
			t.Errorf("PrivilegeEscalation() = %v, want to contain IAMPut", d.PrivilegeEscalation())
		}
	})

	t.Run("Deny iam:* wins over Allow *, IsAdmin looks at Allow only", func(t *testing.T) {
		d := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"*","Resource":"*"},{"Effect":"Deny","Action":"iam:*","Resource":"*"}]}`)
		if d.AllowsAction("iam:CreateUser") {
			t.Errorf("AllowsAction(iam:CreateUser) = true, want false (Deny wins)")
		}
		if !d.AllowsAction("s3:GetObject") {
			t.Errorf("AllowsAction(s3:GetObject) = false, want true")
		}
		if !d.IsAdmin() {
			t.Errorf("IsAdmin() = false, want true (looks at the Allow statement only)")
		}
	})

	t.Run("Deny with NotAction removes everything except the excluded pattern", func(t *testing.T) {
		d := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"*","Resource":"*"},{"Effect":"Deny","NotAction":"s3:*","Resource":"*"}]}`)
		if !d.AllowsAction("s3:GetObject") {
			t.Errorf("AllowsAction(s3:GetObject) = false, want true (Deny's NotAction matches s3:*, so this Deny does not apply to it)")
		}
		if d.AllowsAction("iam:CreateUser") {
			t.Errorf("AllowsAction(iam:CreateUser) = true, want false (Deny's NotAction does not match, so the Deny applies)")
		}
	})

	t.Run("Allow NotAction:s3:* allows everything but s3", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","NotAction":"s3:*","Resource":"*"}}`)
		if !d.AllowsAction("iam:CreateUser") {
			t.Errorf("AllowsAction(iam:CreateUser) = false, want true")
		}
		if d.AllowsAction("s3:GetObject") {
			t.Errorf("AllowsAction(s3:GetObject) = true, want false")
		}
	})

	t.Run("read-only policy has no privilege escalation and is not admin", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":["s3:Get*","ec2:Describe*"],"Resource":"*"}}`)
		if len(d.PrivilegeEscalation()) != 0 {
			t.Errorf("PrivilegeEscalation() = %v, want empty", d.PrivilegeEscalation())
		}
		if d.IsAdmin() {
			t.Errorf("IsAdmin() = true, want false")
		}
	})

	t.Run("action matching is case-insensitive", func(t *testing.T) {
		d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":"IAM:createaccesskey","Resource":"*"}}`)
		if !d.AllowsAction("iam:CreateAccessKey") {
			t.Errorf("AllowsAction(iam:CreateAccessKey) = false, want true (case-insensitive match)")
		}
	})
}

// --- HasServicePrincipalWithoutSourceScope ---

func TestHasServicePrincipalWithoutSourceScope(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{
			name: "unscoped lambda trust",
			doc:  `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}}`,
			want: []string{"lambda"},
		},
		{
			name: "scoped by SourceAccount",
			doc:  `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"123456789012"}}}}`,
			want: nil,
		},
		{
			name: "scoped by SourceArn",
			doc:  `{"Statement":{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"ArnLike":{"aws:SourceArn":"arn:aws:lambda:us-east-1:123456789012:function:*"}}}}`,
			want: nil,
		},
		{
			name: "service not on the confused-deputy list",
			doc:  `{"Statement":{"Effect":"Allow","Principal":{"Service":"foo.amazonaws.com"},"Action":"sts:AssumeRole"}}`,
			want: nil,
		},
		{
			name: "AWS account principal is not a service principal",
			doc:  `{"Statement":{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"sts:AssumeRole"}}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := mustParse(t, tt.doc)
			got := d.HasServicePrincipalWithoutSourceScope()
			if !strSliceEqual(sorted(got), sorted(tt.want)) {
				t.Errorf("HasServicePrincipalWithoutSourceScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- verify round: adversarial attacks against the landed engine ---

// FINDING 1: a real IAM document, when percent-encoded by a real-world
// encoder that leaves '+' unescaped (many do, since '+' needs no escaping
// outside a form body), has any literal '+' in its content silently turned
// into a space. url.QueryUnescape treats '+' as application/x-www-form-urlencoded
// space, which AWS's percent-encoded policy documents are not: AWS encodes a
// real space as %20, so a literal '+' surviving the encoder means a literal
// '+' in the source, not a space. iampolicy.Parse (policy.go:60-63) uses
// url.QueryUnescape and corrupts it.
func TestParse_URLEncoded_LiteralPlusIsNotSpace(t *testing.T) {
	plain := `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"*","Condition":{"StringEquals":{"aws:PrincipalTag/Team":"a+b"}}}]}`
	encoded := strings.ReplaceAll(url.QueryEscape(plain), "%2B", "+")

	d, err := iampolicy.Parse(encoded)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", encoded, err)
	}
	if len(d.Statement) != 1 {
		t.Fatalf("Parse(%q) expected 1 statement, got %d", encoded, len(d.Statement))
	}
	got := d.Statement[0].Condition["StringEquals"]["aws:PrincipalTag/Team"]
	want := []string{"a+b"}
	if !strSliceEqual(got, want) {
		t.Errorf("Condition value = %v, want %v (a literal '+' in a percent-encoded document must not become a space)", got, want)
	}
}

// FINDING 2: a statement carrying both Principal and NotPrincipal (invalid
// under AWS's own policy grammar, but not guaranteed to be rejected for an
// already-attached or hand-edited resource policy read back via a read-only
// API) takes the NotPrincipal branch unconditionally (policy.go:85-90) and
// silently discards the real Principal's AWS entries — evaluate.go:46-48
// then `continue`s past CrossAccount aggregation for the whole statement.
// A concrete cross-account principal must not vanish from CrossAccount just
// because a NotPrincipal key also happens to be present.
func TestEvaluate_PrincipalAndNotPrincipalBothPresent_CrossAccountNotDropped(t *testing.T) {
	doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"210987654321"},"NotPrincipal":{"AWS":"999888777666"},"Action":"s3:GetObject","Resource":"*"}}`
	d := mustParse(t, doc)
	ex := iampolicy.Evaluate(d, "123456789012")
	if !containsStr(ex.CrossAccount, "210987654321") {
		t.Errorf("CrossAccount = %v, want it to contain 210987654321 (the real Principal, not silently dropped because NotPrincipal was also present)", ex.CrossAccount)
	}
}

// --- verify round: correctly-handled edge cases, pinned against regression ---

func TestEvaluate_ForAnyValuePrefixedOperator_StillRestrictive(t *testing.T) {
	doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"s3:GetObject","Resource":"*","Condition":{"ForAnyValue:StringEquals":{"aws:PrincipalOrgID":"o-abc123"}}}}`
	d := mustParse(t, doc)
	ex := iampolicy.Evaluate(d, "123456789012")
	if ex.Public {
		t.Errorf("Public = true, want false: a ForAnyValue:-prefixed operator carrying a real PrincipalOrgID must still scope the wildcard principal")
	}
	if !ex.Conditioned {
		t.Errorf("Conditioned = false, want true")
	}
}

func TestEvaluate_SourceIpArrayMixingAnyAndScoped_NotRestrictive(t *testing.T) {
	doc := `{"Statement":{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"s3:GetObject","Resource":"*","Condition":{"IpAddress":{"aws:SourceIp":["0.0.0.0/0","10.0.0.0/8"]}}}}`
	d := mustParse(t, doc)
	ex := iampolicy.Evaluate(d, "123456789012")
	if !ex.Public {
		t.Errorf("Public = false, want true: 0.0.0.0/0 anywhere in a multi-value SourceIp condition matches every caller, so the /8 alongside it does not narrow anything")
	}
	if ex.Conditioned {
		t.Errorf("Conditioned = true, want false")
	}
}

func TestAllowsAction_PartiallyOverlappingDenyDoesNotShadowUnrelatedActions(t *testing.T) {
	d := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"iam:*","Resource":"*"},{"Effect":"Deny","Action":"iam:Delete*","Resource":"*"}]}`)
	if !d.AllowsAction("iam:CreateAccessKey") {
		t.Errorf("AllowsAction(iam:CreateAccessKey) = false, want true: a Deny on iam:Delete* must not shadow an unrelated iam:* grant")
	}
	if d.AllowsAction("iam:DeleteUser") {
		t.Errorf("AllowsAction(iam:DeleteUser) = true, want false: the Deny does cover this one")
	}
}

func TestPrivilegeEscalation_LowercaseActionsStillMatchCombo(t *testing.T) {
	d := mustParse(t, `{"Statement":{"Effect":"Allow","Action":["iam:passrole","lambda:createfunction","lambda:invokefunction"],"Resource":"*"}}`)
	if !containsStr(d.PrivilegeEscalation(), "PassRole+CreateLambda+Invoke") {
		t.Errorf("PrivilegeEscalation() = %v, want to contain PassRole+CreateLambda+Invoke even with all-lowercase action names", d.PrivilegeEscalation())
	}
}
