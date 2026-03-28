package demo

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

// registerContainerHandlers registers ECS (full chain) and EKS/NodeGroup handlers.
func registerContainerHandlers(t *Transport) {
	registerECSFullHandlers(t)
	registerEKSHandlers(t)
}

// ---------------------------------------------------------------------------
// ECS full chain (awsjson11)
// ---------------------------------------------------------------------------

func registerECSFullHandlers(t *Transport) {
	// DescribeClusters — return all clusters regardless of input ARNs.
	// ECS uses awsjson11 with lowercase "clusters" key.
	t.Handle("ecs", "DescribeClusters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ecs"]()
		clusters := ExtractSDK[ecstypes.Cluster](resources)

		clusterMaps := make([]map[string]interface{}, 0, len(clusters))
		for _, c := range clusters {
			m := map[string]interface{}{
				"clusterName":                      aws.ToString(c.ClusterName),
				"clusterArn":                       aws.ToString(c.ClusterArn),
				"status":                           aws.ToString(c.Status),
				"runningTasksCount":                c.RunningTasksCount,
				"pendingTasksCount":                c.PendingTasksCount,
				"activeServicesCount":              c.ActiveServicesCount,
				"registeredContainerInstancesCount": c.RegisteredContainerInstancesCount,
			}
			clusterMaps = append(clusterMaps, m)
		}

		return JSONResponse(map[string]interface{}{"clusters": clusterMaps})
	})

	// ListServices — return service ARNs filtered by requested cluster.
	t.Handle("ecs", "ListServices", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		clusterFilter, _ := body["cluster"].(string)

		resources := demoData["ecs-svc"]()
		services := ExtractSDK[ecstypes.Service](resources)

		arns := make([]string, 0, len(services))
		for _, s := range services {
			if s.ServiceArn == nil {
				continue
			}
			if clusterFilter != "" && aws.ToString(s.ClusterArn) != clusterFilter {
				continue
			}
			arns = append(arns, *s.ServiceArn)
		}

		resp := map[string]interface{}{"serviceArns": arns}
		return JSONResponse(resp)
	})

	// DescribeServices — return services matching the requested ARNs.
	// ECS uses awsjson11 with lowercase "services" key.
	t.Handle("ecs", "DescribeServices", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		requestedSet := map[string]bool{}
		if raw, ok := body["services"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						requestedSet[s] = true
					}
				}
			}
		}

		resources := demoData["ecs-svc"]()
		services := ExtractSDK[ecstypes.Service](resources)

		svcMaps := make([]map[string]interface{}, 0, len(services))
		for _, s := range services {
			if len(requestedSet) > 0 && !requestedSet[aws.ToString(s.ServiceArn)] {
				continue
			}
			m := map[string]interface{}{
				"serviceName":  aws.ToString(s.ServiceName),
				"serviceArn":   aws.ToString(s.ServiceArn),
				"clusterArn":   aws.ToString(s.ClusterArn),
				"status":       aws.ToString(s.Status),
				"desiredCount": s.DesiredCount,
				"runningCount": s.RunningCount,
				"pendingCount": s.PendingCount,
			}
			if s.LaunchType != "" {
				m["launchType"] = string(s.LaunchType)
			}
			svcMaps = append(svcMaps, m)
		}

		return JSONResponse(map[string]interface{}{"services": svcMaps})
	})

	// ListTasks — return task ARNs filtered by requested cluster.
	t.Handle("ecs", "ListTasks", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		clusterFilter, _ := body["cluster"].(string)

		resources := demoData["ecs-task"]()
		tasks := ExtractSDK[ecstypes.Task](resources)

		arns := make([]string, 0, len(tasks))
		for _, task := range tasks {
			if task.TaskArn == nil {
				continue
			}
			if clusterFilter != "" && aws.ToString(task.ClusterArn) != clusterFilter {
				continue
			}
			arns = append(arns, *task.TaskArn)
		}

		resp := map[string]interface{}{"taskArns": arns}
		return JSONResponse(resp)
	})

	// DescribeTasks — return tasks matching the requested task ARNs.
	// ECS uses awsjson11 with lowercase "tasks" key.
	t.Handle("ecs", "DescribeTasks", func(req *http.Request) (*http.Response, error) {
		var body map[string]interface{}
		if b, err := io.ReadAll(req.Body); err == nil {
			_ = json.Unmarshal(b, &body)
		}
		requestedSet := map[string]bool{}
		if raw, ok := body["tasks"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						requestedSet[s] = true
					}
				}
			}
		}

		resources := demoData["ecs-task"]()
		tasks := ExtractSDK[ecstypes.Task](resources)

		taskMaps := make([]map[string]interface{}, 0, len(tasks))
		for _, task := range tasks {
			if len(requestedSet) > 0 && !requestedSet[aws.ToString(task.TaskArn)] {
				continue
			}
			m := map[string]interface{}{
				"taskArn":           aws.ToString(task.TaskArn),
				"clusterArn":        aws.ToString(task.ClusterArn),
				"lastStatus":        aws.ToString(task.LastStatus),
				"desiredStatus":     aws.ToString(task.DesiredStatus),
				"taskDefinitionArn": aws.ToString(task.TaskDefinitionArn),
			}
			if task.LaunchType != "" {
				m["launchType"] = string(task.LaunchType)
			}
			taskMaps = append(taskMaps, m)
		}

		return JSONResponse(map[string]interface{}{"tasks": taskMaps})
	})
}

