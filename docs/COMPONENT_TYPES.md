# Component Types Reference

Every component in your infrastructure carries a type classification that enables targeted queries and dependency analysis. This guide explains the 15 core component types, detection patterns, and how to work with the classification system.

## Overview

Components are the building blocks of your infrastructure documentation. Each component must be assigned a **primary type** (required) and may carry optional **tags** for additional metadata.

- **Primary type:** The primary classification (service, database, cache, etc.). Used for filtering with `semanticmesh list --type`.
- **Tags:** Optional secondary metadata (criticality, deployment model, compliance status, etc.). Searchable via `--include-tags` flag.
- **Confidence score:** A measure of detection reliability (0.4–1.0). Higher confidence indicates stronger inference evidence.

All 12 core types are built into semanticmesh. Users can also define custom types via seed configuration (see docs/CONFIGURATION.md).

## Core Component Types

### 1. Service

**Description:** An application or microservice component providing business logic, APIs, or request handling.

**Examples:**
- API servers (FastAPI, Spring Boot, Node.js/Express)
- Microservices (order-service, user-service, auth-service)
- Web applications (Django, Rails, React backends)

**Detection patterns:**
- File paths: `services/`, `apps/`, `microservices/`
- Naming conventions: `*-service`, `api-*`, `*-app`
- Keywords: "service", "api", "application", "server"

**Typical confidence:** 0.85–1.0 (high; naming is explicit)

---

### 2. Database

**Description:** A data storage system (relational, document-oriented, or other persistence layer).

**Examples:**
- PostgreSQL, MySQL, Oracle (relational)
- MongoDB, CouchDB (document stores)
- Cassandra, DynamoDB (NoSQL)

**Detection patterns:**
- File paths: `databases/`, `data/`, `persistence/`
- Naming conventions: `*-db`, `postgres*`, `mysql*`, `mongodb-*`
- Keywords: "database", "db", "postgres", "mysql", "postgresql", "cassandra"

**Typical confidence:** 0.90–1.0 (very high; database names are distinctive)

---

### 3. Cache

**Description:** An in-memory data layer for performance optimization.

**Examples:**
- Redis, Memcached
- Ehcache, Hazelcast
- Application-level caches

**Detection patterns:**
- File paths: `cache/`, `caching/`
- Naming conventions: `redis*`, `memcache*`, `*-cache`
- Keywords: "cache", "redis", "memcached"

**Typical confidence:** 0.88–1.0 (high; cache naming is clear)

---

### 4. Queue

**Description:** A message queue system for asynchronous task processing.

**Examples:**
- RabbitMQ, AWS SQS, Apache Kafka
- ActiveMQ, Azure Service Bus
- Job queues (Celery, Sidekiq backends)

**Detection patterns:**
- File paths: `queues/`, `messaging/`, `async/`
- Naming conventions: `*-queue`, `rabbitmq*`, `sqs-*`
- Keywords: "queue", "job queue", "sqs", "celery", "sidekiq"

**Typical confidence:** 0.80–0.95 (good; some overlap with message-broker)

---

### 5. Message-Broker

**Description:** Infrastructure for message routing, event streaming, and pub/sub patterns.

**Examples:**
- Kafka, NATS, Apache Pulsar
- RabbitMQ (when used for event streaming)
- AWS EventBridge

**Detection patterns:**
- File paths: `broker/`, `messaging/`, `events/`
- Naming conventions: `kafka*`, `nats-*`, `*-broker`
- Keywords: "broker", "kafka", "nats", "pulsar", "event stream"

**Typical confidence:** 0.75–0.90 (moderate; overlap with queue and stream processing)

---

### 6. Load-Balancer

**Description:** Infrastructure for distributing traffic across multiple instances.

**Examples:**
- HAProxy, nginx (as load balancer)
- AWS Elastic Load Balancer (ELB, ALB, NLB)
- F5, Citrix NetScaler

**Detection patterns:**
- File paths: `load-balancer/`, `balancing/`, `lb/`
- Naming conventions: `lb-*`, `haproxy*`, `*-balancer`
- Keywords: "load balancer", "elb", "alb", "nlb", "haproxy"

**Typical confidence:** 0.82–1.0 (high; infrastructure naming is explicit)

---

### 7. Gateway

