package catalog

import "github.com/k2m30/a9s/v3/internal/domain"

func colorGlue(_ domain.Resource) domain.Color { return domain.ColorHealthy }

func colorAthena(r domain.Resource) domain.Color {
	switch r.Fields["state"] {
	case "ENABLED":
		return domain.ColorHealthy
	case "DISABLED":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var dataTypes = []ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Glue Jobs",
		ShortName:     "glue",
		Aliases:       []string{"glue", "glue-jobs"},
		Category:      "DATA & ANALYTICS",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "job_name", Title: "Job Name", Width: 32, Sortable: true},
			{Key: "glue_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "worker_type", Title: "Worker Type", Width: 14, Sortable: true},
			{Key: "num_workers", Title: "Workers", Width: 9, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "glue_runs",
			Key:            "enter",
			ContextKeys:    map[string]string{"job_name": "ID"},
			DisplayNameKey: "job_name",
		}},
		Color: colorGlue,
	},
	{
		Name:          "Athena Workgroups",
		ShortName:     "athena",
		Aliases:       []string{"athena", "workgroups"},
		Category:      "DATA & ANALYTICS",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "workgroup_name", Title: "Workgroup", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "engine_version", Title: "Engine", Width: 28, Sortable: true},
		},
		Color: colorAthena,
	},
}
