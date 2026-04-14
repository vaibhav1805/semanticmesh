package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ComponentType represents the classification of an infrastructure component.
// The taxonomy is based on common infrastructure patterns from Backstage and
// Cartography, covering the most common categories found in service-oriented
// architectures.
type ComponentType string

const (
	// ComponentTypeService represents an application service (e.g. payment-api,
	// user-service, auth-service).
	ComponentTypeService ComponentType = "service"

	// ComponentTypeDatabase represents a data store (e.g. postgres, mysql,
	// mongodb, dynamodb).
	ComponentTypeDatabase ComponentType = "database"

	// ComponentTypeCache represents an in-memory caching layer (e.g. redis,
	// memcached, varnish).
	ComponentTypeCache ComponentType = "cache"

	// ComponentTypeQueue represents a message queue (e.g. sqs, celery, sidekiq).
	ComponentTypeQueue ComponentType = "queue"

	// ComponentTypeMessageBroker represents a message broker / event streaming
	// platform (e.g. kafka, rabbitmq, nats, pulsar).
	ComponentTypeMessageBroker ComponentType = "message-broker"

	// ComponentTypeLoadBalancer represents a load balancer or reverse proxy
	// (e.g. nginx, haproxy, envoy, aws alb).
	ComponentTypeLoadBalancer ComponentType = "load-balancer"

	// ComponentTypeGateway represents an API gateway (e.g. kong, aws api-gateway,
	// traefik, ambassador).
	ComponentTypeGateway ComponentType = "gateway"

	// ComponentTypeStorage represents object or file storage (e.g. s3, gcs,
	// minio, azure blob).
	ComponentTypeStorage ComponentType = "storage"

	// ComponentTypeContainerRegistry represents a container image registry
	// (e.g. ecr, gcr, docker-hub, harbor).
	ComponentTypeContainerRegistry ComponentType = "container-registry"

	// ComponentTypeConfigServer represents a configuration management service
	// (e.g. consul, etcd, spring-cloud-config, vault).
	ComponentTypeConfigServer ComponentType = "config-server"

	// ComponentTypeMonitoring represents an observability / monitoring platform
	// (e.g. prometheus, datadog, grafana, new-relic).
	ComponentTypeMonitoring ComponentType = "monitoring"

	// ComponentTypeLogAggregator represents a log collection and search platform
	// (e.g. elasticsearch, splunk, loki, fluentd).
	ComponentTypeLogAggregator ComponentType = "log-aggregator"

	// ComponentTypeOrchestrator represents a container orchestration platform
	// (e.g. kubernetes, nomad, mesos, docker-swarm).
	ComponentTypeOrchestrator ComponentType = "orchestrator"

	// ComponentTypeSecretsManager represents a secrets management service
	// (e.g. vault, aws-secrets-manager, azure-key-vault).
	ComponentTypeSecretsManager ComponentType = "secrets-manager"

	// ComponentTypeSearch represents a search engine
	// (e.g. elasticsearch, solr, algolia, meilisearch).
	ComponentTypeSearch ComponentType = "search"

	// ComponentTypeUnknown is the default type assigned when automatic detection
	// cannot determine a more specific classification.
	ComponentTypeUnknown ComponentType = "unknown"
)

// allComponentTypes is the canonical list of valid component types in display
// order.  Used by AllComponentTypes() and IsValidComponentType().
var allComponentTypes = []ComponentType{
	ComponentTypeService,
	ComponentTypeDatabase,
	ComponentTypeCache,
	ComponentTypeQueue,
	ComponentTypeMessageBroker,
	ComponentTypeLoadBalancer,
	ComponentTypeGateway,
	ComponentTypeStorage,
	ComponentTypeContainerRegistry,
	ComponentTypeConfigServer,
	ComponentTypeMonitoring,
	ComponentTypeLogAggregator,
	ComponentTypeOrchestrator,
	ComponentTypeSecretsManager,
	ComponentTypeSearch,
	ComponentTypeUnknown,
}

