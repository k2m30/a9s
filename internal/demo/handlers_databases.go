package demo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// registerDatabaseHandlers registers all database-related handlers.
func registerDatabaseHandlers(t *Transport) {
	registerRDSHandlers(t)
	registerDynamoDBDescribeHandlers(t)
	registerElastiCacheHandlers(t)
	registerDocDBHandlers(t)
	registerOpenSearchHandlers(t)
	registerRedshiftHandlers(t)
	registerSnapshotHandlers(t)
}

// ---------------------------------------------------------------------------
// RDS (awsquery — XML, service "rds")
// ---------------------------------------------------------------------------

func registerRDSHandlers(t *Transport) {
	t.Handle("rds", "DescribeDBInstances", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["dbi"]()
		instances := ExtractSDK[rdstypes.DBInstance](resources)

		var sb strings.Builder
		sb.WriteString(`<DBInstances>`)
		for _, db := range instances {
			dbID := aws.ToString(db.DBInstanceIdentifier)
			class := aws.ToString(db.DBInstanceClass)
			engine := aws.ToString(db.Engine)
			engineVer := aws.ToString(db.EngineVersion)
			status := aws.ToString(db.DBInstanceStatus)
			endpoint := ""
			port := int32(0)
			if db.Endpoint != nil {
				endpoint = aws.ToString(db.Endpoint.Address)
				if db.Endpoint.Port != nil {
					port = *db.Endpoint.Port
				}
			}
			multiAZ := "false"
			if db.MultiAZ != nil && *db.MultiAZ {
				multiAZ = "true"
			}
			storageType := aws.ToString(db.StorageType)
			arn := aws.ToString(db.DBInstanceArn)
			az := aws.ToString(db.AvailabilityZone)

			fmt.Fprintf(&sb, `<DBInstance>`)
			fmt.Fprintf(&sb, `<DBInstanceIdentifier>%s</DBInstanceIdentifier>`, xmlEscape(dbID))
			fmt.Fprintf(&sb, `<DBInstanceClass>%s</DBInstanceClass>`, xmlEscape(class))
			fmt.Fprintf(&sb, `<Engine>%s</Engine>`, xmlEscape(engine))
			fmt.Fprintf(&sb, `<EngineVersion>%s</EngineVersion>`, xmlEscape(engineVer))
			fmt.Fprintf(&sb, `<DBInstanceStatus>%s</DBInstanceStatus>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<MultiAZ>%s</MultiAZ>`, multiAZ)
			if storageType != "" {
				fmt.Fprintf(&sb, `<StorageType>%s</StorageType>`, xmlEscape(storageType))
			}
			if az != "" {
				fmt.Fprintf(&sb, `<AvailabilityZone>%s</AvailabilityZone>`, xmlEscape(az))
			}
			if arn != "" {
				fmt.Fprintf(&sb, `<DBInstanceArn>%s</DBInstanceArn>`, xmlEscape(arn))
			}
			if endpoint != "" {
				fmt.Fprintf(&sb, `<Endpoint><Address>%s</Address><Port>%d</Port></Endpoint>`, xmlEscape(endpoint), port)
			}
			fmt.Fprintf(&sb, `</DBInstance>`)
		}
		sb.WriteString(`</DBInstances>`)

		body := awsQueryXML("DescribeDBInstances", "http://rds.amazonaws.com/doc/2014-10-31/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// DynamoDB DescribeTable (awsjson10)
// ---------------------------------------------------------------------------

func registerDynamoDBDescribeHandlers(t *Transport) {
	t.Handle("dynamodb", "DescribeTable", func(req *http.Request) (*http.Response, error) {
		// Read TableName from request body (awsjson10 protocol)
		var input struct {
			TableName string `json:"TableName"`
		}
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err == nil {
				json.Unmarshal(bodyBytes, &input) //nolint:errcheck
			}
		}

		resources := demoData["ddb"]()
		ptrs := ExtractSDK[*ddbtypes.TableDescription](resources)

		// Find matching table by name
		var table *ddbtypes.TableDescription
		for _, p := range ptrs {
			if p != nil && p.TableName != nil && *p.TableName == input.TableName {
				table = p
				break
			}
		}
		if table == nil && len(ptrs) > 0 {
			table = ptrs[0] // fallback
		}

		out := &dynamodb.DescribeTableOutput{
			Table: table,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// ElastiCache (awsquery — XML, service "elasticache")
// ---------------------------------------------------------------------------

func registerElastiCacheHandlers(t *Transport) {
	t.Handle("elasticache", "DescribeCacheClusters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["redis"]()
		clusters := ExtractSDK[elasticachetypes.CacheCluster](resources)

		var sb strings.Builder
		sb.WriteString(`<CacheClusters>`)
		for _, c := range clusters {
			clusterID := aws.ToString(c.CacheClusterId)
			status := aws.ToString(c.CacheClusterStatus)
			nodeType := aws.ToString(c.CacheNodeType)
			engine := aws.ToString(c.Engine)
			engineVersion := aws.ToString(c.EngineVersion)
			arn := aws.ToString(c.ARN)
			nodes := int32(0)
			if c.NumCacheNodes != nil {
				nodes = *c.NumCacheNodes
			}
			endpointAddr := ""
			endpointPort := int32(6379)
			if c.ConfigurationEndpoint != nil {
				endpointAddr = aws.ToString(c.ConfigurationEndpoint.Address)
				if c.ConfigurationEndpoint.Port != nil {
					endpointPort = *c.ConfigurationEndpoint.Port
				}
			}

			fmt.Fprintf(&sb, `<CacheCluster>`)
			fmt.Fprintf(&sb, `<CacheClusterId>%s</CacheClusterId>`, xmlEscape(clusterID))
			fmt.Fprintf(&sb, `<CacheClusterStatus>%s</CacheClusterStatus>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<CacheNodeType>%s</CacheNodeType>`, xmlEscape(nodeType))
			fmt.Fprintf(&sb, `<Engine>%s</Engine>`, xmlEscape(engine))
			fmt.Fprintf(&sb, `<EngineVersion>%s</EngineVersion>`, xmlEscape(engineVersion))
			fmt.Fprintf(&sb, `<NumCacheNodes>%d</NumCacheNodes>`, nodes)
			if arn != "" {
				fmt.Fprintf(&sb, `<ARN>%s</ARN>`, xmlEscape(arn))
			}
			if endpointAddr != "" {
				fmt.Fprintf(&sb, `<ConfigurationEndpoint><Address>%s</Address><Port>%d</Port></ConfigurationEndpoint>`, xmlEscape(endpointAddr), endpointPort)
			}
			fmt.Fprintf(&sb, `</CacheCluster>`)
		}
		sb.WriteString(`</CacheClusters>`)

		body := awsQueryXML("DescribeCacheClusters", "http://elasticache.amazonaws.com/doc/2015-02-02/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// DocDB (awsquery — XML, service "rds" — shares rds endpoint)
// ---------------------------------------------------------------------------

func registerDocDBHandlers(t *Transport) {
	t.Handle("rds", "DescribeDBClusters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["dbc"]()
		clusters := ExtractSDK[docdbtypes.DBCluster](resources)

		var sb strings.Builder
		sb.WriteString(`<DBClusters>`)
		for _, c := range clusters {
			clusterID := aws.ToString(c.DBClusterIdentifier)
			arn := aws.ToString(c.DBClusterArn)
			engine := aws.ToString(c.Engine)
			engineVersion := aws.ToString(c.EngineVersion)
			status := aws.ToString(c.Status)
			endpoint := aws.ToString(c.Endpoint)
			masterUser := aws.ToString(c.MasterUsername)
			multiAZ := "false"
			if c.MultiAZ != nil && *c.MultiAZ {
				multiAZ = "true"
			}

			fmt.Fprintf(&sb, `<DBCluster>`)
			fmt.Fprintf(&sb, `<DBClusterIdentifier>%s</DBClusterIdentifier>`, xmlEscape(clusterID))
			fmt.Fprintf(&sb, `<DBClusterArn>%s</DBClusterArn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<Engine>%s</Engine>`, xmlEscape(engine))
			fmt.Fprintf(&sb, `<EngineVersion>%s</EngineVersion>`, xmlEscape(engineVersion))
			fmt.Fprintf(&sb, `<Status>%s</Status>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<MultiAZ>%s</MultiAZ>`, multiAZ)
			if endpoint != "" {
				fmt.Fprintf(&sb, `<Endpoint>%s</Endpoint>`, xmlEscape(endpoint))
			}
			if masterUser != "" {
				fmt.Fprintf(&sb, `<MasterUsername>%s</MasterUsername>`, xmlEscape(masterUser))
			}
			// Members
			sb.WriteString(`<DBClusterMembers>`)
			for _, m := range c.DBClusterMembers {
				writer := "false"
				if m.IsClusterWriter != nil && *m.IsClusterWriter {
					writer = "true"
				}
				fmt.Fprintf(&sb, `<DBClusterMember><DBInstanceIdentifier>%s</DBInstanceIdentifier><IsClusterWriter>%s</IsClusterWriter></DBClusterMember>`,
					xmlEscape(aws.ToString(m.DBInstanceIdentifier)), writer)
			}
			sb.WriteString(`</DBClusterMembers>`)
			fmt.Fprintf(&sb, `</DBCluster>`)
		}
		sb.WriteString(`</DBClusters>`)

		body := awsQueryXML("DescribeDBClusters", "http://rds.amazonaws.com/doc/2014-10-31/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// OpenSearch (restjson1, service "es")
// ---------------------------------------------------------------------------

func registerOpenSearchHandlers(t *Transport) {
	t.Handle("es", "ListDomainNames", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["opensearch"]()
		domains := ExtractSDK[ostypes.DomainStatus](resources)

		domainNames := make([]ostypes.DomainInfo, 0, len(domains))
		for _, d := range domains {
			if d.DomainName != nil {
				domainNames = append(domainNames, ostypes.DomainInfo{
					DomainName: d.DomainName,
				})
			}
		}

		out := &opensearch.ListDomainNamesOutput{
			DomainNames: domainNames,
		}
		return JSONResponse(out)
	})

	t.Handle("es", "DescribeDomains", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["opensearch"]()
		domains := ExtractSDK[ostypes.DomainStatus](resources)

		out := &opensearch.DescribeDomainsOutput{
			DomainStatusList: domains,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// Redshift (awsquery — XML)
// ---------------------------------------------------------------------------

func registerRedshiftHandlers(t *Transport) {
	t.Handle("redshift", "DescribeClusters", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["redshift"]()

		var sb strings.Builder
		sb.WriteString(`<Clusters>`)
		for _, r := range resources {
			clusterID := r.Fields["cluster_id"]
			nodeType := r.Fields["node_type"]
			status := r.Fields["status"]
			nodes := r.Fields["num_nodes"]
			dbName := r.Fields["db_name"]

			fmt.Fprintf(&sb, `<Cluster>`)
			fmt.Fprintf(&sb, `<ClusterIdentifier>%s</ClusterIdentifier>`, xmlEscape(clusterID))
			fmt.Fprintf(&sb, `<NodeType>%s</NodeType>`, xmlEscape(nodeType))
			fmt.Fprintf(&sb, `<ClusterStatus>%s</ClusterStatus>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<NumberOfNodes>%s</NumberOfNodes>`, xmlEscape(nodes))
			if dbName != "" {
				fmt.Fprintf(&sb, `<DBName>%s</DBName>`, xmlEscape(dbName))
			}
			fmt.Fprintf(&sb, `</Cluster>`)
		}
		sb.WriteString(`</Clusters>`)

		body := awsQueryXML("DescribeClusters", "http://redshift.amazonaws.com/doc/2012-12-01/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// RDS Snapshots (awsquery — XML, service "rds")
// ---------------------------------------------------------------------------

func registerSnapshotHandlers(t *Transport) {
	t.Handle("rds", "DescribeDBSnapshots", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["rds-snap"]()
		snapshots := ExtractSDK[rdstypes.DBSnapshot](resources)

		var sb strings.Builder
		sb.WriteString(`<DBSnapshots>`)
		for _, s := range snapshots {
			snapID := aws.ToString(s.DBSnapshotIdentifier)
			dbID := aws.ToString(s.DBInstanceIdentifier)
			status := aws.ToString(s.Status)
			engine := aws.ToString(s.Engine)
			engineVer := aws.ToString(s.EngineVersion)
			snapType := aws.ToString(s.SnapshotType)
			arn := aws.ToString(s.DBSnapshotArn)

			createTime := ""
			if s.SnapshotCreateTime != nil {
				createTime = s.SnapshotCreateTime.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<DBSnapshot>`)
			fmt.Fprintf(&sb, `<DBSnapshotIdentifier>%s</DBSnapshotIdentifier>`, xmlEscape(snapID))
			fmt.Fprintf(&sb, `<DBInstanceIdentifier>%s</DBInstanceIdentifier>`, xmlEscape(dbID))
			fmt.Fprintf(&sb, `<Status>%s</Status>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<Engine>%s</Engine>`, xmlEscape(engine))
			fmt.Fprintf(&sb, `<EngineVersion>%s</EngineVersion>`, xmlEscape(engineVer))
			fmt.Fprintf(&sb, `<SnapshotType>%s</SnapshotType>`, xmlEscape(snapType))
			if arn != "" {
				fmt.Fprintf(&sb, `<DBSnapshotArn>%s</DBSnapshotArn>`, xmlEscape(arn))
			}
			if createTime != "" {
				fmt.Fprintf(&sb, `<SnapshotCreateTime>%s</SnapshotCreateTime>`, createTime)
			}
			fmt.Fprintf(&sb, `</DBSnapshot>`)
		}
		sb.WriteString(`</DBSnapshots>`)

		body := awsQueryXML("DescribeDBSnapshots", "http://rds.amazonaws.com/doc/2014-10-31/", sb.String())
		return XMLResponse(body), nil
	})

	t.Handle("rds", "DescribeDBClusterSnapshots", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["docdb-snap"]()
		snapshots := ExtractSDK[docdbtypes.DBClusterSnapshot](resources)

		var sb strings.Builder
		sb.WriteString(`<DBClusterSnapshots>`)
		for _, s := range snapshots {
			snapID := aws.ToString(s.DBClusterSnapshotIdentifier)
			clusterID := aws.ToString(s.DBClusterIdentifier)
			status := aws.ToString(s.Status)
			engine := aws.ToString(s.Engine)
			engineVer := aws.ToString(s.EngineVersion)
			snapType := aws.ToString(s.SnapshotType)
			arn := aws.ToString(s.DBClusterSnapshotArn)

			createTime := ""
			if s.SnapshotCreateTime != nil {
				createTime = s.SnapshotCreateTime.UTC().Format("2006-01-02T15:04:05Z")
			}

			fmt.Fprintf(&sb, `<DBClusterSnapshot>`)
			fmt.Fprintf(&sb, `<DBClusterSnapshotIdentifier>%s</DBClusterSnapshotIdentifier>`, xmlEscape(snapID))
			fmt.Fprintf(&sb, `<DBClusterIdentifier>%s</DBClusterIdentifier>`, xmlEscape(clusterID))
			fmt.Fprintf(&sb, `<Status>%s</Status>`, xmlEscape(status))
			fmt.Fprintf(&sb, `<Engine>%s</Engine>`, xmlEscape(engine))
			fmt.Fprintf(&sb, `<EngineVersion>%s</EngineVersion>`, xmlEscape(engineVer))
			fmt.Fprintf(&sb, `<SnapshotType>%s</SnapshotType>`, xmlEscape(snapType))
			if arn != "" {
				fmt.Fprintf(&sb, `<DBClusterSnapshotArn>%s</DBClusterSnapshotArn>`, xmlEscape(arn))
			}
			if createTime != "" {
				fmt.Fprintf(&sb, `<SnapshotCreateTime>%s</SnapshotCreateTime>`, createTime)
			}
			fmt.Fprintf(&sb, `</DBClusterSnapshot>`)
		}
		sb.WriteString(`</DBClusterSnapshots>`)

		body := awsQueryXML("DescribeDBClusterSnapshots", "http://rds.amazonaws.com/doc/2014-10-31/", sb.String())
		return XMLResponse(body), nil
	})
}
