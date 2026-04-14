package tfparser

import "strings"

// TFResourcePattern maps a Terraform resource type prefix to an infrastructure component type.
type TFResourcePattern struct {
	// Prefix is matched as a prefix of the resource type string (e.g., "aws_db_instance").
	// More specific prefixes should appear before broader ones — first match wins.
	Prefix string

	// TargetType is the semanticmesh component type assigned on a match.
	TargetType string

	// Confidence is the detection certainty in [0.4, 1.0].
	Confidence float64
}

// TFModulePattern maps a known Terraform registry module source to a component type.
type TFModulePattern struct {
	// Prefix is matched as a substring of the module source string.
	Prefix string

	// TargetType is the component type for this module.
	TargetType string
}

// DefaultResourcePatterns maps Terraform resource type prefixes to component types.
// Evaluated in order; the first matching prefix wins — put more specific entries first.
var DefaultResourcePatterns = []TFResourcePattern{
	// ── AWS Databases ─────────────────────────────────────────────────────────
	{Prefix: "aws_db_instance", TargetType: "database", Confidence: 0.92},
	{Prefix: "aws_db_cluster", TargetType: "database", Confidence: 0.92},
	{Prefix: "aws_rds_cluster", TargetType: "database", Confidence: 0.92},
	{Prefix: "aws_rds_", TargetType: "database", Confidence: 0.85},
	{Prefix: "aws_dynamodb_table", TargetType: "database", Confidence: 0.92},
	{Prefix: "aws_dynamodb_", TargetType: "database", Confidence: 0.85},
	{Prefix: "aws_docdb_", TargetType: "database", Confidence: 0.88},
	{Prefix: "aws_redshift_", TargetType: "database", Confidence: 0.88},
	{Prefix: "aws_neptune_", TargetType: "database", Confidence: 0.88},
	{Prefix: "aws_timestream_", TargetType: "database", Confidence: 0.85},

	// ── AWS Cache ─────────────────────────────────────────────────────────────
	{Prefix: "aws_elasticache_cluster", TargetType: "cache", Confidence: 0.92},
	{Prefix: "aws_elasticache_replication_group", TargetType: "cache", Confidence: 0.92},
	{Prefix: "aws_elasticache_", TargetType: "cache", Confidence: 0.85},

	// ── AWS Queues / Message Brokers ──────────────────────────────────────────
	{Prefix: "aws_sqs_queue", TargetType: "queue", Confidence: 0.92},
	{Prefix: "aws_sqs_", TargetType: "queue", Confidence: 0.85},
	{Prefix: "aws_sns_topic", TargetType: "message-broker", Confidence: 0.88},
	{Prefix: "aws_sns_", TargetType: "message-broker", Confidence: 0.82},
	{Prefix: "aws_msk_cluster", TargetType: "message-broker", Confidence: 0.92},
	{Prefix: "aws_msk_", TargetType: "message-broker", Confidence: 0.85},
	{Prefix: "aws_kinesis_stream", TargetType: "message-broker", Confidence: 0.88},
	{Prefix: "aws_kinesis_", TargetType: "message-broker", Confidence: 0.82},
	{Prefix: "aws_eventbridge_", TargetType: "message-broker", Confidence: 0.82},

	// ── AWS Storage ───────────────────────────────────────────────────────────
	{Prefix: "aws_s3_bucket", TargetType: "storage", Confidence: 0.92},
	{Prefix: "aws_s3_", TargetType: "storage", Confidence: 0.85},
	{Prefix: "aws_efs_", TargetType: "storage", Confidence: 0.85},

	// ── AWS Load Balancers ────────────────────────────────────────────────────
	{Prefix: "aws_lb_", TargetType: "load-balancer", Confidence: 0.90},
	{Prefix: "aws_lb", TargetType: "load-balancer", Confidence: 0.90},
	{Prefix: "aws_alb_", TargetType: "load-balancer", Confidence: 0.90},
	{Prefix: "aws_alb", TargetType: "load-balancer", Confidence: 0.90},
	{Prefix: "aws_elb", TargetType: "load-balancer", Confidence: 0.88},

	// ── AWS API Gateway ───────────────────────────────────────────────────────
	{Prefix: "aws_api_gateway", TargetType: "gateway", Confidence: 0.92},
	{Prefix: "aws_apigatewayv2_", TargetType: "gateway", Confidence: 0.92},

	// ── AWS Orchestration ─────────────────────────────────────────────────────
	{Prefix: "aws_eks_cluster", TargetType: "orchestrator", Confidence: 0.92},
	{Prefix: "aws_eks_", TargetType: "orchestrator", Confidence: 0.85},
	{Prefix: "aws_ecs_cluster", TargetType: "orchestrator", Confidence: 0.88},
	{Prefix: "aws_ecs_", TargetType: "orchestrator", Confidence: 0.82},

	// ── AWS Compute / Services ────────────────────────────────────────────────
	{Prefix: "aws_lambda_function", TargetType: "service", Confidence: 0.92},
	{Prefix: "aws_lambda_", TargetType: "service", Confidence: 0.85},
	{Prefix: "aws_instance", TargetType: "service", Confidence: 0.80},
	{Prefix: "aws_autoscaling_group", TargetType: "service", Confidence: 0.80},
	{Prefix: "aws_apprunner_", TargetType: "service", Confidence: 0.85},

	// ── AWS Container Registry ────────────────────────────────────────────────
	{Prefix: "aws_ecr_repository", TargetType: "container-registry", Confidence: 0.92},
	{Prefix: "aws_ecr_", TargetType: "container-registry", Confidence: 0.85},

	// ── AWS Secrets / Config ──────────────────────────────────────────────────
	{Prefix: "aws_secretsmanager_secret", TargetType: "secrets-manager", Confidence: 0.92},
	{Prefix: "aws_secretsmanager_", TargetType: "secrets-manager", Confidence: 0.85},
	{Prefix: "aws_ssm_parameter", TargetType: "config-server", Confidence: 0.88},
	{Prefix: "aws_ssm_", TargetType: "config-server", Confidence: 0.80},
	{Prefix: "aws_appconfig_", TargetType: "config-server", Confidence: 0.82},

	// ── AWS Observability ─────────────────────────────────────────────────────
	{Prefix: "aws_cloudwatch_log_", TargetType: "log-aggregator", Confidence: 0.90},
	{Prefix: "aws_cloudwatch_", TargetType: "monitoring", Confidence: 0.85},

	// ── AWS Search ────────────────────────────────────────────────────────────
	{Prefix: "aws_elasticsearch_domain", TargetType: "search", Confidence: 0.92},
	{Prefix: "aws_opensearch_domain", TargetType: "search", Confidence: 0.92},
	{Prefix: "aws_opensearch_", TargetType: "search", Confidence: 0.85},

	// ── Kubernetes provider ───────────────────────────────────────────────────
	{Prefix: "kubernetes_deployment", TargetType: "service", Confidence: 0.85},
	{Prefix: "kubernetes_stateful_set", TargetType: "service", Confidence: 0.85},
	{Prefix: "kubernetes_daemonset", TargetType: "service", Confidence: 0.82},
	{Prefix: "kubernetes_cron_job", TargetType: "service", Confidence: 0.80},
	{Prefix: "kubernetes_ingress", TargetType: "gateway", Confidence: 0.85},
	{Prefix: "kubernetes_service", TargetType: "service", Confidence: 0.80},

	// ── Helm ─────────────────────────────────────────────────────────────────
	{Prefix: "helm_release", TargetType: "service", Confidence: 0.78},
}