// validComponentTypes is a set for O(1) membership testing.
var validComponentTypes map[ComponentType]bool

func init() {
	validComponentTypes = make(map[ComponentType]bool, len(allComponentTypes))
	for _, t := range allComponentTypes {
		validComponentTypes[t] = true
	}
}

// AllComponentTypes returns the 12 taxonomy types plus the "unknown" default,
// in canonical display order.
func AllComponentTypes() []ComponentType {
	out := make([]ComponentType, len(allComponentTypes))
	copy(out, allComponentTypes)
	return out
}

// IsValidComponentType returns true when t is one of the 12 taxonomy types or
// "unknown".
func IsValidComponentType(t ComponentType) bool {
	return validComponentTypes[t]
}

// ComponentTypeDescription returns a short human-readable description for each
// component type, suitable for documentation generation and CLI help text.
func ComponentTypeDescription(t ComponentType) string {
	switch t {
	case ComponentTypeService:
		return "Application service (API, backend, worker)"
	case ComponentTypeDatabase:
		return "Data store (relational, document, key-value)"
	case ComponentTypeCache:
		return "In-memory caching layer"
	case ComponentTypeQueue:
		return "Message queue for async task processing"
	case ComponentTypeMessageBroker:
		return "Message broker / event streaming platform"
	case ComponentTypeLoadBalancer:
		return "Load balancer or reverse proxy"
	case ComponentTypeGateway:
		return "API gateway"
	case ComponentTypeStorage:
		return "Object or file storage"
	case ComponentTypeContainerRegistry:
		return "Container image registry"
	case ComponentTypeConfigServer:
		return "Configuration management service"
	case ComponentTypeMonitoring:
		return "Observability and monitoring platform"
	case ComponentTypeLogAggregator:
		return "Log collection and search platform"
	case ComponentTypeOrchestrator:
		return "Container orchestration platform"
	case ComponentTypeSecretsManager:
		return "Secrets management service"
	case ComponentTypeSearch:
		return "Search engine"
	case ComponentTypeUnknown:
		return "Unclassified component (default)"
	default:
		return "Unknown type"
	}
}

// componentTypePatterns maps keyword patterns (lowercase) to component types.
// Used by InferComponentType to classify components from their names and
// surrounding context.
var componentTypePatterns = map[ComponentType][]string{
	ComponentTypeService: {
		"service", "api", "server", "worker", "backend", "microservice",
		"app", "application", "rest api", "http endpoint", "graphql",
		"grpc service", "web service", "endpoint", "controller",
		"manager", "operator", "mendix", "mpr", "microflow", "nanoflow",
		"mendix app", "mendix-app",
	},
	ComponentTypeDatabase: {
		"database", "db", "postgres", "postgresql", "mysql", "mariadb",
		"mongodb", "mongo", "dynamodb", "cockroachdb", "cassandra",
		"couchdb", "sqlite", "rds", "aurora", "datastore", "store",
		"data store", "persistence", "relational", "nosql", "sql",
	},
	ComponentTypeCache: {
		"cache", "redis", "memcached", "memcache", "varnish", "cdn",
		"elasticache",
	},
	ComponentTypeQueue: {
		"queue", "sqs", "celery", "sidekiq", "delayed-job", "bull",
		"beanstalkd",
	},
	ComponentTypeMessageBroker: {
		"kafka", "rabbitmq", "rabbit", "nats", "pulsar", "kinesis",
		"eventbridge", "event-bus", "message-broker", "broker",
		"event-stream", "pub/sub", "pubsub", "event streaming",
		"streaming platform",
	},
	ComponentTypeLoadBalancer: {
		"load-balancer", "loadbalancer", "lb", "haproxy", "envoy",
		"alb", "elb", "nlb", "reverse proxy", "nginx", "proxy",
	},
	ComponentTypeGateway: {
		"gateway", "api-gateway", "kong", "traefik", "ambassador",
		"ingress",
	},
	ComponentTypeStorage: {
		"storage", "s3", "gcs", "minio", "blob", "bucket", "object-store",
		"file-store",
	},
	ComponentTypeContainerRegistry: {
		"registry", "ecr", "gcr", "docker-hub", "harbor",
		"container-registry",
	},
	ComponentTypeConfigServer: {
		"config", "consul", "etcd", "vault", "config-server",
		"spring-cloud-config", "zookeeper",
	},
	ComponentTypeMonitoring: {
		"monitoring", "prometheus", "datadog", "grafana", "new-relic",
		"newrelic", "nagios", "pagerduty", "alertmanager", "observability",
	},
	ComponentTypeLogAggregator: {
		"log", "logging", "elasticsearch", "elk", "splunk", "loki",
		"fluentd", "logstash", "kibana", "log-aggregator",
	},
	ComponentTypeOrchestrator: {
		"orchestrator", "kubernetes", "k8s", "nomad", "mesos",
		"docker-swarm", "swarm", "openshift", "rancher",
	},
	ComponentTypeSecretsManager: {
		"secrets-manager", "secrets", "vault", "aws-secrets-manager",
		"secretsmanager", "azure-key-vault", "key-vault",
	},
	ComponentTypeSearch: {
		"search", "elasticsearch", "solr", "algolia", "meilisearch",
		"opensearch",
	},
}

