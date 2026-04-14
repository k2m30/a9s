package resource

import "strings"

// cfnStackColor maps CloudFormation stack status strings to a Color.
// Healthy: *_COMPLETE (except DELETE_COMPLETE), Warning: *_IN_PROGRESS,
// Broken: *_FAILED and ROLLBACK_COMPLETE/ROLLBACK_FAILED, Dim: DELETE_COMPLETE.
func cfnStackColor(status string) Color {
	switch status {
	case "CREATE_COMPLETE", "UPDATE_COMPLETE", "IMPORT_COMPLETE":
		return ColorHealthy
	case "DELETE_COMPLETE":
		return ColorDim
	case "ROLLBACK_COMPLETE", "ROLLBACK_FAILED", "UPDATE_ROLLBACK_COMPLETE":
		return ColorBroken
	}
	if strings.HasSuffix(status, "_IN_PROGRESS") {
		return ColorWarning
	}
	if strings.HasSuffix(status, "_FAILED") {
		return ColorBroken
	}
	return ColorHealthy
}

func cicdResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "CloudFormation Stacks",
			ShortName:     "cfn",
			Aliases:       []string{"cfn", "cloudformation", "stacks"},
			Category:      "CI/CD",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "stack_name", Title: "Stack Name", Width: 36, Sortable: true},
				{Key: "status", Title: "Status", Width: 24, Sortable: true},
				{Key: "creation_time", Title: "Created", Width: 22, Sortable: true},
				{Key: "last_updated", Title: "Updated", Width: 22, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
			Color: func(r Resource) Color {
				return cfnStackColor(r.Fields["status"])
			},
			Children: []ChildViewDef{
				{ChildType: "cfn_events", Key: "enter", ContextKeys: map[string]string{"stack_name": "ID"}, DisplayNameKey: "Name"},
				{ChildType: "cfn_resources", Key: "R", ContextKeys: map[string]string{"stack_name": "ID"}, DisplayNameKey: "Name"},
			},
		},
		{
			Name:          "CodePipelines",
			ShortName:     "pipeline",
			Aliases:       []string{"pipeline", "codepipeline", "pipelines"},
			Category:      "CI/CD",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Pipeline Name", Width: 30, Sortable: true},
				{Key: "pipeline_type", Title: "Type", Width: 6, Sortable: true},
				{Key: "version", Title: "Version", Width: 9, Sortable: true},
				{Key: "created", Title: "Created", Width: 22, Sortable: true},
				{Key: "updated", Title: "Updated", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["latest_execution_status"] {
				case "Succeeded":
					return ColorHealthy
				case "InProgress":
					return ColorWarning
				case "Failed", "Cancelled", "Superseded":
					return ColorBroken
				}
				return ColorHealthy
			},
			Children: []ChildViewDef{{
				ChildType:      "pipeline_stages",
				Key:            "enter",
				ContextKeys:    map[string]string{"pipeline_name": "ID"},
				DisplayNameKey: "Name",
			}},
		},
		{
			Name:          "CodeBuild Projects",
			ShortName:     "cb",
			Aliases:       []string{"cb", "codebuild"},
			Category:      "CI/CD",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Project Name", Width: 32, Sortable: true},
				{Key: "source_type", Title: "Source Type", Width: 14, Sortable: true},
				{Key: "description", Title: "Description", Width: 36, Sortable: false},
				{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["latest_build_status"] {
				case "SUCCEEDED":
					return ColorHealthy
				case "IN_PROGRESS":
					return ColorWarning
				case "FAILED", "FAULT", "TIMED_OUT", "STOPPED":
					return ColorBroken
				}
				return ColorHealthy
			},
			Children: []ChildViewDef{{
				ChildType:      "cb_builds",
				Key:            "enter",
				ContextKeys:    map[string]string{"project_name": "ID"},
				DisplayNameKey: "project_name",
			}},
		},
		{
			Name:          "ECR Repositories",
			ShortName:     "ecr",
			Aliases:       []string{"ecr", "container-registry"},
			Category:      "CI/CD",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "repository_name", Title: "Repository", Width: 36, Sortable: true},
				{Key: "uri", Title: "URI", Width: 60, Sortable: false},
				{Key: "tag_mutability", Title: "Tag Mutability", Width: 16, Sortable: true},
				{Key: "scan_on_push", Title: "Scan", Width: 6, Sortable: true},
				{Key: "created_at", Title: "Created", Width: 22, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			Children: []ChildViewDef{{
				ChildType:      "ecr_images",
				Key:            "enter",
				ContextKeys:    map[string]string{"repository_name": "ID", "repository_uri": "uri"},
				DisplayNameKey: "repository_name",
			}},
		},
		{
			Name:          "CodeArtifact Repos",
			ShortName:     "codeartifact",
			Aliases:       []string{"codeartifact", "artifact", "ca"},
			Category:      "CI/CD",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "repo_name", Title: "Repository", Width: 28, Sortable: true},
				{Key: "domain_name", Title: "Domain", Width: 24, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
				{Key: "domain_owner", Title: "Owner", Width: 14, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
		},
	}
}
