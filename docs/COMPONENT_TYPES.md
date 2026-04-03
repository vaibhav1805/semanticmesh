# Component Types Reference

Every component in your infrastructure carries a type classification that enables targeted queries and dependency analysis. This guide explains the 12 core component types, detection patterns, and how to work with the classification system.

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

Graphmd ships with 12 core types, but your infrastructure may have unique component categories. To add custom types without modifying code, use seed configuration:

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "internal-tools/*"
    type: "internal-tool"  # custom type
    confidence_override: 1.0
```

See docs/CONFIGURATION.md for complete guidance on extending the taxonomy.

---

**Last updated:** 2026-03-19
