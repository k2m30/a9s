package resource

func computeResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:      "EC2 Instances",
			ShortName: "ec2",
			Aliases:   []string{"ec2", "instances"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "type", Title: "Type", Width: 14, Sortable: true},
				{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
				{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
				{Key: "launch_time", Title: "Launch Time", Width: 22, Sortable: true},
			},
		},
		{
			Name:      "ECS Services",
			ShortName: "ecs-svc",
			Aliases:   []string{"ecs-svc", "ecs-services"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "service_name", Title: "Service Name", Width: 32, Sortable: true},
				{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "desired_count", Title: "Desired", Width: 9, Sortable: true},
				{Key: "running_count", Title: "Running", Width: 9, Sortable: true},
				{Key: "launch_type", Title: "Launch Type", Width: 12, Sortable: true},
			},
			Children: []ChildViewDef{
				{
					ChildType:      "ecs_tasks",
					Key:            "enter",
					ContextKeys:    map[string]string{"cluster": "cluster", "service_name": "service_name"},
					DisplayNameKey: "service_name",
				},
				{
					ChildType:      "ecs_svc_events",
					Key:            "e",
					ContextKeys:    map[string]string{"cluster": "cluster", "service_name": "service_name"},
					DisplayNameKey: "service_name",
				},
				{
					ChildType:      "ecs_svc_logs",
					Key:            "L",
					ContextKeys:    map[string]string{"cluster": "cluster", "service_name": "service_name", "task_definition": "task_definition"},
					DisplayNameKey: "service_name",
				},
			},
		},
		{
			Name:      "ECS Clusters",
			ShortName: "ecs",
			Aliases:   []string{"ecs", "ecs-clusters"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "cluster_name", Title: "Cluster Name", Width: 32, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "running_tasks", Title: "Running", Width: 9, Sortable: true},
				{Key: "pending_tasks", Title: "Pending", Width: 9, Sortable: true},
				{Key: "services_count", Title: "Services", Width: 10, Sortable: true},
			},
		},
		{
			Name:      "ECS Tasks",
			ShortName: "ecs-task",
			Aliases:   []string{"ecs-task", "ecs-tasks", "tasks"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "task_id", Title: "Task ID", Width: 38, Sortable: true},
				{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "task_definition", Title: "Task Definition", Width: 30, Sortable: true},
				{Key: "launch_type", Title: "Launch", Width: 10, Sortable: true},
				{Key: "cpu", Title: "CPU", Width: 6, Sortable: true},
				{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
			},
		},
		{
			Name:      "Lambda Functions",
			ShortName: "lambda",
			Aliases:   []string{"lambda", "functions"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "function_name", Title: "Function Name", Width: 36, Sortable: true},
				{Key: "runtime", Title: "Runtime", Width: 16, Sortable: true},
				{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
				{Key: "timeout", Title: "Timeout", Width: 8, Sortable: true},
				{Key: "handler", Title: "Handler", Width: 30, Sortable: false},
				{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			},
			Children: []ChildViewDef{
				{
					ChildType:      "lambda_invocations",
					Key:            "enter",
					ContextKeys:    map[string]string{"function_name": "function_name", "log_group": "log_group"},
					DisplayNameKey: "function_name",
				},
			},
		},
		{
			Name:      "Auto Scaling Groups",
			ShortName: "asg",
			Aliases:   []string{"asg", "autoscaling", "auto-scaling"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "asg_name", Title: "ASG Name", Width: 36, Sortable: true},
				{Key: "min_size", Title: "Min", Width: 6, Sortable: true},
				{Key: "max_size", Title: "Max", Width: 6, Sortable: true},
				{Key: "desired", Title: "Desired", Width: 8, Sortable: true},
				{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
			},
		},
		{
			Name:      "Elastic Beanstalk",
			ShortName: "eb",
			Aliases:   []string{"eb", "beanstalk", "elastic-beanstalk"},
			Category:  "COMPUTE",
			Columns: []Column{
				{Key: "environment_name", Title: "Environment", Width: 28, Sortable: true},
				{Key: "application_name", Title: "Application", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "health", Title: "Health", Width: 10, Sortable: true},
				{Key: "version_label", Title: "Version", Width: 16, Sortable: true},
			},
		},
	}
}
