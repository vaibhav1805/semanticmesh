package pyparser

import "regexp"

// PyDetectionPattern defines a mapping from a Python package + function call to an
// infrastructure dependency signal.
type PyDetectionPattern struct {
	// Package is the Python package name (e.g., "requests", "psycopg2").
	Package string

	// Function is the function or method name (e.g., "get", "connect").
	// For from-imports where the function IS the class (e.g., from redis import Redis),
	// this matches the bare call name.
	Function string

	// Kind is the detection_kind for the signal (e.g., "http_call", "db_connection").
	Kind string

	// TargetType classifies the dependency target (e.g., "service", "database", "cache").
	TargetType string

	// Confidence is the detection certainty in [0.4, 1.0].
	Confidence float64

	// ArgIndex is the position of the argument containing the target URL/connection string.
	// -1 means use keyword argument extraction instead.
	ArgIndex int
}

// DefaultPythonPatterns contains the built-in detection patterns for Python infrastructure calls.
var DefaultPythonPatterns = []PyDetectionPattern{
	// HTTP calls - requests library
	{Package: "requests", Function: "get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "requests", Function: "post", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "requests", Function: "put", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "requests", Function: "delete", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "requests", Function: "patch", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},

	// HTTP calls - httpx library
	{Package: "httpx", Function: "get", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "httpx", Function: "post", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "httpx", Function: "put", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},
	{Package: "httpx", Function: "delete", Kind: "http_call", TargetType: "service", Confidence: 0.9, ArgIndex: 0},

	// Database connections
	{Package: "psycopg2", Function: "connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{Package: "asyncpg", Function: "connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{Package: "asyncpg", Function: "create_pool", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{Package: "pymongo", Function: "MongoClient", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{Package: "sqlalchemy", Function: "create_engine", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},
	{Package: "pymysql", Function: "connect", Kind: "db_connection", TargetType: "database", Confidence: 0.85, ArgIndex: 0},

	// Cache clients
	{Package: "redis", Function: "Redis", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: -1},
	{Package: "redis", Function: "StrictRedis", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: -1},
	{Package: "redis", Function: "from_url", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: 0},
	{Package: "pymemcache.client", Function: "Client", Kind: "cache_client", TargetType: "cache", Confidence: 0.85, ArgIndex: 0},

	// Queue - Kafka
	{Package: "kafka", Function: "KafkaProducer", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},
	{Package: "kafka", Function: "KafkaConsumer", Kind: "queue_consumer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},

	// Queue - RabbitMQ (pika)
	{Package: "pika", Function: "BlockingConnection", Kind: "queue_producer", TargetType: "message-broker", Confidence: 0.85, ArgIndex: -1},

	// Queue - AWS SQS (boto3)
	{Package: "boto3", Function: "client", Kind: "queue_producer", TargetType: "queue", Confidence: 0.85, ArgIndex: -1},
}

// Import regexes for Python import statements.
var (
	// bareImportRe matches: import requests
	bareImportRe = regexp.MustCompile(`^import\s+(\w+(?:\.\w+)*)\s*$`)

	// aliasedImportRe matches: import requests as req
	aliasedImportRe = regexp.MustCompile(`^import\s+(\w+(?:\.\w+)*)\s+as\s+(\w+)\s*$`)

	// fromImportRe matches: from redis import Redis
	// and: from redis import Redis as R
	// and: from pymemcache.client import Client
	fromImportRe = regexp.MustCompile(`^from\s+(\w+(?:\.\w+)*)\s+import\s+(\w+)(?:\s+as\s+(\w+))?\s*$`)
)

// callPatternRe matches Python function/method calls like:
//
//	requests.get("http://...")
//	req.post("http://...")
//	Redis(host="cache-01")
//	KafkaProducer(bootstrap_servers="broker:9092")
//
// Group 1: optional object (e.g., "requests" in requests.get)
// Group 2: function name (e.g., "get")
// Group 3: argument string (everything inside parentheses)
var callPatternRe = regexp.MustCompile(`(?:(\w+)\.)?(\w+)\(([^)]*)\)`)

// stringArgRe extracts the first quoted string argument from a call's argument list.
var stringArgRe = regexp.MustCompile(`["']([^"']+)["']`)

// kwargHostRe extracts host= keyword argument value.
var kwargHostRe = regexp.MustCompile(`host\s*=\s*["']([^"']+)["']`)

// kwargBootstrapRe extracts bootstrap_servers= keyword argument value.
var kwargBootstrapRe = regexp.MustCompile(`bootstrap_servers\s*=\s*["']([^"']+)["']`)

// pikaParamsRe extracts the host from pika.ConnectionParameters("host").
var pikaParamsRe = regexp.MustCompile(`ConnectionParameters\(\s*["']([^"']+)["']`)

// boto3SQSArgRe checks if boto3.client call has "sqs" as first argument.
var boto3SQSArgRe = regexp.MustCompile(`["']sqs["']`)

// buildPatternIndex creates a lookup map keyed by "package.function".
func buildPatternIndex(patterns []PyDetectionPattern) map[string]PyDetectionPattern {
	idx := make(map[string]PyDetectionPattern, len(patterns))
	for _, p := range patterns {
		key := p.Package + "." + p.Function
		idx[key] = p
	}
	return idx
}