// InferComponentType attempts to classify a component based on its name and
// optional context strings.  Returns the best-matching ComponentType and a
// confidence score in [0.4, 1.0].
//
// Matching priority:
//  1. Exact type name match (e.g. name == "database") -> 0.95
//  2. Pattern substring match in name -> 0.85 (boosted by +0.05 per additional pattern match)
//  3. Pattern substring match in context -> 0.65-0.75 (weighted by position: title > early context > late context)
//  4. No match -> (ComponentTypeUnknown, 0.5)
func InferComponentType(name string, context ...string) (ComponentType, float64) {
	lowerName := strings.ToLower(name)

	// Priority 1: exact match against type name.
	for _, ct := range allComponentTypes {
		if ct == ComponentTypeUnknown {
			continue
		}
		if lowerName == string(ct) {
			return ct, 0.95
		}
	}

	// Priority 2: pattern match in name with multi-pattern boosting.
	// Track best match by specificity (longer pattern = more specific).
	// Count additional matches to boost confidence.
	bestType := ComponentTypeUnknown
	bestLen := 0
	matchCounts := make(map[ComponentType]int)

	for ct, patterns := range componentTypePatterns {
		for _, p := range patterns {
			if strings.Contains(lowerName, p) {
				matchCounts[ct]++
				if len(p) > bestLen {
					bestType = ct
					bestLen = len(p)
				}
			}
		}
	}

	if bestType != ComponentTypeUnknown {
		// Base confidence 0.85, boost by 0.05 per additional pattern (max 0.95)
		conf := 0.85 + float64(matchCounts[bestType]-1)*0.05
		if conf > 0.95 {
			conf = 0.95
		}
		return bestType, conf
	}

	// Priority 3: pattern match in context strings, weighted by position.
	// First context (usually title) has higher weight than later contexts.
	for idx, ctx := range context {
		lowerCtx := strings.ToLower(ctx)
		// Weight: 0.75 for first context (title), 0.70 for second, 0.65 for rest
		baseConf := 0.75
		if idx == 1 {
			baseConf = 0.70
		} else if idx > 1 {
			baseConf = 0.65
		}

		for ct, patterns := range componentTypePatterns {
			for _, p := range patterns {
				if strings.Contains(lowerCtx, p) && len(p) > bestLen {
					bestType = ct
					bestLen = len(p)
					// Return immediately with position-weighted confidence
					return bestType, baseConf
				}
			}
		}
	}

	return ComponentTypeUnknown, 0.5
}

// SeedConfig represents user-supplied type mappings that override automatic
// detection.  Loaded from a YAML configuration file.
type SeedConfig struct {
	// TypeMappings maps component name patterns to component types.
	// Pattern matching is case-insensitive substring.
	// Example: {"redis*": "cache", "postgres*": "database"}
	TypeMappings []SeedMapping `yaml:"type_mappings"`
}