// DefaultModulePatterns maps known Terraform registry module source substrings to component types.
var DefaultModulePatterns = []TFModulePattern{
	// terraform-aws-modules
	{Prefix: "terraform-aws-modules/rds", TargetType: "database"},
	{Prefix: "terraform-aws-modules/aurora", TargetType: "database"},
	{Prefix: "terraform-aws-modules/dynamodb-table", TargetType: "database"},
	{Prefix: "terraform-aws-modules/elasticache", TargetType: "cache"},
	{Prefix: "terraform-aws-modules/sqs", TargetType: "queue"},
	{Prefix: "terraform-aws-modules/msk", TargetType: "message-broker"},
	{Prefix: "terraform-aws-modules/kafka", TargetType: "message-broker"},
	{Prefix: "terraform-aws-modules/s3-bucket", TargetType: "storage"},
	{Prefix: "terraform-aws-modules/efs", TargetType: "storage"},
	{Prefix: "terraform-aws-modules/eks", TargetType: "orchestrator"},
	{Prefix: "terraform-aws-modules/ecs", TargetType: "orchestrator"},
	{Prefix: "terraform-aws-modules/lambda", TargetType: "service"},
	{Prefix: "terraform-aws-modules/alb", TargetType: "load-balancer"},
	{Prefix: "terraform-aws-modules/elb", TargetType: "load-balancer"},
	{Prefix: "terraform-aws-modules/api-gateway", TargetType: "gateway"},
	{Prefix: "terraform-aws-modules/ecr", TargetType: "container-registry"},
	{Prefix: "terraform-aws-modules/secrets-manager", TargetType: "secrets-manager"},
}

// lookupResourcePattern returns the first pattern whose Prefix matches the start of resourceType.
func lookupResourcePattern(resourceType string) (TFResourcePattern, bool) {
	lower := strings.ToLower(resourceType)
	for _, p := range DefaultResourcePatterns {
		if strings.HasPrefix(lower, p.Prefix) {
			return p, true
		}
	}
	return TFResourcePattern{}, false
}

// lookupModulePattern returns the first pattern whose Prefix is a substring of source.
func lookupModulePattern(source string) (TFModulePattern, bool) {
	lower := strings.ToLower(source)
	for _, p := range DefaultModulePatterns {
		if strings.Contains(lower, p.Prefix) {
			return p, true
		}
	}
	return TFModulePattern{}, false
}
