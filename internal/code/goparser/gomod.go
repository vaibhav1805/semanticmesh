package goparser

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"golang.org/x/mod/modfile"
)

// ModuleMapping maps a Go module path prefix to a detected infrastructure component.
type ModuleMapping struct {
	// Prefix is matched against require paths (longest prefix wins).
	Prefix string

	// ComponentName is the target component name for the signal.
	ComponentName string

	// TargetType classifies the dependency: service, database, cache, message-broker, etc.
	TargetType string

	// Kind is the detection_kind for the signal.
	Kind string

	// Confidence for this detection.
	Confidence float64
}

// DefaultModuleMappings contains known Go modules that indicate infrastructure dependencies.
// Ordered by specificity (more specific prefixes first within each category).
var DefaultModuleMappings = []ModuleMapping{
	// --- AWS SDK services ---
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/s3", ComponentName: "aws-s3", TargetType: "storage", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/sqs", ComponentName: "aws-sqs", TargetType: "queue", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/sns", ComponentName: "aws-sns", TargetType: "queue", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/dynamodb", ComponentName: "aws-dynamodb", TargetType: "database", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/secretsmanager", ComponentName: "aws-secrets-manager", TargetType: "service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/ecr", ComponentName: "aws-ecr", TargetType: "container-registry", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/sts", ComponentName: "aws-sts", TargetType: "service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/iam", ComponentName: "aws-iam", TargetType: "service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/lambda", ComponentName: "aws-lambda", TargetType: "service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/kinesis", ComponentName: "aws-kinesis", TargetType: "queue", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/cloudwatch", ComponentName: "aws-cloudwatch", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/rds", ComponentName: "aws-rds", TargetType: "database", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go-v2/service/elasticache", ComponentName: "aws-elasticache", TargetType: "cache", Kind: "sdk_import", Confidence: 0.9},
	// AWS SDK v1 (legacy but widely used)
	{Prefix: "github.com/aws/aws-sdk-go/service/s3", ComponentName: "aws-s3", TargetType: "storage", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/sqs", ComponentName: "aws-sqs", TargetType: "queue", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/sns", ComponentName: "aws-sns", TargetType: "queue", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/dynamodb", ComponentName: "aws-dynamodb", TargetType: "database", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/secretsmanager", ComponentName: "aws-secrets-manager", TargetType: "service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/ecr", ComponentName: "aws-ecr", TargetType: "container-registry", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/sts", ComponentName: "aws-sts", TargetType: "service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/kinesis", ComponentName: "aws-kinesis", TargetType: "queue", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/aws/aws-sdk-go/service/rds", ComponentName: "aws-rds", TargetType: "database", Kind: "sdk_import", Confidence: 0.9},
	// General AWS SDK (if no specific service matched)
	{Prefix: "github.com/aws/aws-sdk-go-v2", ComponentName: "aws", TargetType: "cloud-platform", Kind: "sdk_import", Confidence: 0.7},
	{Prefix: "github.com/aws/aws-sdk-go", ComponentName: "aws", TargetType: "cloud-platform", Kind: "sdk_import", Confidence: 0.7},
	// AWS ACK controllers
	{Prefix: "github.com/aws-controllers-k8s/ecr-controller", ComponentName: "aws-ecr", TargetType: "container-registry", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "github.com/aws-controllers-k8s/s3-controller", ComponentName: "aws-s3", TargetType: "storage", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "github.com/aws-controllers-k8s/rds-controller", ComponentName: "aws-rds", TargetType: "database", Kind: "sdk_import", Confidence: 0.85},

	// --- Databases ---
	{Prefix: "database/sql", ComponentName: "sql-database", TargetType: "database", Kind: "sdk_import", Confidence: 0.8},
	{Prefix: "github.com/jmoiron/sqlx", ComponentName: "sql-database", TargetType: "database", Kind: "sdk_import", Confidence: 0.8},
	{Prefix: "gorm.io/gorm", ComponentName: "sql-database", TargetType: "database", Kind: "sdk_import", Confidence: 0.8},
	{Prefix: "github.com/jinzhu/gorm", ComponentName: "sql-database", TargetType: "database", Kind: "sdk_import", Confidence: 0.8},
	{Prefix: "go.mongodb.org/mongo-driver", ComponentName: "mongodb", TargetType: "database", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/lib/pq", ComponentName: "postgres", TargetType: "database", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "github.com/jackc/pgx", ComponentName: "postgres", TargetType: "database", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "github.com/go-sql-driver/mysql", ComponentName: "mysql", TargetType: "database", Kind: "sdk_import", Confidence: 0.85},

	// --- Caches ---
	{Prefix: "github.com/redis/go-redis", ComponentName: "redis", TargetType: "cache", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/go-redis/redis", ComponentName: "redis", TargetType: "cache", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/bradfitz/gomemcache", ComponentName: "memcached", TargetType: "cache", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/patrickmn/go-cache", ComponentName: "in-memory-cache", TargetType: "cache", Kind: "sdk_import", Confidence: 0.7},

	// --- Message brokers ---
	{Prefix: "github.com/IBM/sarama", ComponentName: "kafka", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/Shopify/sarama", ComponentName: "kafka", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/segmentio/kafka-go", ComponentName: "kafka", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/confluentinc/confluent-kafka-go", ComponentName: "kafka", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/rabbitmq/amqp091-go", ComponentName: "rabbitmq", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/streadway/amqp", ComponentName: "rabbitmq", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/nats-io/nats.go", ComponentName: "nats", TargetType: "message-broker", Kind: "sdk_import", Confidence: 0.9},

	// --- Observability ---
	{Prefix: "gopkg.in/DataDog/dd-trace-go.v1", ComponentName: "datadog", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/DataDog/datadog-go", ComponentName: "datadog", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/prometheus/client_golang", ComponentName: "prometheus", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "go.opentelemetry.io/otel", ComponentName: "opentelemetry", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "github.com/newrelic/go-agent", ComponentName: "new-relic", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/getsentry/sentry-go", ComponentName: "sentry", TargetType: "monitoring", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/elastic/go-elasticsearch", ComponentName: "elasticsearch", TargetType: "search", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/olivere/elastic", ComponentName: "elasticsearch", TargetType: "search", Kind: "sdk_import", Confidence: 0.9},

	// --- Kubernetes ---
	{Prefix: "k8s.io/client-go", ComponentName: "kubernetes", TargetType: "orchestrator", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "sigs.k8s.io/controller-runtime", ComponentName: "kubernetes", TargetType: "orchestrator", Kind: "sdk_import", Confidence: 0.85},
	{Prefix: "github.com/pivotal/kpack", ComponentName: "kpack", TargetType: "build-service", Kind: "sdk_import", Confidence: 0.9},
	{Prefix: "github.com/kubernetes-sigs/service-catalog", ComponentName: "service-catalog", TargetType: "service-broker", Kind: "sdk_import", Confidence: 0.9},

	// --- HTTP frameworks (indicate this is a service, not a dependency) ---
	// Skipped intentionally — these describe the service itself, not its deps.

	// --- gRPC ---
	{Prefix: "google.golang.org/grpc", ComponentName: "grpc", TargetType: "service", Kind: "sdk_import", Confidence: 0.8},

	// --- Secrets / config ---
	{Prefix: "github.com/hashicorp/vault/api", ComponentName: "vault", TargetType: "secrets-manager", Kind: "sdk_import", Confidence: 0.9},

	// --- Container registries ---
	{Prefix: "github.com/google/go-containerregistry", ComponentName: "container-registry", TargetType: "container-registry", Kind: "sdk_import", Confidence: 0.85},
}

// AnalyzeGoMod reads a go.mod file from the given project directory and returns
// CodeSignal values for recognized infrastructure dependencies.
// Returns nil, nil if no go.mod is found.
func AnalyzeGoMod(dir string) ([]code.CodeSignal, error) {
	goModPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	f, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return nil, err
	}

	// Build a sorted index of mappings (longest prefix first for correct matching).
	mappings := sortMappingsByPrefixLength(DefaultModuleMappings)

	// Track which component names we've already emitted to avoid duplicates.
	seen := make(map[string]bool)

	// First pass: collect all matched component names to detect specifics.
	allMatched := make(map[string]bool)
	for _, req := range f.Require {
		if req.Indirect {
			continue
		}
		if mapping, ok := matchModule(req.Mod.Path, mappings); ok {
			allMatched[mapping.ComponentName] = true
		}
	}

	var signals []code.CodeSignal

	for _, req := range f.Require {
		if req.Indirect {
			continue
		}

		mapping, ok := matchModule(req.Mod.Path, mappings)
		if !ok {
			continue
		}

		if seen[mapping.ComponentName] {
			continue
		}

		// Suppress general platform SDKs when specific services are detected.
		// e.g., skip "aws" when "aws-s3" or "aws-ecr" exists.
		if isGeneralSDK(mapping.ComponentName) && hasSpecificChild(mapping.ComponentName, allMatched) {
			continue
		}

		seen[mapping.ComponentName] = true

		lineNum := 0
		if req.Syntax != nil {
			lineNum = req.Syntax.Start.Line
		}

		signals = append(signals, code.CodeSignal{
			SourceFile:      "go.mod",
			LineNumber:      lineNum,
			TargetComponent: mapping.ComponentName,
			TargetType:      mapping.TargetType,
			DetectionKind:   mapping.Kind,
			Evidence:        req.Mod.Path + " " + req.Mod.Version,
			Language:        "go",
			Confidence:      mapping.Confidence,
		})
	}

	return signals, nil
}

// matchModule finds the best (longest prefix) mapping for a module path.
func matchModule(modulePath string, mappings []ModuleMapping) (ModuleMapping, bool) {
	for _, m := range mappings {
		if modulePath == m.Prefix || strings.HasPrefix(modulePath, m.Prefix+"/") {
			return m, true
		}
	}
	return ModuleMapping{}, false
}

// generalSDKs are platform-level SDKs that should be suppressed when more
// specific service-level components are detected.
var generalSDKs = map[string]bool{
	"aws":        true,
	"kubernetes": true,
}

// isGeneralSDK returns true if the component name is a general-purpose SDK.
func isGeneralSDK(name string) bool {
	return generalSDKs[name]
}

// hasSpecificChild returns true if allMatched contains any component prefixed
// with parent+"-" (e.g., "aws-s3" for parent "aws").
func hasSpecificChild(parent string, allMatched map[string]bool) bool {
	prefix := parent + "-"
	for name := range allMatched {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// sortMappingsByPrefixLength returns a copy sorted by descending prefix length
// so longer (more specific) prefixes match first.
func sortMappingsByPrefixLength(mappings []ModuleMapping) []ModuleMapping {
	sorted := make([]ModuleMapping, len(mappings))
	copy(sorted, mappings)
	// Simple insertion sort — small list, only runs once.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j].Prefix) > len(sorted[j-1].Prefix); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}
