package knowledge

import (
	"regexp"
	"strings"
)

// InfrastructurePattern defines a pattern for detecting infrastructure components
// mentioned in documentation text.
type InfrastructurePattern struct {
	// Name is the canonical component name (e.g., "dynamodb", "postgresql")
	Name string

	// Type is the component type (e.g., "database", "cache", "message-broker")
	Type ComponentType

	// Patterns are regex patterns or keywords to match in text
	Patterns []string

	// Confidence is the base confidence score for this detection
	Confidence float64

	// RequiresContext indicates if the pattern needs surrounding context to be valid
	RequiresContext bool
}

// infrastructurePatterns defines detection patterns for common infrastructure components.
var infrastructurePatterns = []InfrastructurePattern{
	// Databases
	{Name: "dynamodb", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\bdynamodb\b`}, Confidence: 0.85},
	{Name: "postgresql", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\b(postgresql|postgres)\b`}, Confidence: 0.85},
	{Name: "mysql", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\bmysql\b`}, Confidence: 0.85},
	{Name: "mongodb", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\b(mongodb|mongo)\b`}, Confidence: 0.85},
	{Name: "redis", Type: ComponentTypeCache, Patterns: []string{`(?i)\bredis\b`}, Confidence: 0.85},
	{Name: "cassandra", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\bcassandra\b`}, Confidence: 0.85},
	{Name: "aurora", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\b(aurora|aws aurora)\b`}, Confidence: 0.85},
	{Name: "rds", Type: ComponentTypeDatabase, Patterns: []string{`(?i)\b(rds|aws rds|amazon rds)\b`}, Confidence: 0.80},

	// Cloud Services - AWS
	{Name: "s3", Type: ComponentTypeStorage, Patterns: []string{`(?i)\b(s3|aws s3|amazon s3)\b`}, Confidence: 0.85},
	{Name: "aws-secrets-manager", Type: ComponentTypeSecretsManager, Patterns: []string{`(?i)\b(secrets manager|aws secrets manager|secretsmanager)\b`}, Confidence: 0.85},
	{Name: "kms", Type: ComponentTypeSecretsManager, Patterns: []string{`(?i)\b(kms|aws kms|key management service)\b`}, Confidence: 0.80},
	{Name: "cloudformation", Type: ComponentTypeConfigServer, Patterns: []string{`(?i)\b(cloudformation|aws cloudformation)\b`}, Confidence: 0.85},
	{Name: "sns", Type: ComponentTypeQueue, Patterns: []string{`(?i)\b(sns|aws sns)\b`}, Confidence: 0.80},
	{Name: "sqs", Type: ComponentTypeQueue, Patterns: []string{`(?i)\b(sqs|aws sqs)\b`}, Confidence: 0.80},
	{Name: "kinesis", Type: ComponentTypeQueue, Patterns: []string{`(?i)\b(kinesis|aws kinesis)\b`}, Confidence: 0.85},
	{Name: "lambda", Type: ComponentTypeService, Patterns: []string{`(?i)\b(lambda|aws lambda)\b`}, Confidence: 0.85},
	{Name: "ecr", Type: ComponentTypeContainerRegistry, Patterns: []string{`(?i)\b(ecr|aws ecr)\b`}, Confidence: 0.80},
	{Name: "eks", Type: ComponentTypeOrchestrator, Patterns: []string{`(?i)\b(eks|aws eks)\b`}, Confidence: 0.80},
	{Name: "iam", Type: ComponentTypeService, Patterns: []string{`(?i)\b(iam|aws iam)\b`}, Confidence: 0.75, RequiresContext: true},

	// Auth Systems
	{Name: "ldap", Type: ComponentTypeService, Patterns: []string{`(?i)\bldap\b`}, Confidence: 0.85},
	{Name: "active-directory", Type: ComponentTypeService, Patterns: []string{`(?i)\b(active directory|activedirectory)\b`}, Confidence: 0.80},
	{Name: "okta", Type: ComponentTypeService, Patterns: []string{`(?i)\bokta\b`}, Confidence: 0.85},
	{Name: "oauth", Type: ComponentTypeService, Patterns: []string{`(?i)\boauth\b`}, Confidence: 0.75},

	// Message Brokers
	{Name: "kafka", Type: ComponentTypeMessageBroker, Patterns: []string{`(?i)\b(kafka|apache kafka)\b`}, Confidence: 0.85},
	{Name: "rabbitmq", Type: ComponentTypeMessageBroker, Patterns: []string{`(?i)\brabbitmq\b`}, Confidence: 0.85},
	{Name: "nats", Type: ComponentTypeMessageBroker, Patterns: []string{`(?i)\bnats\b`}, Confidence: 0.85},
	{Name: "eventbridge", Type: ComponentTypeMessageBroker, Patterns: []string{`(?i)\beventbridge\b`}, Confidence: 0.85},

	// Monitoring & Observability
	{Name: "prometheus", Type: ComponentTypeMonitoring, Patterns: []string{`(?i)\bprometheus\b`}, Confidence: 0.85},
	{Name: "datadog", Type: ComponentTypeMonitoring, Patterns: []string{`(?i)\bdatadog\b`}, Confidence: 0.85},
	{Name: "grafana", Type: ComponentTypeMonitoring, Patterns: []string{`(?i)\bgrafana\b`}, Confidence: 0.85},
	{Name: "pagerduty", Type: ComponentTypeMonitoring, Patterns: []string{`(?i)\b(pagerduty|pager duty)\b`}, Confidence: 0.85},
	{Name: "new-relic", Type: ComponentTypeMonitoring, Patterns: []string{`(?i)\b(new relic|newrelic)\b`}, Confidence: 0.85},

	// Secrets Management
	{Name: "vault", Type: ComponentTypeSecretsManager, Patterns: []string{`(?i)\b(vault|hashicorp vault)\b`}, Confidence: 0.85},

	// Load Balancers
	{Name: "haproxy", Type: ComponentTypeLoadBalancer, Patterns: []string{`(?i)\bhaproxy\b`}, Confidence: 0.85},
	{Name: "nginx", Type: ComponentTypeLoadBalancer, Patterns: []string{`(?i)\bnginx\b`}, Confidence: 0.85},
	{Name: "envoy", Type: ComponentTypeLoadBalancer, Patterns: []string{`(?i)\benvoy\b`}, Confidence: 0.85},
	{Name: "alb", Type: ComponentTypeLoadBalancer, Patterns: []string{`(?i)\b(alb|application load balancer)\b`}, Confidence: 0.80},

	// Search
	{Name: "elasticsearch", Type: ComponentTypeSearch, Patterns: []string{`(?i)\belasticsearch\b`}, Confidence: 0.85},
	{Name: "opensearch", Type: ComponentTypeSearch, Patterns: []string{`(?i)\bopensearch\b`}, Confidence: 0.85},

	// Orchestration
	{Name: "kubernetes", Type: ComponentTypeOrchestrator, Patterns: []string{`(?i)\b(kubernetes|k8s)\b`}, Confidence: 0.85},
	{Name: "docker", Type: ComponentTypeOrchestrator, Patterns: []string{`(?i)\bdocker\b`}, Confidence: 0.75},

	// Configuration Management
	{Name: "consul", Type: ComponentTypeConfigServer, Patterns: []string{`(?i)\bconsul\b`}, Confidence: 0.85},
	{Name: "etcd", Type: ComponentTypeConfigServer, Patterns: []string{`(?i)\betcd\b`}, Confidence: 0.85},
}