// SeedMapping is a single pattern-to-type mapping in the seed configuration.
type SeedMapping struct {
	// Pattern is a case-insensitive substring or glob pattern to match
	// against component names.
	Pattern string `yaml:"pattern"`

	// Type is the component type to assign when the pattern matches.
	Type ComponentType `yaml:"type"`
}

// ApplySeedConfig checks name against seed config mappings and returns the
// matching type and confidence.  Seed config matches have highest priority
// (confidence 1.0).  Returns ("", 0) when no mapping matches.
func (sc *SeedConfig) ApplySeedConfig(name string) (ComponentType, float64) {
	if sc == nil {
		return "", 0
	}
	lowerName := strings.ToLower(name)
	for _, m := range sc.TypeMappings {
		pattern := strings.ToLower(m.Pattern)
		// Support simple glob: "redis*" matches "redis-cache", "redis-cluster", etc.
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(lowerName, prefix) {
				return m.Type, 1.0
			}
		} else if lowerName == pattern || strings.Contains(lowerName, pattern) {
			return m.Type, 1.0
		}
	}
	return "", 0
}

// TraversalState tracks visited nodes, the current DFS path (for cycle
// detection), detected cycles, and traversal depth during graph traversal.
type TraversalState struct {
	// Visited maps node ID → true when the node has been fully processed.
	Visited map[string]bool

	// Path holds the current DFS ancestor chain (for back-edge detection).
	Path []string

	// pathSet provides O(1) membership testing for Path.
	pathSet map[string]bool

	// Cycles collects all detected cycles as slices of node IDs.
	Cycles [][]string

	// Depth is the current traversal depth (incremented on descent).
	Depth int
}

// NewTraversalState returns a ready-to-use TraversalState.
func NewTraversalState() *TraversalState {
	return &TraversalState{
		Visited: make(map[string]bool),
		pathSet: make(map[string]bool),
	}
}

// HasVisited returns true if nodeID was already fully processed.
func (ts *TraversalState) HasVisited(nodeID string) bool {
	return ts.Visited[nodeID]
}

// MarkVisited records nodeID as fully processed.
func (ts *TraversalState) MarkVisited(nodeID string) {
	ts.Visited[nodeID] = true
}

// IsInPath returns true if nodeID is an ancestor in the current DFS path,
// meaning adding an edge to it would create a cycle.
func (ts *TraversalState) IsInPath(nodeID string) bool {
	return ts.pathSet[nodeID]
}

// AddPathNode appends nodeID to the current DFS path.
func (ts *TraversalState) AddPathNode(nodeID string) {
	ts.Path = append(ts.Path, nodeID)
	ts.pathSet[nodeID] = true
}

// RemovePathNode pops the last node from the current DFS path.
func (ts *TraversalState) RemovePathNode() {
	if len(ts.Path) == 0 {
		return
	}
	last := ts.Path[len(ts.Path)-1]
	ts.Path = ts.Path[:len(ts.Path)-1]
	delete(ts.pathSet, last)
}

// RecordCycle appends a detected cycle to the cycles list. The caller should
// pass a copy of the path forming the cycle.
func (ts *TraversalState) RecordCycle(cycle []string) {
	ts.Cycles = append(ts.Cycles, cycle)
}

// AtMaxDepth returns true when Depth >= maxDepth, indicating traversal should
// not descend further.
func (ts *TraversalState) AtMaxDepth(maxDepth int) bool {
	return ts.Depth >= maxDepth
}

// AffectedNode represents a node reached during a query traversal, serialized
// in JSON results so agents can see what components are affected and at what
// distance from the root.
type AffectedNode struct {
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	Confidence       float64 `json:"confidence"`
	RelationshipType string  `json:"relationship_type"` // "direct-dependency" or "cyclic-dependency"
	Distance         int     `json:"distance"`          // hops from root node
}

