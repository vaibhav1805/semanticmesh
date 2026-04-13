# How semanticmesh Works

**Last updated:** 2026-04-13

This document explains semanticmesh's internal detection pipeline, component classification algorithms, and graph construction process.

## Overview

semanticmesh builds dependency graphs for microservices, databases, and infrastructure through a **three-stage detection pipeline**:

1. **Markdown Parsing** — Extract component names and relationships from documentation structure (services, APIs, databases)
2. **Code Analysis** — Detect dependencies from source code (AST parsing: DB clients, HTTP servers, message brokers)
3. **Infrastructure Text Extraction** — Mine component mentions from documentation content using pattern matching

Each stage contributes **signals** that are merged, deduplicated, and scored to produce a unified dependency graph covering service-to-service and service-to-infrastructure relationships.

---

## Architecture Diagram

```
┌─────────────────────────────┐
│  Input Sources              │
│  - Markdown (architecture)  │
│  - Go code                  │
│  - Python code              │
│  - JS code                  │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────┐
│           Three-Stage Detection Pipeline            │
├─────────────────────────────────────────────────────┤
│  Stage 1: Markdown Parsing                          │
│    → Extract headings, links, keywords              │
│    → Detect component names from structure          │
│    → Build initial relationship graph               │
├─────────────────────────────────────────────────────┤
│  Stage 2: Code Analysis (AST Parsing)               │
│    → Parse Go/Python/JS files                       │
│    → Detect connection strings, SDK calls           │
│    → Extract framework usage (Gin, Echo, K8s, etc.) │
├─────────────────────────────────────────────────────┤
│  Stage 3: Infrastructure Text Extraction            │
│    → Scan doc content with 50+ regex patterns       │
│    → Detect: databases, queues, cloud services      │
│    → Create infrastructure nodes from mentions      │
└────────┬────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────┐
│           Signal Merging & Classification           │
├─────────────────────────────────────────────────────┤
│  → Deduplicate signals across stages                │
│  → Apply type classification (content + patterns)   │
│  → Calculate confidence scores (0.4-1.0)            │
│  → Merge edges: doc + code → "both" source type     │
└────────┬────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────┐
│              Graph Construction                      │
├─────────────────────────────────────────────────────┤
│  → Create nodes (components + infrastructure)       │
│  → Create edges (relationships with provenance)     │
│  → Store in SQLite (graph.db)                       │
│  → Index for fast queries                           │
└─────────────────────────────────────────────────────┘
```

---

## Stage 1: Markdown Parsing

### What It Does