**Description:** API gateway or service gateway component for request routing, authentication, and API management.

**Examples:**
- Kong, Traefik, Tyk
- AWS API Gateway
- Azure API Management
- Spring Cloud Gateway

**Detection patterns:**
- File paths: `gateway/`, `gateways/`, `api-gateway/`
- Naming conventions: `*-gateway`, `kong*`, `traefik*`
- Keywords: "gateway", "api gateway", "kong", "traefik"

**Typical confidence:** 0.85–1.0 (high; gateway naming is explicit)

---

### 8. Storage

**Description:** Object or blob storage for unstructured data.

**Examples:**
- AWS S3, Google Cloud Storage, Azure Blob Storage
- MinIO, DigitalOcean Spaces
- OpenStack Swift

**Detection patterns:**
- File paths: `storage/`, `s3/`, `blob/`
- Naming conventions: `s3-*`, `bucket-*`, `*-storage`
- Keywords: "s3", "gcs", "blob storage", "object storage"

**Typical confidence:** 0.90–1.0 (very high; storage naming is distinctive)

---

### 9. Container-Registry

**Description:** Image repository for Docker or container images.

**Examples:**
- Docker Hub, Docker Registry
- AWS ECR (Elastic Container Registry)
- Google Container Registry (GCR)
- GitLab Container Registry, GitHub Container Registry

**Detection patterns:**
- File paths: `registry/`, `container-registry/`, `ecr/`
- Naming conventions: `ecr-*`, `registry-*`, `*-registry`
- Keywords: "container registry", "ecr", "docker registry", "gcr"

**Typical confidence:** 0.92–1.0 (very high; registry naming is clear)

---

### 10. Config-Server

**Description:** Centralized configuration management system.

**Examples:**
- Consul, etcd, Spring Cloud Config Server
- Vault (secrets management)
- HashiCorp Consul
- AWS Systems Manager Parameter Store

**Detection patterns:**
- File paths: `config/`, `configuration/`, `consul/`, `etcd/`
- Naming conventions: `consul-*`, `etcd-*`, `*-config`
- Keywords: "config server", "consul", "etcd", "vault", "configuration"

**Typical confidence:** 0.85–0.95 (good; some overlap with secrets management)

---

### 11. Monitoring

**Description:** Metrics collection, visualization, and alerting system for observability.

**Examples:**
- Prometheus, Grafana, Datadog
- New Relic, Splunk (metrics module)
- CloudWatch, Azure Monitor
- InfluxDB (time-series metrics)

**Detection patterns:**
- File paths: `monitoring/`, `metrics/`, `prometheus/`
- Naming conventions: `prometheus-*`, `datadog-*`, `*-monitoring`
- Keywords: "monitoring", "prometheus", "grafana", "datadog", "metrics"

**Typical confidence:** 0.88–1.0 (high; monitoring naming is explicit)

---

### 12. Log-Aggregator

**Description:** Centralized logging and log analysis system.

**Examples:**
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Splunk, Datadog Logs
- Loki, Fluentd
- AWS CloudWatch Logs, Azure Log Analytics

**Detection patterns:**
- File paths: `logging/`, `logs/`, `elk/`, `splunk/`
- Naming conventions: `elasticsearch-*`, `kibana-*`, `splunk-*`
- Keywords: "log", "logging", "elasticsearch", "kibana", "elk", "splunk"

**Typical confidence:** 0.85–0.98 (high; logging naming is usually clear)

---

## Unknown Type

When a component cannot be confidently classified into one of the 12 core types, it is assigned the **`unknown`** type.

**When does unknown occur?**
- Component name is ambiguous or generic ("app1", "service-123")
- No detection algorithm can classify confidently
- Component name has no distinctive keywords or patterns

**Is unknown a failure?**
No. `unknown` indicates that the detection system has insufficient signal to classify confidently. This is not an error — it's a conservative classification that prevents false positives.

**How to override unknown classifications:**
If a component is persistently misclassified as `unknown`, use seed configuration to provide an explicit mapping:

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "my-internal-app"
    type: "service"
    confidence_override: 1.0
