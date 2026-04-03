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

	// AWS SQS
	{ImportPath: "github.com/aws/aws-sdk-go-v2/service/sqs", Function: "NewFromConfig", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},
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

