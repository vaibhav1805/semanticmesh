package goparser

// DetectionPattern defines a mapping from a Go import + function call to an
// infrastructure dependency signal.
type DetectionPattern struct {
	// ImportPath is the full Go import path (e.g., "net/http").
	ImportPath string

	// Function is the function or method name called on the package (e.g., "Get").
	Function string

	// Kind is the detection_kind for the signal (e.g., "http_call", "db_connection").
	Kind string

	// TargetType classifies the dependency target (e.g., "service", "database", "cache").
	TargetType string

	// Confidence is the detection certainty in [0.4, 1.0].
	Confidence float64

	// ArgIndex is the position of the argument containing the target URL or
	// connection string. -1 means no argument extraction.
	ArgIndex int

	// FallbackTarget overrides the default import-path-derived target name
	// when no argument extraction succeeds. If empty, the last segment of
	// ImportPath is used.
	FallbackTarget string
}

// DefaultPatterns contains the built-in detection patterns for Go infrastructure calls.
var DefaultPatterns = []DetectionPattern{
	// HTTP calls
	{ImportPath: "net/http", Function: "Get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{ImportPath: "net/http", Function: "Post", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{ImportPath: "net/http", Function: "NewRequest", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 1},

	// Database connections
	{ImportPath: "database/sql", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 1},
	{ImportPath: "github.com/jmoiron/sqlx", Function: "Connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 1},
	{ImportPath: "github.com/jmoiron/sqlx", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 1},
	{ImportPath: "gorm.io/gorm", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{ImportPath: "go.mongodb.org/mongo-driver/mongo", Function: "Connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 1},

	// Cache clients
	{ImportPath: "github.com/redis/go-redis/v9", Function: "NewClient", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/go-redis/redis/v8", Function: "NewClient", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/bradfitz/gomemcache/memcache", Function: "New", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: 0},

	// Queue producers
	{ImportPath: "github.com/IBM/sarama", Function: "NewSyncProducer", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/IBM/sarama", Function: "NewAsyncProducer", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},

	// Queue consumers
	{ImportPath: "github.com/IBM/sarama", Function: "NewConsumer", Kind: "queue_consumer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/IBM/sarama", Function: "NewConsumerGroup", Kind: "queue_consumer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},

	// RabbitMQ
	{ImportPath: "github.com/rabbitmq/amqp091-go", Function: "Dial", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: 0},

	// NATS
	{ImportPath: "github.com/nats-io/nats.go", Function: "Connect", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: 0},

	// AWS SDK v2 services
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/sqs", Function: "NewFromConfig", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/s3", Function: "NewFromConfig", Kind: "storage_client", TargetType: "storage", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/s3", Function: "New", Kind: "storage_client", TargetType: "storage", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/dynamodb", Function: "NewFromConfig", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/ecr", Function: "NewFromConfig", Kind: "registry_client", TargetType: "container-registry", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/secretsmanager", Function: "NewFromConfig", Kind: "secrets_client", TargetType: "secrets-manager", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/sns", Function: "NewFromConfig", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/kinesis", Function: "NewFromConfig", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/sts", Function: "NewFromConfig", Kind: "auth_client", TargetType: "service", Confidence: 0.85, ArgIndex: -1},

	// AWS SDK v1
	{ImportPath: "github.com/aws/aws-sdk-go/service/s3", Function: "New", Kind: "storage_client", TargetType: "storage", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go/service/sqs", Function: "New", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go/service/dynamodb", Function: "New", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go/service/ecr", Function: "New", Kind: "registry_client", TargetType: "container-registry", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go/service/secretsmanager", Function: "New", Kind: "secrets_client", TargetType: "secrets-manager", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/aws/aws-sdk-go/service/sns", Function: "New", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},

	// Datadog
	{ImportPath: "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer", Function: "Start", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "datadog"},
	{ImportPath: "github.com/DataDog/datadog-go/v5/statsd", Function: "New", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.9, ArgIndex: 0, FallbackTarget: "datadog"},

	// Prometheus
	{ImportPath: "github.com/prometheus/client_golang/prometheus", Function: "NewCounter", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.8, ArgIndex: -1, FallbackTarget: "prometheus"},
	{ImportPath: "github.com/prometheus/client_golang/prometheus", Function: "NewGauge", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.8, ArgIndex: -1, FallbackTarget: "prometheus"},
	{ImportPath: "github.com/prometheus/client_golang/prometheus", Function: "NewHistogram", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.8, ArgIndex: -1, FallbackTarget: "prometheus"},

	// gRPC
	{ImportPath: "google.golang.org/grpc", Function: "Dial", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{ImportPath: "google.golang.org/grpc", Function: "DialContext", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 1},
	{ImportPath: "google.golang.org/grpc", Function: "NewClient", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},

	// Vault
	{ImportPath: "github.com/hashicorp/vault/api", Function: "NewClient", Kind: "secrets_client", TargetType: "secrets-manager", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "vault"},

	// Elasticsearch
	{ImportPath: "github.com/elastic/go-elasticsearch/v8", Function: "NewClient", Kind: "search_client", TargetType: "search", Confidence: 0.85, ArgIndex: -1, FallbackTarget: "elasticsearch"},
	{ImportPath: "github.com/olivere/elastic", Function: "NewClient", Kind: "search_client", TargetType: "search", Confidence: 0.85, ArgIndex: -1, FallbackTarget: "elasticsearch"},

	// Container registry
	{ImportPath: "github.com/google/go-containerregistry/pkg/crane", Function: "Pull", Kind: "registry_client", TargetType: "container-registry", Confidence: 0.85, ArgIndex: 0},
	{ImportPath: "github.com/google/go-containerregistry/pkg/crane", Function: "Push", Kind: "registry_client", TargetType: "container-registry", Confidence: 0.85, ArgIndex: -1},

	// Sentry
	{ImportPath: "github.com/getsentry/sentry-go", Function: "Init", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "sentry"},

	// OpenTelemetry
	{ImportPath: "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc", Function: "New", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.85, ArgIndex: -1, FallbackTarget: "opentelemetry"},

	// ========== REST API Servers (CRITICAL - high detection value) ==========
	// Gin framework
	{ImportPath: "github.com/gin-gonic/gin", Function: "New", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	{ImportPath: "github.com/gin-gonic/gin", Function: "Default", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	// Echo framework
	{ImportPath: "github.com/labstack/echo/v4", Function: "New", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	// Fiber framework
	{ImportPath: "github.com/gofiber/fiber/v2", Function: "New", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	// Chi router
	{ImportPath: "github.com/go-chi/chi/v5", Function: "NewRouter", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	// Gorilla Mux
	{ImportPath: "github.com/gorilla/mux", Function: "NewRouter", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	// Standard library HTTP server
	{ImportPath: "net/http", Function: "ListenAndServe", Kind: "http_server", TargetType: "service", Confidence: 0.95, ArgIndex: 0},
	{ImportPath: "net/http", Function: "ListenAndServeTLS", Kind: "http_server", TargetType: "service", Confidence: 0.95, ArgIndex: 0},
	{ImportPath: "net/http", Function: "Serve", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},
	// GraphQL servers
	{ImportPath: "github.com/graphql-go/graphql", Function: "NewSchema", Kind: "http_server", TargetType: "service", Confidence: 0.85, ArgIndex: -1},
	{ImportPath: "github.com/99designs/gqlgen/graphql/handler", Function: "NewDefaultServer", Kind: "http_server", TargetType: "service", Confidence: 0.9, ArgIndex: -1},

	// ========== Kubernetes Integrations (HIGH VALUE) ==========
	// controller-runtime
	{ImportPath: "sigs.k8s.io/controller-runtime/pkg/client", Function: "New", Kind: "k8s_client", TargetType: "orchestrator", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "kubernetes"},
	{ImportPath: "sigs.k8s.io/controller-runtime/pkg/manager", Function: "New", Kind: "k8s_client", TargetType: "orchestrator", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "kubernetes"},
	// client-go
	{ImportPath: "k8s.io/client-go/kubernetes", Function: "NewForConfig", Kind: "k8s_client", TargetType: "orchestrator", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "kubernetes"},
	{ImportPath: "k8s.io/client-go/rest", Function: "InClusterConfig", Kind: "k8s_client", TargetType: "orchestrator", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "kubernetes"},
	{ImportPath: "k8s.io/client-go/dynamic", Function: "NewForConfig", Kind: "k8s_client", TargetType: "orchestrator", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "kubernetes"},

	// ========== Enhanced Database Patterns ==========
	// pgx (modern PostgreSQL driver)
	{ImportPath: "github.com/jackc/pgx/v5/pgxpool", Function: "Connect", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 1, FallbackTarget: "postgresql"},
	{ImportPath: "github.com/jackc/pgx/v5/pgxpool", Function: "New", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 1, FallbackTarget: "postgresql"},
	{ImportPath: "github.com/jackc/pgx/v5", Function: "Connect", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 1, FallbackTarget: "postgresql"},
	// GORM dialectors
	{ImportPath: "gorm.io/driver/postgres", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 0, FallbackTarget: "postgresql"},
	{ImportPath: "gorm.io/driver/mysql", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 0, FallbackTarget: "mysql"},
	{ImportPath: "gorm.io/driver/sqlite", Function: "Open", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 0, FallbackTarget: "sqlite"},
	// Cassandra
	{ImportPath: "github.com/gocql/gocql", Function: "NewCluster", Kind: "db_connection", TargetType: "database", Confidence: 0.9, ArgIndex: 0, FallbackTarget: "cassandra"},
	// InfluxDB
	{ImportPath: "github.com/influxdata/influxdb-client-go/v2", Function: "New", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0, FallbackTarget: "influxdb"},

	// ========== HTTP Client Libraries ==========
	// go-resty
	{ImportPath: "github.com/go-resty/resty/v2", Function: "New", Kind: "http_call", TargetType: "service", Confidence: 0.85, ArgIndex: -1},
	// Additional net/http methods
	{ImportPath: "net/http", Function: "Head", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{ImportPath: "net/http", Function: "Put", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{ImportPath: "net/http", Function: "Patch", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{ImportPath: "net/http", Function: "Delete", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},

	// ========== Additional Message Brokers ==========
	// Pulsar
	{ImportPath: "github.com/apache/pulsar-client-go/pulsar", Function: "NewClient", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "pulsar"},
	// NSQ
	{ImportPath: "github.com/nsqio/go-nsq", Function: "NewProducer", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.9, ArgIndex: 0, FallbackTarget: "nsq"},
	{ImportPath: "github.com/nsqio/go-nsq", Function: "NewConsumer", Kind: "queue_consumer", TargetType: "message-broker", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "nsq"},
	// AWS Kinesis (additional patterns)
	{ImportPath: "github.com/aws/aws-sdk-go/service/kinesis", Function: "New", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1, FallbackTarget: "kinesis"},
	// AWS EventBridge
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/eventbridge", Function: "NewFromConfig", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1, FallbackTarget: "eventbridge"},
	{ImportPath: "github.com/aws/aws-sdk-go/service/eventbridge", Function: "New", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1, FallbackTarget: "eventbridge"},

	// ========== Configuration Management ==========
	// Consul
	{ImportPath: "github.com/hashicorp/consul/api", Function: "NewClient", Kind: "config_client", TargetType: "config-server", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "consul"},
	// etcd
	{ImportPath: "go.etcd.io/etcd/client/v3", Function: "New", Kind: "config_client", TargetType: "config-server", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "etcd"},
	// Viper (configuration library)
	{ImportPath: "github.com/spf13/viper", Function: "New", Kind: "config_client", TargetType: "config-server", Confidence: 0.7, ArgIndex: -1},

	// ========== Enhanced Monitoring ==========
	// Jaeger
	{ImportPath: "github.com/jaegertracing/jaeger-client-go", Function: "NewTracer", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "jaeger"},
	// Zipkin
	{ImportPath: "github.com/openzipkin/zipkin-go", Function: "NewTracer", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "zipkin"},
	// StatsD
	{ImportPath: "github.com/cactus/go-statsd-client/v5/statsd", Function: "NewClient", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.85, ArgIndex: 0, FallbackTarget: "statsd"},
	// Prometheus Summary
	{ImportPath: "github.com/prometheus/client_golang/prometheus", Function: "NewSummary", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.8, ArgIndex: -1, FallbackTarget: "prometheus"},
	// New Relic
	{ImportPath: "github.com/newrelic/go-agent/v3/newrelic", Function: "NewApplication", Kind: "monitoring_client", TargetType: "monitoring", Confidence: 0.9, ArgIndex: -1, FallbackTarget: "newrelic"},
}

// buildPatternIndex creates a lookup map keyed by "importPath.Function".
func buildPatternIndex(patterns []DetectionPattern) map[string]DetectionPattern {
	idx := make(map[string]DetectionPattern, len(patterns))
	for _, p := range patterns {
		key := p.ImportPath + "." + p.Function
		idx[key] = p
	}
	return idx
}