// ---------------------------------------------------------------------------
// EKS (restjson1 — routed by URL path)
// ---------------------------------------------------------------------------

func registerEKSHandlers(t *Transport) {
	// ListClusters — return cluster names.
	// EKS uses restjson1 with lowercase "clusters" key.
	t.Handle("eks", "ListClusters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["eks"]()

		names := make([]string, 0, len(resources))
		for _, r := range resources {
			names = append(names, r.ID)
		}

		return JSONResponse(map[string]interface{}{"clusters": names})
	})

	// DescribeCluster — return the cluster matching the name in the URL path.
	// EKS uses restjson1 with lowercase "cluster" key.
	// URL path format: /clusters/{name}
	t.Handle("eks", "DescribeCluster", func(req *http.Request) (*http.Response, error) {
		// Extract cluster name from path: /clusters/{name}
		path := req.URL.Path
		clusterName := ""
		if strings.HasPrefix(path, "/clusters/") {
			parts := strings.Split(strings.TrimPrefix(path, "/clusters/"), "/")
			if len(parts) > 0 {
				clusterName = parts[0]
			}
		}

		resources := demoData["eks"]()
		ptrs := ExtractSDK[*ekstypes.Cluster](resources)

		var cluster *ekstypes.Cluster
		for _, c := range ptrs {
			if c != nil && aws.ToString(c.Name) == clusterName {
				cluster = c
				break
			}
		}
		if cluster == nil && len(ptrs) > 0 {
			cluster = ptrs[0] // fallback
		}

		var clusterMap map[string]interface{}
		if cluster != nil {
			clusterMap = map[string]interface{}{
				"name":                    aws.ToString(cluster.Name),
				"arn":                     aws.ToString(cluster.Arn),
				"status":                  string(cluster.Status),
				"kubernetesNetworkConfig": map[string]interface{}{},
			}
			if cluster.Version != nil {
				clusterMap["version"] = *cluster.Version
			}
			if cluster.Endpoint != nil {
				clusterMap["endpoint"] = *cluster.Endpoint
			}
		}

		return JSONResponse(map[string]interface{}{"cluster": clusterMap})
	})

	// ListNodegroups — return nodegroup names filtered by cluster name from URL path.
	// EKS uses restjson1 with lowercase "nodegroups" key.
	// URL path format: /clusters/{name}/node-groups
	t.Handle("eks", "ListNodegroups", func(req *http.Request) (*http.Response, error) {
		// Extract cluster name from path: /clusters/{name}/node-groups
		path := req.URL.Path
		clusterName := ""
		if strings.HasPrefix(path, "/clusters/") {
			parts := strings.Split(strings.TrimPrefix(path, "/clusters/"), "/")
			if len(parts) > 0 {
				clusterName = parts[0]
			}
		}

		resources := demoData["ng"]()
		names := make([]string, 0)
		for _, r := range resources {
			if ng, ok := r.RawStruct.(ekstypes.Nodegroup); ok {
				if aws.ToString(ng.ClusterName) == clusterName {
					names = append(names, r.ID)
				}
			}
		}

		return JSONResponse(map[string]interface{}{"nodegroups": names})
	})

	// DescribeNodegroup — return the nodegroup matching cluster+nodegroup names from URL path.
	// EKS uses restjson1 with lowercase "nodegroup" key.
	// URL path format: /clusters/{cluster}/node-groups/{nodegroup}
	t.Handle("eks", "DescribeNodegroup", func(req *http.Request) (*http.Response, error) {
		// Extract cluster and nodegroup names from path
		path := req.URL.Path
		clusterName := ""
		nodegroupName := ""
		if strings.HasPrefix(path, "/clusters/") {
			parts := strings.Split(strings.TrimPrefix(path, "/clusters/"), "/")
			if len(parts) >= 1 {
				clusterName = parts[0]
			}
			if len(parts) >= 3 && parts[1] == "node-groups" {
				nodegroupName = parts[2]
			}
		}

		resources := demoData["ng"]()
		ngs := ExtractSDK[ekstypes.Nodegroup](resources)

		var matched *ekstypes.Nodegroup
		for i := range ngs {
			if aws.ToString(ngs[i].ClusterName) == clusterName && aws.ToString(ngs[i].NodegroupName) == nodegroupName {
				matched = &ngs[i]
				break
			}
		}
		if matched == nil && len(ngs) > 0 {
			matched = &ngs[0] // fallback
		}

		var ngMap map[string]interface{}
		if matched != nil {
			ngMap = map[string]interface{}{
				"nodegroupName": aws.ToString(matched.NodegroupName),
				"nodegroupArn":  aws.ToString(matched.NodegroupArn),
				"clusterName":   aws.ToString(matched.ClusterName),
				"status":        string(matched.Status),
			}
			if matched.InstanceTypes != nil {
				ngMap["instanceTypes"] = matched.InstanceTypes
			}
		}

		return JSONResponse(map[string]interface{}{"nodegroup": ngMap})
	})
}