Analyzes markdown documentation to extract:
- **Component names** from headings (# Service Name, ## API Gateway, ## PostgreSQL)
- **Relationships** from links between documents (service → service, service → database)
- **Keywords** for type detection (service, database, API, queue, message-broker)
- **Document structure** (titles, sections, lists)

### Detection Methods

| Method | Description | Example |
|--------|-------------|---------|
| **Heading extraction** | Top-level headings → component names | `# Payment API` → component "payment-api" |
| **Link analysis** | `[text](path)` → relationship edge | `[uses Redis](redis.md)` → edge to redis |
| **Keyword matching** | Doc content → type signals | "PostgreSQL database" → type: database |
| **Path inference** | File paths → component names | `services/auth-service.md` → "auth-service" |

### Output

```json
{
  "component": "payment-api",
  "type": "service",
  "confidence": 0.85,
  "detection_method": "heading",
  "source_file": "docs/payment-api.md",
  "relationships": [
    {"target": "postgres-db", "relationship_type": "depends_on"}
  ]
}
```

---

## Stage 2: Code Analysis (AST Parsing)

### What It Does

Parses source code (Go, Python, JavaScript) to detect infrastructure dependencies:
- **Database connections** (PostgreSQL, MySQL, Redis, MongoDB, Cassandra)
- **HTTP servers/clients** (Gin, Echo, Fiber, Chi, http.Client)
- **Message brokers** (Kafka, RabbitMQ, NATS, Pulsar, NSQ)
- **Kubernetes integration** (controller-runtime, client-go)
- **Monitoring/observability** (Datadog, Prometheus, Jaeger, OpenTelemetry)
- **Cloud SDKs** (AWS, GCP, Azure)

### Go Parser Patterns (55+)

The Go parser in `internal/code/goparser/patterns.go` includes 55+ detection patterns:

#### REST API Servers (7 patterns)
```go
"github.com/gin-gonic/gin"           → http_server (Gin)
"github.com/labstack/echo"           → http_server (Echo)
"github.com/gofiber/fiber"           → http_server (Fiber)
"github.com/go-chi/chi"              → http_server (Chi)
"github.com/gorilla/mux"             → http_server (Gorilla Mux)
"net/http".ListenAndServe            → http_server (stdlib)
```

#### Kubernetes Integration (7 patterns)
```go
"sigs.k8s.io/controller-runtime/pkg/client"       → kubernetes_client
"sigs.k8s.io/controller-runtime/pkg/manager"      → kubernetes_controller
"k8s.io/client-go/kubernetes"                     → kubernetes_client
"k8s.io/apimachinery/pkg/runtime/schema"          → kubernetes_resource
```

#### Databases (15+ patterns)
```go
"github.com/jackc/pgx"               → database (pgx)
"gorm.io/driver/postgres"            → database (GORM PostgreSQL)
"gorm.io/driver/mysql"               → database (GORM MySQL)
"github.com/gocql/gocql"             → database (Cassandra)
"github.com/influxdata/influxdb-client-go" → database (InfluxDB)
```

#### Message Brokers (10+ patterns)
```go
"github.com/confluentinc/confluent-kafka-go" → message_broker (Kafka)
"github.com/apache/pulsar-client-go"         → message_broker (Pulsar)
"github.com/nats-io/nats.go"                 → message_broker (NATS)
"github.com/nsqio/go-nsq"                    → message_broker (NSQ)
"github.com/aws/aws-sdk-go/service/eventbridge" → message_broker (EventBridge)
```

#### Monitoring & Observability (10+ patterns)
```go
"github.com/DataDog/datadog-api-client-go"   → monitoring (Datadog)
"github.com/prometheus/client_golang"        → monitoring (Prometheus)
"github.com/jaegertracing/jaeger-client-go"  → monitoring (Jaeger)
"github.com/openzipkin/zipkin-go"            → monitoring (Zipkin)
"go.opentelemetry.io/otel"                   → monitoring (OpenTelemetry)
```

### Detection Process

1. **Parse imports** — Extract all import statements
2. **Match patterns** — Check against 55+ known infrastructure patterns
3. **Classify type** — Map pattern → component type (database, message-broker, etc.)
4. **Extract metadata** — Get package version, connection details if available
5. **Generate signals** — Create code_signals records with confidence scores

### Example Detection

**Input (Go code):**
```go
import (
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5"
    "github.com/DataDog/datadog-api-client-go/api/v2"
)
```

**Output (signals):**
```json
[
  {
    "signal_type": "http_server",
    "detected_value": "gin",
    "confidence": 0.90,
    "detection_method": "import_pattern_match",
    "file_path": "cmd/api/main.go",
    "component_type": "service"
  },
  {
    "signal_type": "database_client",
    "detected_value": "pgx",
    "confidence": 0.90,
    "detection_method": "import_pattern_match",
    "file_path": "cmd/api/main.go",
    "component_type": "database"
  },
  {
    "signal_type": "monitoring_client",
    "detected_value": "datadog",
    "confidence": 0.85,
    "detection_method": "import_pattern_match",
    "file_path": "cmd/api/main.go",
    "component_type": "monitoring"
  }
]
```

### Python & JavaScript Parsers

Similar pattern matching for:
- **Python:** `psycopg2`, `pymongo`, `redis`, `kafka-python`, `boto3`
- **JavaScript:** `pg`, `mongodb`, `redis`, `kafkajs`, `aws-sdk`

---

## Stage 3: Infrastructure Text Extraction

### What It Does (NEW in v0.3)

Scans documentation **content** (not just structure) to extract infrastructure component mentions using regex patterns. This catches components referenced in prose that may not have dedicated documentation.

**Example:**
> "We use **DynamoDB** for session storage, **LDAP** for authentication, and **AWS Secrets Manager** for credential management."

All three components (DynamoDB, LDAP, AWS Secrets Manager) are detected and added as infrastructure nodes.

### Pattern Library (50+ patterns)

Located in `internal/knowledge/infra_extractor.go`:

#### Databases (15 patterns)
```
PostgreSQL, MySQL, MongoDB, Redis, Cassandra, DynamoDB,
Elasticsearch, Neo4j, InfluxDB, TimescaleDB, CockroachDB,
MariaDB, Oracle, SQL Server, SQLite
```

#### Message Brokers (8 patterns)
```
Kafka, RabbitMQ, NATS, Pulsar, NSQ, ActiveMQ, ZeroMQ, EventBridge
```

#### Cloud Services (12 patterns)
```
AWS Lambda, S3, EC2, RDS, ECS, EKS, SQS, SNS, CloudFront,
DynamoDB, Secrets Manager, Parameter Store
```

#### Authentication (5 patterns)
```
LDAP, Active Directory, Okta, Auth0, Keycloak
```

#### Container Orchestration (4 patterns)
```
Kubernetes, Docker, OpenShift, Nomad
```

#### Monitoring (6 patterns)
```
Datadog, Prometheus, Grafana, Jaeger, Zipkin, New Relic
```

### Detection Algorithm

```python
for doc in markdown_documents:
    content = doc.title + " " + doc.body[:500]  # First 500 chars
    
    for pattern in infrastructure_patterns:
        if pattern.regex.match(content, case_insensitive=True):
            create_infrastructure_node(
                name=pattern.name,
                type=pattern.type,
                confidence=0.70,  # Text extraction baseline
                source_file=doc.path,
                detection_method="text_pattern_match"
            )
```

### Confidence Scoring

- **Exact match in title:** 0.85
- **Match in first paragraph:** 0.75
- **Match in body:** 0.70
- **Multiple mentions:** +0.05 per additional occurrence (max 0.95)

### Example Output

**Input (markdown content):**
```markdown
# Authentication Service

We use **LDAP** for user authentication and **AWS Secrets Manager**
to store API keys. Session data is cached in **DynamoDB**.
```

**Output (infrastructure nodes):**
```json
[
  {
    "id": "infrastructure/ldap",
    "name": "LDAP",
    "type": "authentication",
    "confidence": 0.85,
    "detection_method": "text_pattern_match",
    "source_file": "docs/auth-service.md"
  },
  {
    "id": "infrastructure/aws-secrets-manager",
    "name": "AWS Secrets Manager",
    "type": "secrets-manager",
    "confidence": 0.75,
    "detection_method": "text_pattern_match",
    "source_file": "docs/auth-service.md"
  },
  {
    "id": "infrastructure/dynamodb",
    "name": "DynamoDB",
    "type": "database",
    "confidence": 0.75,
    "detection_method": "text_pattern_match",
    "source_file": "docs/auth-service.md"
  }
]
```

---

## Component Type Classification

### Algorithm

Type classification combines **pattern matching** and **content analysis** with confidence scoring:

```python
def classify_component(name, content, signals):
    candidates = []
    
    # 1. Exact name match (highest confidence)
    if name in KNOWN_COMPONENTS:
        return (KNOWN_COMPONENTS[name], 1.0)
    
    # 2. Pattern matching on name
    for pattern, type in NAME_PATTERNS:
        if pattern.match(name):
            candidates.append((type, 0.90))
    
    # 3. Content-based inference
    title_keywords = extract_keywords(content.title)
    body_keywords = extract_keywords(content.body[:500])
    
    for keyword, type in TYPE_KEYWORDS:
        if keyword in title_keywords:
            candidates.append((type, 0.75))
        elif keyword in body_keywords:
            candidates.append((type, 0.65))
    
    # 4. Code signal corroboration
    for signal in signals:
        if signal.component_type:
            candidates.append((signal.component_type, 0.85))
    
    # 5. Multi-pattern confidence boosting
    type_scores = {}
    for type, confidence in candidates:
        type_scores[type] = type_scores.get(type, 0) + confidence
    
    best_type = max(type_scores, key=type_scores.get)
    final_confidence = min(type_scores[best_type], 1.0)
    
    return (best_type, final_confidence)
```

### Component Type Taxonomy

semanticmesh recognizes **15 component types**:

| Type | Description | Example |
|------|-------------|---------|
| `service` | Application service, API, backend | payment-api, user-service |
| `database` | Relational or NoSQL database | PostgreSQL, MongoDB, Redis |
| `cache` | In-memory data store | Redis, Memcached |
| `queue` | Task queue | Celery, Sidekiq, BullMQ |
| `message-broker` | Pub/sub messaging | Kafka, RabbitMQ, NATS |
| `load-balancer` | Traffic distribution | Nginx, HAProxy, ALB |
| `gateway` | API gateway | Kong, Ambassador, AWS API Gateway |
| `storage` | Object/blob storage | S3, GCS, Azure Blob |
| `container-registry` | Container image storage | Docker Hub, ECR, GCR |
| `config-server` | Configuration management | Consul, etcd, Spring Config |
| `monitoring` | Observability platform | Datadog, Prometheus, Grafana |
| `log-aggregator` | Log collection | Fluentd, Logstash, CloudWatch |
| `orchestrator` | Container orchestration | Kubernetes, ECS, Nomad |
| `secrets-manager` | Credential storage | AWS Secrets Manager, Vault |
| `search` | Search engine | Elasticsearch, Algolia |

### Confidence Levels

| Range | Interpretation | Typical Source |
|-------|----------------|----------------|
| **1.0** | Exact match | Known component list |
| **0.85-0.95** | Strong inference | Code signals + doc structure |
| **0.70-0.80** | Moderate inference | Content keywords + patterns |
| **0.50-0.65** | Weak inference | Ambiguous signals |
| **0.40-0.49** | Guess | Insufficient signals, fallback |

---

## Signal Merging

### What Is Signal Merging?

When multiple detection stages identify the **same component** or **same relationship**, their signals are merged to produce a single, higher-confidence record.

### Merging Rules

#### 1. Component Node Merging

```python
# Stage 1 finds: ("payment-api", type="service", confidence=0.75)
# Stage 2 finds: ("payment-api", type="service", confidence=0.90)
# Stage 3 finds: ("payment-api", type="service", confidence=0.70)

# Merged result:
{
  "name": "payment-api",
  "type": "service",
  "confidence": 0.90,  # Take highest
  "detection_methods": ["heading", "code_analysis", "text_extraction"],
  "source_files": ["docs/payment-api.md", "cmd/payment-api/main.go"]
}
```

#### 2. Edge Merging (Relationships)

```python
# Documentation edge: payment-api → postgres-db (source_type: "doc")
# Code edge: payment-api → postgres-db (source_type: "code")

# Merged edge (BOTH):
{
  "source": "payment-api",
  "target": "postgres-db",
  "source_type": "both",  # Corroborated by doc + code
  "confidence": 0.95,     # Boosted from 0.85
  "provenance": [
    {"method": "markdown_link", "file": "docs/payment-api.md"},
    {"method": "connection_string", "file": "internal/db/postgres.go"}
  ]
}
```

### Source Types

| Source Type | Meaning | Confidence Boost |
|-------------|---------|------------------|
| `markdown` | Only found in documentation | Baseline |
| `code` | Only found in source code | Baseline |
| `both` | Corroborated by doc + code | +10% |

---

### Multi-Source Confidence Merging

When the **same relationship** (same source component, target component, and edge type) is detected by both markdown analysis and code analysis, their confidence scores are merged using the **probabilistic OR formula**:

```
merged_confidence = 1.0 - (1.0 - markdown_confidence) * (1.0 - code_confidence)
```

This formula models the two sources as independent evidence. The merged confidence is always higher than either individual score, reflecting the increased certainty from corroboration.

#### When Merging Applies

- The same source-target-type triple must be detected by **both** markdown and code signals
- If only one source type detected the relationship, the confidence is left unchanged
- The merged confidence is clamped to a maximum of 1.0

#### Merging Examples

| Markdown Confidence | Code Confidence | Merged Confidence | Calculation |
|--------------------|-----------------|-------------------|-------------|
| 0.6 | 0.7 | 0.88 | `1 - (0.4 * 0.3) = 0.88` |
| 0.5 | 0.5 | 0.75 | `1 - (0.5 * 0.5) = 0.75` |
| 0.8 | 0.9 | 0.98 | `1 - (0.2 * 0.1) = 0.98` |
| 0.99 | 0.99 | 0.9999 | Near-certain from both sources |
| 0.4 | 0.0 | 0.4 | Only markdown detected it; no merge |

#### Effect on Confidence Tiers

Merging frequently promotes relationships to higher confidence tiers:

- A `moderate` (0.6) markdown edge corroborated by `moderate` (0.6) code evidence merges to `strong-inference` (0.84)
- A `weak` (0.5) + `weak` (0.5) merges to `strong-inference` (0.75)

---

### Integration Pipeline

During graph construction, signal merging follows this sequence:

1. **Convert** code signals to graph edges, grouped by (source, target, type). For each group, the highest-confidence signal sets the edge confidence.

2. **Create stub nodes** for code-detected targets not already in the graph (prevents dangling edge references).

3. **Merge** code-discovered edges with markdown-discovered edges. Edges with matching source-target-type keys have their signal lists combined.

4. **Apply probabilistic OR** to merged edges that have signals from both markdown and code sources.

5. **Set source_type** on each edge based on which signal sources are present:
   - Only markdown signals: `"markdown"`
   - Only code signals: `"code"`
   - Both: `"both"`

6. **Add edges to graph** — code-originated and dual-source edges are added to the graph structure.

---

### Querying by Source Type

Query commands accept `--source-type` to filter results by detection source:

| Filter Value | Matches edges with source_type |
|-------------|-------------------------------|
| `markdown` | `"markdown"` or `"both"` |
| `code` | `"code"` or `"both"` |
| `both` | `"both"` only |
| *(omitted)* | All edges |

**Key insight:** `--source-type markdown` includes `"both"` edges because markdown analysis was involved. Similarly, `--source-type code` includes `"both"` edges because code analysis was involved. Use `--source-type both` to find only relationships corroborated by both methods.

**Examples:**

```bash
# All relationships where code analysis contributed
semanticmesh query impact --component payment-api --source-type code

# Only relationships confirmed by both sources
semanticmesh query dependencies --component auth-service --source-type both

# Markdown-detected relationships (including those also found in code)
semanticmesh query list --source-type markdown
```

---

### Signal Provenance

Raw code analysis signals are persisted in the `code_signals` table for full provenance:

```sql
CREATE TABLE code_signals (
    id INTEGER PRIMARY KEY,
    file_path TEXT NOT NULL,
    signal_type TEXT NOT NULL,     -- "http_server", "database_client"
    detected_value TEXT,           -- "gin", "pgx"
    confidence REAL,
    context TEXT,
    line_number INTEGER,
    language TEXT
);
```

This table preserves fine-grained signal details that are aggregated into the coarser graph edge representation. It enables "where exactly in the code was this detected?" queries.

---

### Edge Drop Warnings

During graph export, edges referencing components not present as graph nodes are dropped. When this occurs, a warning is printed:

```
Warning: 3 edge(s) dropped (missing endpoint nodes)
  service-a -> unknown-target
  missing-source -> database-b
```

This can happen when:
- Code analysis detects a dependency on a component not found in documentation and stub node creation fails
- An edge references a component that was removed between detection and export

The warning is informational — the graph is saved successfully with the remaining valid edges.

---

## Graph Construction

### Schema Overview

semanticmesh stores the dependency graph in **SQLite** (`knowledge.db`):

```sql
-- Component nodes
CREATE TABLE graph_nodes (
    id TEXT PRIMARY KEY,           -- "service/payment-api"
    name TEXT NOT NULL,            -- "payment-api"
    type TEXT NOT NULL,            -- "component" or "infrastructure"
    component_type TEXT NOT NULL,  -- "service", "database", etc.
    confidence REAL NOT NULL,      -- 0.4-1.0
    metadata TEXT                  -- JSON blob
);

-- Relationships (edges)
CREATE TABLE graph_edges (
    source_id TEXT NOT NULL,       -- "service/payment-api"
    target_id TEXT NOT NULL,       -- "database/postgres-db"
    relationship_type TEXT,        -- "depends_on", "calls", etc.
    source_type TEXT,              -- "doc", "code", "both"
    confidence REAL,               -- 0.4-1.0
    metadata TEXT,                 -- JSON blob
    FOREIGN KEY (source_id) REFERENCES graph_nodes(id),
    FOREIGN KEY (target_id) REFERENCES graph_nodes(id)
);

-- Detection provenance
CREATE TABLE component_mentions (
    component_id TEXT NOT NULL,
    source_file TEXT NOT NULL,
    detection_method TEXT NOT NULL,
    confidence REAL NOT NULL,
    context TEXT,                  -- Surrounding text
    FOREIGN KEY (component_id) REFERENCES graph_nodes(id)
);

-- Code signals (from Stage 2)
CREATE TABLE code_signals (
    id INTEGER PRIMARY KEY,
    file_path TEXT NOT NULL,
    signal_type TEXT NOT NULL,     -- "http_server", "database_client"
    detected_value TEXT,           -- "gin", "pgx"
    confidence REAL,
    context TEXT
);
```

### Graph Construction Process

```python
def build_graph(markdown_signals, code_signals, infra_signals):
    # 1. Create nodes from all signals
    nodes = {}
    for signal in markdown_signals + code_signals + infra_signals:
        node_id = generate_id(signal.name, signal.type)
        if node_id not in nodes:
            nodes[node_id] = Node(
                id=node_id,
                name=signal.name,
                type=signal.type,
                confidence=signal.confidence
            )
        else:
            # Merge: take highest confidence
            nodes[node_id].confidence = max(
                nodes[node_id].confidence,
                signal.confidence
            )
            nodes[node_id].detection_methods.append(signal.method)
    
    # 2. Create edges from relationships
    edges = {}
    for signal in markdown_signals + code_signals:
        for relationship in signal.relationships:
            edge_key = (signal.name, relationship.target)
            if edge_key not in edges:
                edges[edge_key] = Edge(
                    source=signal.name,
                    target=relationship.target,
                    source_type=signal.source_type,
                    confidence=relationship.confidence
                )
            else:
                # Merge: if both doc and code, upgrade to "both"
                existing = edges[edge_key]
                if existing.source_type != signal.source_type:
                    existing.source_type = "both"
                    existing.confidence += 0.10  # Boost
    
    # 3. Persist to SQLite
    db.execute("INSERT INTO graph_nodes ...", nodes.values())
    db.execute("INSERT INTO graph_edges ...", edges.values())
    
    return graph
```

---

## Performance & Scalability

### Benchmarks (8 production repositories)

| Metric | Value |
|--------|-------|
| **Documents indexed** | 98 markdown files |
| **Code files analyzed** | 145 Go files |
| **Graph nodes created** | 120 components + 26 infrastructure |
| **Relationships discovered** | 1,183 edges (186 cross-repo) |
| **Total processing time** | ~4.5 seconds |
| **Database size** | 1.2 MB (SQLite) |

### Optimization Techniques

1. **Incremental parsing** — Only re-parse changed files (planned)
2. **Parallel code analysis** — Parse files concurrently (Go routines)
3. **SQLite indexes** — B-tree indexes on `graph_nodes.id` and `graph_edges.source_id`
4. **Pattern caching** — Compile regex patterns once at startup

---

## Limitations & Edge Cases

### Current Limitations

1. **Kotlin/Java parsers not implemented** — Repositories using JVM languages rely on doc extraction only
2. **Dynamic imports** — Runtime-loaded dependencies not detected (e.g., plugin systems)
3. **Implicit relationships** — Some dependencies inferred from config files, not code
4. **Cross-language calls** — Python→Go or JS→Go edges not detected from code (doc hints only)

### Known Edge Cases

1. **Comment hints overdetection** — URLs in comments (e.g., "www.apache.org") generate noise
   - **Workaround:** Filter URLs with `WHERE signal_type != 'comment_hint'`
2. **Broken doc links** — Links to non-existent files create dangling edges
   - **Handling:** Edges stored but flagged as `broken: true` in metadata
3. **Ambiguous component names** — Generic names (e.g., "api", "database") may collide
   - **Resolution:** Use fully qualified IDs (e.g., "service/api", "database/postgres")

---

## Future Improvements

### Planned Enhancements

1. **Kotlin/Java parsers** — Add JVM language support
2. **Configuration file parsing** — Detect dependencies from `application.yml`, `docker-compose.yml`
3. **Incremental updates** — Only reprocess changed files
4. **ML-based type classification** — Use embeddings for better type inference
5. **Dependency version tracking** — Store package versions for vulnerability scanning
6. **Visual graph explorer** — Web UI for interactive graph exploration

---

## Summary

semanticmesh's three-stage detection pipeline enables **70-80% detection accuracy** by combining:

1. **Markdown parsing** — Structural component discovery (services, databases, APIs)
2. **Code analysis** — Dependency detection from source code (55+ Go patterns for DB clients, HTTP servers, message brokers)
3. **Text extraction** — Prose-based component mining (50+ infrastructure patterns)

Signals are merged, classified, and stored in a queryable SQLite graph, enabling AI agents to answer "what breaks if X fails?" by querying the graph instead of ingesting entire architectures.

For detailed pattern lists, see:
- [CODE_ANALYSIS.md](./CODE_ANALYSIS.md) — Full code parser pattern reference
- [COMPONENT_TYPES.md](./COMPONENT_TYPES.md) — Type classification taxonomy
- [INFRASTRUCTURE_EXTRACTION.md](./INFRASTRUCTURE_EXTRACTION.md) — Text extraction patterns

---

**See also:**
- [ARCHITECTURE.md](./ARCHITECTURE.md) — High-level system design
- [CLI_REFERENCE.md](./CLI_REFERENCE.md) — Command usage
- [QUERY_REFERENCE.md](./QUERY_REFERENCE.md) — Query examples
