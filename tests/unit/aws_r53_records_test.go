package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Route 53 DNS Records fetcher tests
// ---------------------------------------------------------------------------

// TestFetchR53Records_Basic verifies parsing of A (multi-value), CNAME, and MX
// records with correct ID, Name, Status, Fields (name, type, ttl, values), and
// RawStruct.
func TestFetchR53Records_Basic(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("example.com."),
						Type: r53types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("1.2.3.4")},
							{Value: aws.String("5.6.7.8")},
						},
					},
					{
						Name: aws.String("www.example.com."),
						Type: r53types.RRTypeCname,
						TTL:  aws.Int64(3600),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("example.com.")},
						},
					},
					{
						Name: aws.String("example.com."),
						Type: r53types.RRTypeMx,
						TTL:  aws.Int64(600),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("10 mail.example.com.")},
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/Z123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	// Verify A record
	t.Run("A_record_ID", func(t *testing.T) {
		if resources[0].ID != "example.com.|A" {
			t.Errorf("ID: expected %q, got %q", "example.com.|A", resources[0].ID)
		}
	})

	t.Run("A_record_Name", func(t *testing.T) {
		if resources[0].Name != "example.com." {
			t.Errorf("Name: expected %q, got %q", "example.com.", resources[0].Name)
		}
	})

	t.Run("A_record_Status", func(t *testing.T) {
		if resources[0].Status != "A" {
			t.Errorf("Status: expected %q, got %q", "A", resources[0].Status)
		}
	})

	t.Run("A_record_fields", func(t *testing.T) {
		r := resources[0]
		if r.Fields["name"] != "example.com." {
			t.Errorf("Fields[name]: expected %q, got %q", "example.com.", r.Fields["name"])
		}
		if r.Fields["type"] != "A" {
			t.Errorf("Fields[type]: expected %q, got %q", "A", r.Fields["type"])
		}
		if r.Fields["ttl"] != "300" {
			t.Errorf("Fields[ttl]: expected %q, got %q", "300", r.Fields["ttl"])
		}
		if r.Fields["values"] != "1.2.3.4, 5.6.7.8" {
			t.Errorf("Fields[values]: expected %q, got %q", "1.2.3.4, 5.6.7.8", r.Fields["values"])
		}
	})

	t.Run("A_record_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(r53types.ResourceRecordSet)
		if !ok {
			t.Fatalf("RawStruct should be r53types.ResourceRecordSet, got %T", r.RawStruct)
		}
		if raw.Name == nil || *raw.Name != "example.com." {
			t.Errorf("RawStruct.Name: expected %q", "example.com.")
		}
	})

	// Verify CNAME record
	t.Run("CNAME_record", func(t *testing.T) {
		r := resources[1]
		if r.ID != "www.example.com.|CNAME" {
			t.Errorf("ID: expected %q, got %q", "www.example.com.|CNAME", r.ID)
		}
		if r.Name != "www.example.com." {
			t.Errorf("Name: expected %q, got %q", "www.example.com.", r.Name)
		}
		if r.Status != "CNAME" {
			t.Errorf("Status: expected %q, got %q", "CNAME", r.Status)
		}
		if r.Fields["ttl"] != "3600" {
			t.Errorf("Fields[ttl]: expected %q, got %q", "3600", r.Fields["ttl"])
		}
		if r.Fields["values"] != "example.com." {
			t.Errorf("Fields[values]: expected %q, got %q", "example.com.", r.Fields["values"])
		}
	})

	// Verify MX record
	t.Run("MX_record", func(t *testing.T) {
		r := resources[2]
		if r.ID != "example.com.|MX" {
			t.Errorf("ID: expected %q, got %q", "example.com.|MX", r.ID)
		}
		if r.Status != "MX" {
			t.Errorf("Status: expected %q, got %q", "MX", r.Status)
		}
		if r.Fields["ttl"] != "600" {
			t.Errorf("Fields[ttl]: expected %q, got %q", "600", r.Fields["ttl"])
		}
		if r.Fields["values"] != "10 mail.example.com." {
			t.Errorf("Fields[values]: expected %q, got %q", "10 mail.example.com.", r.Fields["values"])
		}
	})

	// Verify all records have required fields
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"name", "type", "ttl", "values"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchR53Records_AliasRecord verifies that alias records (AliasTarget
// instead of ResourceRecords) produce the correct values and empty TTL.
func TestFetchR53Records_AliasRecord(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("cdn.example.com."),
						Type: r53types.RRTypeA,
						AliasTarget: &r53types.AliasTarget{
							DNSName:              aws.String("target.example.com."),
							HostedZoneId:         aws.String("Z2FDTNDATAQYW2"),
							EvaluateTargetHealth: true,
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZALIAS")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("alias_values", func(t *testing.T) {
		expected := "ALIAS: target.example.com."
		if r.Fields["values"] != expected {
			t.Errorf("Fields[values]: expected %q, got %q", expected, r.Fields["values"])
		}
	})

	t.Run("alias_ttl_empty", func(t *testing.T) {
		if r.Fields["ttl"] != "" {
			t.Errorf("Fields[ttl]: expected empty string for alias, got %q", r.Fields["ttl"])
		}
	})

	t.Run("alias_id", func(t *testing.T) {
		if r.ID != "cdn.example.com.|A" {
			t.Errorf("ID: expected %q, got %q", "cdn.example.com.|A", r.ID)
		}
	})

	t.Run("alias_name_and_type", func(t *testing.T) {
		if r.Name != "cdn.example.com." {
			t.Errorf("Name: expected %q, got %q", "cdn.example.com.", r.Name)
		}
		if r.Fields["type"] != "A" {
			t.Errorf("Fields[type]: expected %q, got %q", "A", r.Fields["type"])
		}
	})

	t.Run("alias_status", func(t *testing.T) {
		if r.Status != "A" {
			t.Errorf("Status: expected %q, got %q", "A", r.Status)
		}
	})

	t.Run("alias_rawstruct", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(r53types.ResourceRecordSet)
		if !ok {
			t.Fatalf("RawStruct should be r53types.ResourceRecordSet, got %T", r.RawStruct)
		}
		if raw.AliasTarget == nil {
			t.Fatal("RawStruct.AliasTarget should not be nil")
		}
		if raw.AliasTarget.DNSName == nil || *raw.AliasTarget.DNSName != "target.example.com." {
			t.Errorf("RawStruct.AliasTarget.DNSName: expected %q", "target.example.com.")
		}
	})
}