// ExtractedInfraComponent represents an infrastructure component extracted from text.
type ExtractedInfraComponent struct {
	Name       string
	Type       ComponentType
	Confidence float64
	Evidence   string // The text snippet where it was found
	Line       int    // Line number in the document
}

// ExtractInfrastructureComponents scans document content for mentions of
// infrastructure components and returns a list of detected components.
func ExtractInfrastructureComponents(content string, filePath string) []ExtractedInfraComponent {
	var extracted []ExtractedInfraComponent
	seen := make(map[string]bool) // Deduplicate by name

	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		// Skip empty lines and very short lines
		if len(strings.TrimSpace(line)) < 3 {
			continue
		}

		for _, pattern := range infrastructurePatterns {
			// Skip if we've already found this component
			if seen[pattern.Name] {
				continue
			}

			// Try each regex pattern
			for _, regexStr := range pattern.Patterns {
				re, err := regexp.Compile(regexStr)
				if err != nil {
					continue
				}

				if re.MatchString(line) {
					// For patterns requiring context, check for action verbs nearby
					if pattern.RequiresContext {
						lowerLine := strings.ToLower(line)
						hasContext := strings.Contains(lowerLine, "use") ||
							strings.Contains(lowerLine, "connect") ||
							strings.Contains(lowerLine, "store") ||
							strings.Contains(lowerLine, "access") ||
							strings.Contains(lowerLine, "authenticate") ||
							strings.Contains(lowerLine, "deploy") ||
							strings.Contains(lowerLine, "manage")

						if !hasContext {
							continue
						}
					}

					// Extract evidence snippet (max 100 chars)
					evidence := line
					if len(evidence) > 100 {
						evidence = evidence[:100] + "..."
					}

					extracted = append(extracted, ExtractedInfraComponent{
						Name:       pattern.Name,
						Type:       pattern.Type,
						Confidence: pattern.Confidence,
						Evidence:   strings.TrimSpace(evidence),
						Line:       lineNum + 1, // 1-indexed
					})

					seen[pattern.Name] = true
					break // Found this pattern, move to next
				}
			}
		}
	}

	return extracted
}

// InfrastructureMention represents a detected mention of an infrastructure
// component in documentation, used for creating graph nodes and relationships.
type InfrastructureMention struct {
	ComponentName string
	ComponentType ComponentType
	SourceFile    string
	Evidence      string
	Line          int
	Confidence    float64
}

// ExtractInfrastructureMentions is a convenience wrapper around
// ExtractInfrastructureComponents that returns InfrastructureMention objects.
func ExtractInfrastructureMentions(doc *Document) []InfrastructureMention {
	extracted := ExtractInfrastructureComponents(doc.Content, doc.Path)
	mentions := make([]InfrastructureMention, 0, len(extracted))

	for _, comp := range extracted {
		mentions = append(mentions, InfrastructureMention{
			ComponentName: comp.Name,
			ComponentType: comp.Type,
			SourceFile:    doc.Path,
			Evidence:      comp.Evidence,
			Line:          comp.Line,
			Confidence:    comp.Confidence,
		})
	}

	return mentions
}