```

See docs/CONFIGURATION.md for detailed seed config guidance.

---

## Tags (Optional Secondary Classification)

Primary types capture the main category, but components may have additional metadata relevant to operations:

**Common tag patterns:**
- **Criticality:** `critical`, `non-critical`, `degraded-ok`
- **Deployment:** `internal-only`, `public-facing`, `on-premises`, `cloud-native`
- **Compliance:** `compliance-critical`, `hipaa`, `pci-dss`, `gdpr`
- **Lifecycle:** `deprecated`, `beta`, `production`, `experimental`
- **Provider:** `aws`, `gcp`, `azure`, `self-hosted`

**Example:**
```json
{
  "name": "postgres-primary",
  "type": "database",
  "tags": ["critical", "public-facing", "pci-dss"],
  "confidence": 0.95
}
```

**Using tags in queries:**
By default, `semanticmesh list --type TYPE` returns only components with that primary type. To also include components tagged with the type:

```bash
semanticmesh list --type database --include-tags
```

This would return:
- All components with `type: "database"`
- All components with `database` in their tags (but different primary type)

---

## Workflow: Querying by Type

### List all services

```bash
semanticmesh list --type service --output json
```

**Response:** All components with `type: "service"`, ordered by confidence.

### Find critical databases

```bash
semanticmesh list --type database --include-tags --output json | jq '.components[] | select(.tags[]? | contains("critical"))'
```

**Response:** Databases or components tagged as critical.

### Count components by type

```bash
semanticmesh list --output json | jq '.components | group_by(.type) | map({type: .[0].type, count: length})'
```

**Response:** Distribution of components across all types.

---

## Detection & Confidence

Graphmd uses multiple detection algorithms to infer component type from file paths, naming conventions, and keyword patterns. Each detection method contributes evidence, resulting in a confidence score (0.4–1.0).

- **0.95–1.0:** Very high confidence (explicit naming, strong keyword match)
- **0.80–0.94:** High confidence (clear but not explicit)
- **0.65–0.79:** Moderate confidence (some ambiguity, multiple interpretations possible)
- **0.40–0.64:** Low confidence (weak signal, may be misclassified)

All confidence scores are included in query results, allowing AI agents to make risk-aware decisions about which components to trust.

---

## Extending Component Types

semanticmesh ships with 15 core types, but your infrastructure may have unique component categories. To add custom types without modifying code, use seed configuration:

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "internal-tools/*"
    type: "internal-tool"  # custom type
    confidence_override: 1.0
```

See docs/CONFIGURATION.md for complete guidance on extending the taxonomy.

---

## Design Decisions & Architecture

This section documents the architectural decisions behind semanticmesh's component type classification system.

**Status:** Accepted  
**Original Decision Date:** 2026-03-19  
**Last Updated:** 2026-04-13

### Problem

Infrastructure dependency graphs are only useful if AI agents can ask targeted questions like:

- **"If our database fails, what services depend on it?"**
- **"Which critical components need immediate monitoring?"**
- **"How many message brokers do we have, and where are they used?"**

Without component type information, answering these questions requires either:
1. Feeding agents the entire architecture (expensive, slow)
2. Having agents infer types from component names (unreliable, fragile)

This defeats the core value of semanticmesh: allowing AI agents to efficiently query pre-computed dependency graphs for rapid incident response and architectural analysis.

### Solution

Every component in the graph carries a **type classification**. This enables:
- **Targeted queries:** `"SELECT * FROM graph_nodes WHERE component_type = 'database'"`
- **Confidence-aware decisions:** Agents can filter by confidence threshold for risk assessment
- **Extensibility:** Users can define custom types via seed configuration without modifying code

### Why 15 Types (Not More, Not Fewer)?

**Options considered:**
- **Flat/single type** (everything is "component") — Loses query power
- **Huge taxonomy** (50+ types) — Overwhelming, difficult to auto-detect reliably
- **15 core types** — Sweet spot

**Rationale:**
- **Coverage:** 15 types cover ~95% of typical infrastructure components
- **Detectability:** Each type has distinctive naming patterns, making auto-detection reliable
- **Extensibility:** Seed config allows users to add custom types for edge cases without bloating the core
- **Evolution:** Started with 12 types, added 3 (orchestrator, secrets-manager, search) based on real-world usage

### Why Confidence Scores (Not Binary Classification)?

**Options considered:**
- **Binary** (classified or unknown) — Simple, but loses important signal
- **Confidence scores** (0.4–1.0) — Complex, but enables risk-aware queries
- **Multiple candidate types** — Too complex; requires resolution logic

