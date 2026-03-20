package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("iam-group", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMGroups(ctx, c.IAM)
	})
	resource.RegisterFieldKeys("iam-group", []string{"group_name", "group_id", "path", "create_date", "arn"})
}

// FetchIAMGroups calls the IAM ListGroups API and converts the
// response into a slice of generic Resource structs.
func FetchIAMGroups(ctx context.Context, api IAMListGroupsAPI) ([]resource.Resource, error) {
	output, err := api.ListGroups(ctx, &iam.ListGroupsInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, group := range output.Groups {
		groupName := ""
		if group.GroupName != nil {
			groupName = *group.GroupName
		}

		groupID := ""
		if group.GroupId != nil {
			groupID = *group.GroupId
		}

		path := ""
		if group.Path != nil {
			path = *group.Path
		}

		createDate := ""
		if group.CreateDate != nil {
			createDate = group.CreateDate.Format("2006-01-02T15:04:05Z07:00")
		}

		arn := ""
		if group.Arn != nil {
			arn = *group.Arn
		}

		detail := map[string]string{
			"Group Name": groupName,
			"Group ID":   groupID,
			"ARN":        arn,
			"Path":       path,
			"Created":    createDate,
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(group, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     groupName,
			Name:   groupName,
			Status: "",
			Fields: map[string]string{
				"group_name":  groupName,
				"group_id":    groupID,
				"path":        path,
				"create_date": createDate,
				"arn":         arn,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  group,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