// QueryEdge represents a single edge in a query result, including full
// provenance so agents can assess evidence quality.
type QueryEdge struct {
	From             string  `json:"from"`
	To               string  `json:"to"`
	Confidence       float64 `json:"confidence"`
	Type             string  `json:"type"`
	RelationshipType string  `json:"relationship_type"`
	Evidence         string  `json:"evidence"`
	SourceFile       string  `json:"source_file"`
	ExtractionMethod string  `json:"extraction_method"`
	EvidencePointer  string  `json:"evidence_pointer"`
	SignalsCount     int     `json:"signals_count"`
}

// QueryResult is the top-level JSON structure returned by impact and crawl
// queries. It contains the full subgraph topology plus metadata for agent
// consumption.
type QueryResult struct {
	Query         string                 `json:"query"`
	Root          string                 `json:"root"`
	Depth         int                    `json:"depth"`
	TraverseMode  string                 `json:"traverse_mode"`
	MinConfidence float64                `json:"min_confidence"`
	MinTier       string                 `json:"min_tier"`
	AffectedNodes []AffectedNode         `json:"affected_nodes"`
	Edges         []QueryEdge            `json:"edges"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// String returns a pretty-printed JSON representation of the QueryResult
// for debugging purposes.
func (qr *QueryResult) String() string {
	b, err := json.MarshalIndent(qr, "", "  ")
	if err != nil {
		return fmt.Sprintf("QueryResult{query=%q, root=%q, error=%v}", qr.Query, qr.Root, err)
	}
	return string(b)
}

// Validate checks that the QueryResult has structural integrity:
// - Root is non-empty
// - AffectedNodes and Edges are non-empty for a valid result
// - Every edge from/to appears in AffectedNodes
func (qr *QueryResult) Validate() error {
	if qr.Root == "" {
		return fmt.Errorf("QueryResult.Validate: root must not be empty")
	}
	if len(qr.AffectedNodes) == 0 {
		return fmt.Errorf("QueryResult.Validate: affected_nodes must not be empty")
	}
	if len(qr.Edges) == 0 {
		return fmt.Errorf("QueryResult.Validate: edges must not be empty")
	}

	nodeSet := make(map[string]bool, len(qr.AffectedNodes))
	for _, n := range qr.AffectedNodes {
		nodeSet[n.Name] = true
	}
	for _, e := range qr.Edges {
		if !nodeSet[e.From] {
			return fmt.Errorf("QueryResult.Validate: edge from %q has no corresponding affected_node", e.From)
		}
		if !nodeSet[e.To] {
			return fmt.Errorf("QueryResult.Validate: edge to %q has no corresponding affected_node", e.To)
		}
	}
	return nil
}

// RelationshipLocation tracks where a relationship was detected in the source
// documentation. Used for deduplication (same file:line = same evidence) and
// traceability (agents can see exactly where evidence was found).
type RelationshipLocation struct {
	// File is the relative path to the source file (no leading /).
	File string

	// Line is the line number within File where the relationship was detected (0-indexed is valid).
	Line int

	// ByteOffset is the byte offset within File for precise positioning.
	ByteOffset int

	// Evidence is a short snippet of text around the detection point.
	Evidence string
}

// RelationshipLocationKey returns a deterministic deduplication key for a
// location in the format "file:line". Signals at the same file:line are
// considered duplicates regardless of which algorithm detected them.
func RelationshipLocationKey(loc RelationshipLocation) string {
	return fmt.Sprintf("%s:%d", loc.File, loc.Line)
}

// String returns a human-readable representation of the location.
func (loc RelationshipLocation) String() string {
	if loc.Evidence != "" {
		return fmt.Sprintf("%s:%d (%s)", loc.File, loc.Line, loc.Evidence)
	}
	return fmt.Sprintf("%s:%d", loc.File, loc.Line)
}

// IsValid returns true when the location has a non-empty, relative file path
// and a non-negative line number.
func (loc RelationshipLocation) IsValid() bool {
	return loc.File != "" && !strings.HasPrefix(loc.File, "/") && loc.Line >= 0
}