**Rationale:**
- **Reality:** Type detection is probabilistic, not deterministic. A component named "cache-helper" might be a cache (high confidence) or a helper service (medium confidence).
- **Agent needs:** AI agents need to assess detection reliability. A 0.95 confidence match is safer to act on than 0.65.
- **Query filtering:** Agents can adjust confidence thresholds for different risk tolerance: `WHERE confidence >= 0.8` for high-confidence queries.
- **Implementation:** Confidence is cheap to compute (already generated by detection algorithms) and easy to persist.

### Why Seed Config for Extensibility?

**Options considered:**
- **Fixed taxonomy only** — Users adapt to our types (inflexible)
- **Code modification** — Users fork and modify types.go (unmaintainable)
- **Seed config file** — Users provide YAML mappings at index time (flexible, maintained)

**Rationale:**
- **Customization:** Organizations have custom component categories (e.g., "internal-tool", "legacy-system"). Seed config lets them define these without code changes.
- **Overrides:** Some components may be consistently misclassified by auto-detection. Seed config provides escape hatch.
- **Confidence:** Seed-defined mappings get confidence = 1.0 (explicit user intent), superseding auto-detection.
- **Simplicity:** YAML is user-friendly; no code review needed.

### Why Tags for Metadata?

**Options considered:**
- **Ignore metadata** (type only) — Loses important context
- **Type hierarchy** (service > internal-service > critical-internal-service) — Complex, rigid
- **Tags** (primary type + optional tag array) — Flexible, composable

**Rationale:**
- **Flexibility:** Tags capture metadata that doesn't fit neatly into a hierarchy:
  - Criticality: `critical`, `non-critical`
  - Deployment: `internal-only`, `public-facing`
  - Compliance: `pci-dss`, `hipaa`
  - Lifecycle: `deprecated`, `beta`
- **Composability:** One component can have multiple tags; no need for a deep type tree.
- **Query power:** Tags enable search by metadata without creating new types.
- **Extensibility:** Users define their own tags (no code changes needed).

### Trade-offs

#### Flexibility vs. Simplicity

**Trade-off:** More types = more flexibility but also more complexity, detection ambiguity, and user confusion.

**Resolution:** 15 core types + seed config provides the best of both:
- Core types handle 95% of use cases (simplicity)
- Seed config handles edge cases (flexibility)
- Users rarely need more than 2–3 custom types

#### Auto-Detection vs. Manual Classification

**Trade-off:** Auto-detection is convenient but imperfect; manual classification is accurate but burdensome.

**Resolution:** Hybrid approach:
- **Default:** Auto-detection from naming patterns (0.65–1.0 confidence)
- **Override:** Seed config for persistent misclassifications (confidence 1.0)
- **Fallback:** `unknown` type for ambiguous cases (conservative)

#### Single vs. Multiple Types

**Trade-off:** Allowing one component to have multiple primary types increases expressiveness but complicates queries.

**Resolution:** Primary type (singular) + tags (plural):
- Primary type is the main classification (used for filtering)
- Tags capture secondary metadata (optional, searchable separately)
- Queries filter by primary type; tags are supplementary

### Consequences

**Positive:**
- ✅ Agents can now answer "what depends on databases?" efficiently
- ✅ Confidence scores let agents make risk-aware decisions
- ✅ Seed config lets users customize without code changes
- ✅ 15 types are learnable; users understand their own infrastructure

**Negative:**
- ⚠️ Detection pipeline is more complex than simple heuristics
- ⚠️ Type taxonomy may need updates as infrastructure patterns evolve
- ⚠️ Some components will remain `unknown` or misclassified (trade-off accepted)

**Neutral:**
- ℹ️ Requires `component_type` column on `graph_nodes` (minimal schema impact)
- ℹ️ Queries return confidence scores (agents need to handle numeric values)

### References

- **Backstage:** Spotify's service catalog (https://backstage.io/) — inspired component taxonomy
- **Cartography:** Lyft's cloud asset inventory — inspired detection patterns
- **semanticmesh CLI_REFERENCE.md:** Command syntax and query examples
- **semanticmesh CONFIGURATION.md:** Seed config guide for customization

---

**Last updated:** 2026-04-13
