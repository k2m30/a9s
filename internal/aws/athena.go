package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/athena"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("athena", []string{"workgroup_name", "state", "description", "engine_version"})
	resource.Register("athena", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAthenaWorkgroups(ctx, c.Athena)
	})
}

// FetchAthenaWorkgroups calls the Athena ListWorkGroups API and converts the
// response into a slice of generic Resource structs.
func FetchAthenaWorkgroups(ctx context.Context, api AthenaListWorkGroupsAPI) ([]resource.Resource, error) {
	output, err := api.ListWorkGroups(ctx, &athena.ListWorkGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching Athena workgroups: %w", err)
	}

	var resources []resource.Resource

	for _, wg := range output.WorkGroups {
		wgName := ""
		if wg.Name != nil {
			wgName = *wg.Name
		}

		state := string(wg.State)

		description := ""
		if wg.Description != nil {
			description = *wg.Description
		}

		creationTime := ""
		if wg.CreationTime != nil {
			creationTime = wg.CreationTime.Format("2006-01-02 15:04:05")
		}

		engineVersion := ""
		if wg.EngineVersion != nil && wg.EngineVersion.EffectiveEngineVersion != nil {
			engineVersion = *wg.EngineVersion.EffectiveEngineVersion
		}

		r := resource.Resource{
			ID:     wgName,
			Name:   wgName,
			Status: state,
			Fields: map[string]string{
				"workgroup_name": wgName,
				"state":          state,
				"description":    description,
				"creation_time":  creationTime,
				"engine_version": engineVersion,
			},
			RawStruct: wg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
