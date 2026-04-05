package demo

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ec2XMLFieldOverride maps "TypeName/FieldName" to the EC2 XML element name.
// The EC2 API uses names that diverge from "lowercase first character of field name"
// for several fields — this table captures every known exception discovered from
// the AWS SDK deserializers.go.
//
// Format: "GoTypeName/GoFieldName" → "xmlElementName"
//
// Source of truth: awsEc2query_deserializeDocument* functions in
// github.com/aws/aws-sdk-go-v2/service/ec2@v1.296.0/deserializers.go
var ec2XMLFieldOverride = map[string]string{
	// ec2types.Instance
	"Instance/State":                 "instanceState", // State *InstanceState → <instanceState>
	"Instance/SecurityGroups":        "groupSet",       // SecurityGroups []GroupIdentifier → <groupSet>
	"Instance/Tags":                  "tagSet",         // Tags []Tag → <tagSet>
	"Instance/PublicIpAddress":       "ipAddress",      // PublicIpAddress *string → <ipAddress>
	"Instance/PublicDnsName":         "dnsName",        // PublicDnsName *string → <dnsName>
	"Instance/StateTransitionReason": "reason",         // StateTransitionReason *string → <reason>
	"Instance/BlockDeviceMappings":   "blockDeviceMapping",
	"Instance/NetworkInterfaces":     "networkInterfaceSet",
	"Instance/ElasticGpuAssociations":                    "elasticGpuAssociationSet",
	"Instance/ElasticInferenceAcceleratorAssociations":   "elasticInferenceAcceleratorAssociationSet",
	"Instance/ProductCodes":          "productCodes",
	"Instance/Licenses":              "licenseSet",
	"Instance/SecondaryInterfaces":   "secondaryInterfaceSet",

	// ec2types.Image
	"Image/State":   "imageState",   // State ImageState → <imageState>
	"Image/Public":  "isPublic",     // Public *bool → <isPublic>
	"Image/OwnerId": "imageOwnerId", // OwnerId *string → <imageOwnerId>
	"Image/Tags":    "tagSet",
	"Image/BlockDeviceMappings": "blockDeviceMapping",
	"Image/ProductCodes":        "productCodes",

	// ec2types.Volume
	"Volume/State":       "status",        // State VolumeState → <status>
	"Volume/Attachments": "attachmentSet", // Attachments []VolumeAttachment → <attachmentSet>
	"Volume/Tags":        "tagSet",

	// ec2types.VolumeAttachment
	"VolumeAttachment/State": "status", // State VolumeAttachmentState → <status>

	// ec2types.Snapshot
	"Snapshot/State": "status", // State SnapshotState → <status>
	"Snapshot/Tags":  "tagSet",

	// ec2types.Vpc
	"Vpc/Tags":  "tagSet",
	"Vpc/CidrBlockAssociationSet": "cidrBlockAssociationSet",
	"Vpc/Ipv6CidrBlockAssociationSet": "ipv6CidrBlockAssociationSet",

	// ec2types.Subnet
	"Subnet/Tags": "tagSet",

	// ec2types.SecurityGroup
	"SecurityGroup/Tags":               "tagSet",
	"SecurityGroup/IpPermissions":       "ipPermissions",
	"SecurityGroup/IpPermissionsEgress": "ipPermissionsEgress",

	// General Tag lists on any EC2 type (fallback — handled via type lookup too)
}

// ec2XMLTypeNameCache caches reflect.Type → simple type name lookups.
var ec2XMLTypeNameCache = map[reflect.Type]string{}

// ec2TypeName returns the simple (unqualified) name of the type, following
// pointer chains to get to the concrete type.
func ec2TypeName(t reflect.Type) string {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if cached, ok := ec2XMLTypeNameCache[t]; ok {
		return cached
	}
	name := t.Name()
	ec2XMLTypeNameCache[t] = name
	return name
}

// ec2ItemXML converts an AWS SDK struct to EC2 Query XML format using reflection.
// Field names are lowercased first character (e.g., InstanceId → instanceId) unless
// overridden by ec2XMLFieldOverride. Nil pointers are skipped. Slices use <item>
// wrapper elements. Enums (typed strings) are rendered as plain text.
// time.Time is formatted as ISO 8601 (2006-01-02T15:04:05.000Z).
func ec2ItemXML(v interface{}) string {
	return ec2ValueXML(reflect.ValueOf(v), "")
}

var timeType = reflect.TypeOf(time.Time{})

// ec2ValueXML recursively converts a reflected value to EC2 XML content (inner text).
// parentTypeName is the simple name of the containing struct type, used to look up
// per-type field name overrides.
func ec2ValueXML(v reflect.Value, parentTypeName string) string {
	// Dereference pointers.
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// Handle interface values.
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		return ec2ValueXML(v.Elem(), parentTypeName)
	}

	// time.Time — format as ISO 8601.
	if v.Type() == timeType {
		t := v.Interface().(time.Time)
		if t.IsZero() {
			return ""
		}
		return t.UTC().Format("2006-01-02T15:04:05.000Z")
	}

	switch v.Kind() {
	case reflect.Struct:
		return ec2StructXML(v)

	case reflect.Slice:
		if v.IsNil() || v.Len() == 0 {
			return ""
		}
		return ec2SliceXML(v)

	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())

	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())

	case reflect.String:
		s := v.String()
		if s == "" {
			return ""
		}
		return xmlEscape(s)

	default:
		return ""
	}
}

// ec2StructXML serializes a struct value to EC2 XML elements.
// Each exported non-zero field becomes one XML element.
func ec2StructXML(v reflect.Value) string {
	typeName := ec2TypeName(v.Type())
	t := v.Type()
	var sb strings.Builder
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields (includes noSmithyDocumentSerde and other embedded SDK internals).
		if !field.IsExported() {
			continue
		}

		fieldVal := v.Field(i)
		content := ec2ValueXML(fieldVal, typeName)
		if content == "" {
			// Skip zero-value / nil fields.
			continue
		}

		xmlName := ec2XMLElementNameFor(typeName, field.Name)
		fmt.Fprintf(&sb, "<%s>%s</%s>", xmlName, content, xmlName)
	}
	return sb.String()
}

// ec2SliceXML serializes a slice of EC2 values as <item> elements.
func ec2SliceXML(v reflect.Value) string {
	var sb strings.Builder
	for i := 0; i < v.Len(); i++ {
		content := ec2ValueXML(v.Index(i), "")
		if content == "" {
			continue
		}
		fmt.Fprintf(&sb, "<item>%s</item>", content)
	}
	return sb.String()
}

// ec2XMLElementNameFor returns the EC2 XML element name for a given parent type
// and Go struct field name.  It first checks the per-type override table; if no
// match is found it falls back to lowercasing the first character of the field name
// (e.g., InstanceId → instanceId).
func ec2XMLElementNameFor(parentTypeName, goFieldName string) string {
	key := parentTypeName + "/" + goFieldName
	if override, ok := ec2XMLFieldOverride[key]; ok {
		return override
	}
	if len(goFieldName) == 0 {
		return goFieldName
	}
	return strings.ToLower(goFieldName[:1]) + goFieldName[1:]
}