// TestFetchR53Records_Pagination verifies that paginated responses (IsTruncated
// + NextRecordName/NextRecordType) are followed and all records collected.
func TestFetchR53Records_Pagination(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated:    true,
				NextRecordName: aws.String("page2.example.com."),
				NextRecordType: r53types.RRTypeA,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("first.example.com."),
						Type: r53types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("1.1.1.1")},
						},
					},
					{
						Name: aws.String("second.example.com."),
						Type: r53types.RRTypeCname,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("first.example.com.")},
						},
					},
				},
			},
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: aws.String("page2.example.com."),
						Type: r53types.RRTypeA,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("2.2.2.2")},
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZPAGE")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first_record", func(t *testing.T) {
		if resources[0].Name != "first.example.com." {
			t.Errorf("resources[0].Name: expected %q, got %q", "first.example.com.", resources[0].Name)
		}
	})

	t.Run("page1_second_record", func(t *testing.T) {
		if resources[1].Name != "second.example.com." {
			t.Errorf("resources[1].Name: expected %q, got %q", "second.example.com.", resources[1].Name)
		}
	})

	t.Run("page2_record", func(t *testing.T) {
		if resources[2].Name != "page2.example.com." {
			t.Errorf("resources[2].Name: expected %q, got %q", "page2.example.com.", resources[2].Name)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})
}

// TestFetchR53Records_SetIdentifier verifies that records with a SetIdentifier
// (weighted/latency/failover routing) include the identifier in the ID, while
// records without a SetIdentifier do not.
func TestFetchR53Records_SetIdentifier(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated: false,
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name:          aws.String("weighted.example.com."),
						Type:          r53types.RRTypeA,
						TTL:           aws.Int64(60),
						SetIdentifier: aws.String("us-east-1"),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("10.0.0.1")},
						},
					},
					{
						Name:          aws.String("weighted.example.com."),
						Type:          r53types.RRTypeA,
						TTL:           aws.Int64(60),
						SetIdentifier: aws.String("us-west-2"),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("10.0.1.1")},
						},
					},
					{
						Name: aws.String("simple.example.com."),
						Type: r53types.RRTypeCname,
						TTL:  aws.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: aws.String("target.example.com.")},
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZWEIGHT")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("id_with_set_identifier_east", func(t *testing.T) {
		expected := "weighted.example.com.|A|us-east-1"
		if resources[0].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[0].ID)
		}
	})

	t.Run("id_with_set_identifier_west", func(t *testing.T) {
		expected := "weighted.example.com.|A|us-west-2"
		if resources[1].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[1].ID)
		}
	})

	t.Run("id_without_set_identifier", func(t *testing.T) {
		expected := "simple.example.com.|CNAME"
		if resources[2].ID != expected {
			t.Errorf("ID: expected %q, got %q", expected, resources[2].ID)
		}
	})
}

// TestFetchR53Records_Error verifies that API errors are propagated correctly.
func TestFetchR53Records_Error(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	resources, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZERR")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchR53Records_Empty verifies that an empty record set returns an empty
// non-nil slice with no error.
func TestFetchR53Records_Empty(t *testing.T) {
	mock := &mockRoute53RecordSetsClient{
		outputs: []*route53.ListResourceRecordSetsOutput{
			{
				IsTruncated:        false,
				ResourceRecordSets: []r53types.ResourceRecordSet{},
			},
		},
	}

	resources, err := awsclient.FetchR53Records(context.Background(), mock, "/hostedzone/ZEMPTY")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestR53RecordColumns verifies that R53RecordColumns returns the expected
// 4 columns with the correct keys: name, type, ttl, values.
func TestR53RecordColumns(t *testing.T) {
	cols := resource.R53RecordColumns()

	expectedKeys := []string{"name", "type", "ttl", "values"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 4 {
			t.Fatalf("expected 4 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})
}
