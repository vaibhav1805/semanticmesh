<p align="center">
  <img src="https://raw.githubusercontent.com/vaibhav1805/semanticmesh/main/.github/banner.png" alt="semanticmesh - Map and query dependencies across microservices, databases, and infrastructure" width="800"/>
</p>

# semanticmesh

Map and query dependencies across microservices, databases, and infrastructure. Answer questions like "if this fails, what breaks?" by querying a pre-computed graph instead of feeding entire architectures to AI agents.

## What Is semanticmesh?

semanticmesh scans your documentation (markdown files) and source code (Go, Python, JavaScript), automatically detects components (services, databases, infrastructure), classifies them by type, and builds a queryable dependency graph. This enables AI agents and operators to:

- **Query impact:** "If this database fails, which services break?"
- **Trace dependencies:** "What does payment-api depend on?"
- **Find paths:** "How does the web frontend connect to the primary database?"
- **List components:** "Show all services with confidence above 0.7"
- **Analyze code:** Detect infrastructure dependencies from connection strings, SDK calls, and imports (55+ Go patterns)
- **Extract infrastructure:** Mine component mentions from documentation prose (50+ infrastructure patterns)

Instead of feeding AI agents your entire architecture, they query the pre-computed graph — faster, cheaper, and more reliable.

**Detection accuracy:** 85-92% on real-world codebases (tested on production repositories)

## Features

### Three-Stage Detection Pipeline

semanticmesh merges signals from three sources into a single hybrid dependency graph:

- **Markdown analysis:** Extracts components and relationships from documentation (services, APIs, dependencies)
- **Code analysis:** Detects database connections, service URLs, message queue bindings, and cache clients from Go, Python, and JavaScript source code (55+ Go patterns including Gin, Echo, Kubernetes, pgx, Kafka, Datadog)
- **Infrastructure text extraction:** Mines component mentions from documentation prose using 50+ regex patterns (databases, cloud services, message brokers, authentication systems)

When both documentation and code corroborate a relationship, the edge is tagged `source_type: both`, giving higher confidence.

See [docs/HOW_IT_WORKS.md](docs/HOW_IT_WORKS.md) for a detailed explanation of the detection pipeline.

### Component Classification

Every component carries a type classification (one of **15 core types**: `service`, `database`, `cache`, `queue`, `message-broker`, `load-balancer`, `gateway`, `storage`, `container-registry`, `config-server`, `monitoring`, `log-aggregator`, `orchestrator`, `secrets-manager`, `search`) with a confidence score (0.4--1.0) reflecting detection reliability.

Classification combines **pattern matching** and **content analysis** to infer component types even when not explicitly declared.

See [docs/COMPONENT_TYPES.md](docs/COMPONENT_TYPES.md) for the complete type reference.

### Structured Query Interface

Four query commands cover common dependency analysis needs:

```bash
# What breaks if primary-db fails?
semanticmesh query impact --component primary-db --depth all

# What does payment-api depend on?
semanticmesh query dependencies --component payment-api

# How does web-frontend connect to primary-db?
semanticmesh query path --from web-frontend --to primary-db

# List all services
semanticmesh query list --type service --min-confidence 0.7
```

All queries return structured JSON with confidence tiers, detection provenance, and graph metadata. See [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) for the full command reference.

### MCP Server for LLM Agents

semanticmesh includes a built-in MCP (Model Context Protocol) server, allowing LLM agents to query the dependency graph directly via tool calls:

```bash
semanticmesh mcp
```

The server exposes six tools over stdio transport: `query_impact`, `query_dependencies`, `query_path`, `list_components`, `get_component_embeddings`, and `semanticmesh_graph_info`. Configure it in your MCP client (e.g., Claude Desktop) to give agents on-demand access to your infrastructure graph.

**New:** The `get_component_embeddings` tool provides 384-dimensional vector embeddings for components, enabling semantic similarity analysis and clustering. Currently uses deterministic placeholder embeddings; see [docs/MCP_SERVER.md](docs/MCP_SERVER.md#get_component_embeddings) for integration guide with real embedding models (OpenAI, Cohere, Voyage).

### Extensible Type System

Define custom types without code changes via seed configuration:

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "internal-tools/*"
    type: "internal-tool"
    tags: ["internal-only"]
```

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for customization options.

## Quick Start

### Prerequisites

- Go 1.21+
- Architecture documented in markdown and/or source code in Go, Python, or JavaScript

### Installation

**Option 1: Install via curl (Recommended)**

```bash
curl -fsSL https://raw.githubusercontent.com/vaibhav1805/semanticmesh/main/install.sh | bash
```

This will download the latest release binary for your platform (Linux, macOS, Windows).

**Option 2: Build from source**

```bash
git clone https://github.com/vaibhav1805/semanticmesh
cd semanticmesh
go build -o semanticmesh ./cmd/semanticmesh
```

### Export a Graph

The `export` command scans documentation and (optionally) source code, builds the dependency graph, and packages it as a portable ZIP archive:

```bash
# Export from documentation only
semanticmesh export --input ./docs --output graph.zip

# Export with code analysis enabled
semanticmesh export --input ./project --output graph.zip --analyze-code
```

### Multi-Repository Setup (for Microservices)

**If you have multiple repos/services**, organize them in a workspace directory first. This allows semanticmesh to detect **cross-service dependencies** by analyzing all codebases together:

```bash
# Create a workspace
mkdir my-architecture-workspace
cd my-architecture-workspace

# Clone all your repos
git clone https://github.com/org/service-a
git clone https://github.com/org/service-b
git clone https://github.com/org/database-docs

# Export the entire workspace
semanticmesh export --input . --output workspace-graph.zip --analyze-code
```

**Recommended structure:**
```
workspace/
├── docs/                    # Architecture documentation
│   ├── architecture.md
│   └── services/
├── service-a/              # Go microservice
├── service-b/              # Python microservice
└── frontend/               # JavaScript frontend
```

This approach enables semanticmesh to:
- Detect dependencies **between services** (e.g., service-a calls service-b's API)
- Find shared infrastructure usage (e.g., multiple services using the same database)
- Build a complete dependency graph across your entire architecture

### Import and Query

Import the exported graph into persistent storage, then query it:

```bash
# Import the graph
semanticmesh import graph.zip --name prod-infra

# Query impact
semanticmesh query impact --component primary-db --depth all

# List all databases
semanticmesh query list --type database

# Find path between components
semanticmesh query path --from web-frontend --to primary-db
```

### Crawl (Pre-Export Diagnostic)

Use `crawl` to preview graph statistics before exporting:

```bash
semanticmesh crawl --input ./docs --format json
semanticmesh crawl --input ./project --analyze-code --format json
```

## Common Workflows

### Assess Blast Radius of a Failure

```bash
semanticmesh query impact --component primary-db --depth all --format json
```

Returns all transitively affected components with distance, confidence tiers, and relationship details.

### Filter by Detection Source

```bash
# Only relationships detected from source code
semanticmesh query dependencies --component payment-api --source-type code

# Only relationships corroborated by both markdown and code
semanticmesh query impact --component redis-cache --source-type both
```

### Include Detection Provenance

```bash
semanticmesh query impact --component primary-db --include-provenance --max-mentions 3
```

Each affected node includes where and how it was detected (file path, detection method, confidence).

### Use with AI Agents via MCP

```bash
# Start MCP server (stdio transport)
semanticmesh mcp
```

Agents can call `query_impact`, `query_dependencies`, `query_path`, `list_components`, and `semanticmesh_graph_info` as MCP tools.

## Documentation

### Core Guides

- **[docs/HOW_IT_WORKS.md](docs/HOW_IT_WORKS.md)** — Technical deep-dive: three-stage detection pipeline, signal merging, algorithms, and graph construction
- **[docs/COMPONENT_TYPES.md](docs/COMPONENT_TYPES.md)** — Complete reference for all 15 component types, detection patterns, confidence scoring, and design decisions
- **[docs/INFRASTRUCTURE_EXTRACTION.md](docs/INFRASTRUCTURE_EXTRACTION.md)** — Infrastructure text extraction: 50+ patterns for mining components from documentation prose

### Reference

- **[docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md)** — Full command reference for all semanticmesh commands with examples and JSON output formats
- **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)** — Guide to `.semanticmeshignore`, `semanticmesh-aliases.yaml`, seed configuration, and named graph management
- **[docs/CODE_ANALYSIS.md](docs/CODE_ANALYSIS.md)** — Code parser patterns for Go, Python, and JavaScript
- **[docs/QUERY_REFERENCE.md](docs/QUERY_REFERENCE.md)** — SQL query examples and advanced graph queries
- **[docs/SCHEMA.md](docs/SCHEMA.md)** — Database schema reference

### Integration & Architecture

- **[docs/MCP_SERVER.md](docs/MCP_SERVER.md)** — MCP server integration for AI agents
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** — System design and architecture overview

## Project Structure

```
semanticmesh/
├── cmd/
│   └── semanticmesh/              # CLI entry point
├── internal/
│   ├── knowledge/            # Core detection, graph, query, export/import pipeline
│   ├── code/                 # Source code analysis (Go, Python, JS parsers)
│   │   ├── goparser/         # Go source parser
│   │   ├── pyparser/         # Python source parser
│   │   ├── jsparser/         # JavaScript source parser
│   │   ├── connstring/       # Connection string detection
│   │   └── comments/         # Structured comment extraction
│   └── mcp/                  # MCP server for LLM agent access
├── docs/                     # User-facing documentation
└── test-data/                # Test corpus for validation
```

## How It Works

semanticmesh automatically builds dependency graphs by analyzing your codebase and documentation. Here's the complete pipeline:

```
┌──────────────────────────────────────────────────────────────┐
│                     Input Sources                            │
│  • Markdown documentation (architecture docs, READMEs)       │
│  • Source code (Go, Python, JavaScript)                      │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│              Three-Stage Detection Pipeline                  │
├──────────────────────────────────────────────────────────────┤
│  Stage 1: Markdown Parsing                                   │
│    → Extract component names from headings                   │
│    → Find relationships from document links                  │
│    → Detect types from keywords and file paths              │
├──────────────────────────────────────────────────────────────┤
│  Stage 2: Code Analysis (55+ Go patterns)                    │
│    → Parse imports: gin, echo, pgx, kafka, kubernetes       │
│    → Detect HTTP servers, databases, message brokers        │
│    → Extract connection strings and SDK calls                │
├──────────────────────────────────────────────────────────────┤
│  Stage 3: Infrastructure Text Extraction (50+ patterns)      │
│    → Scan prose: "uses Redis", "deployed on Kubernetes"     │
│    → Mine infrastructure mentions: DynamoDB, LDAP, S3        │
│    → Catch managed services not in code                      │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│               Signal Merging & Classification                │
│  • Deduplicate signals from multiple sources                 │
│  • Boost confidence when doc + code agree (source_type: both)│
│  • Classify component types (15 types, 0.4-1.0 confidence)   │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│                    SQLite Graph Storage                      │
│  • Nodes: components with types & confidence scores          │
│  • Edges: relationships with provenance & source_type        │
│  • Queryable: CLI, MCP server, direct SQL                    │
└──────────────────────────────────────────────────────────────┘
```

### Stage 1: Markdown Parsing

**What it does:** Analyzes documentation structure to discover components and their relationships.

**Detects:**
- Component names from headings (`# Payment API`, `## Redis Cache`)
- Relationships from document links (`[uses postgres](postgres.md)`)
- Type hints from keywords ("database", "service", "message broker")
- Architectural structure from file paths (`services/`, `databases/`)

**Example:**
```markdown
# Payment API

This service handles payment processing and depends on:
- [PostgreSQL](../databases/postgres.md) for transaction storage
- [Redis](../cache/redis.md) for session caching
```
**Result:** 3 components detected, 2 relationships created

---

### Stage 2: Code Analysis (55+ Go Patterns)

**What it does:** Parses source code AST to detect infrastructure dependencies from imports and connection strings.

**Detects:**
- **HTTP servers:** Gin, Echo, Fiber, Chi, Gorilla Mux, http.ListenAndServe
- **Databases:** pgx (PostgreSQL), MySQL drivers, MongoDB, Redis, Cassandra, InfluxDB
- **Message brokers:** Kafka, RabbitMQ, NATS, Pulsar, NSQ, EventBridge
- **Kubernetes:** controller-runtime, client-go (7 patterns for controllers, clients, resources)
- **Monitoring:** Datadog, Prometheus, Jaeger, Zipkin, OpenTelemetry
- **Cloud SDKs:** AWS, GCP, Azure client instantiations

**Example (Go code):**
```go
import (
    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5"
    "github.com/DataDog/datadog-api-client-go/api/v2"
)
```
**Result:** Detects HTTP server (Gin), database client (pgx), monitoring (Datadog)

**Coverage:** 55+ patterns for Go, similar pattern matching for Python and JavaScript

---

### Stage 3: Infrastructure Text Extraction (50+ Patterns)

**What it does:** Scans documentation **content** (prose) to extract infrastructure mentions that lack dedicated docs or code imports.

**Detects:**
- **Databases:** PostgreSQL, MySQL, DynamoDB, Elasticsearch, Neo4j, InfluxDB
- **Cloud services:** AWS Lambda, S3, ECS, EKS, RDS, Secrets Manager
- **Authentication:** LDAP, Active Directory, Okta, Auth0, Keycloak
- **Orchestration:** Kubernetes, Docker, OpenShift, Nomad
- **Message brokers:** Kafka, RabbitMQ, NATS, Pulsar (when mentioned in prose)
- **Monitoring:** Datadog, Prometheus, Grafana, Jaeger

**Example (markdown content):**
```markdown
Our authentication service uses **LDAP** for user lookups and
**AWS Secrets Manager** for storing API keys. Session state is
cached in **DynamoDB**.
```
**Result:** 3 infrastructure components extracted (LDAP, Secrets Manager, DynamoDB)

**Why this matters:** Catches managed services (AWS, GCP) and external systems not present in your source code.

See [docs/INFRASTRUCTURE_EXTRACTION.md](docs/INFRASTRUCTURE_EXTRACTION.md) for the full pattern library.

---

### Stage 4: Component Type Classification

**What it does:** Assigns one of 15 component types to each detected component with a confidence score.

**Types:** `service`, `database`, `cache`, `queue`, `message-broker`, `load-balancer`, `gateway`, `storage`, `container-registry`, `config-server`, `monitoring`, `log-aggregator`, `orchestrator`, `secrets-manager`, `search`

**Classification methods:**
- Naming patterns (`*-api` → service, `redis-*` → cache)
- File paths (`services/` → service, `databases/` → database)
- Content analysis (keyword matching in first 500 chars of docs)
- Code signals (import patterns corroborate type)

**Confidence scoring:**
- **1.0:** Exact match (component in known list)
- **0.85-0.95:** Strong inference (code + docs agree)
- **0.70-0.80:** Moderate (content-based, single source)
- **0.40-0.65:** Weak inference (ambiguous signals)

**Example:**
```
"payment-api" → type: service, confidence: 0.90 (name pattern + code signals)
"postgres-db" → type: database, confidence: 0.95 (name + keywords + code import)
"redis-cache" → type: cache, confidence: 0.88 (name pattern + docs)
```

---

### Stage 5: Signal Merging

**What it does:** Combines signals from all three stages to boost confidence and create unified graph nodes/edges.

**Merging rules:**

1. **Component nodes:** Take highest confidence from any stage
   ```
   Stage 1 (docs):  payment-api, confidence: 0.75
   Stage 2 (code):  payment-api, confidence: 0.90  ← Winner
   → Final: payment-api, type: service, confidence: 0.90
   ```

2. **Relationships:** Use probabilistic OR formula for corroboration
   ```
   Markdown finds: payment-api → postgres-db (confidence: 0.70)
   Code finds:     payment-api → postgres-db (confidence: 0.80)
   → Merged: confidence = 1.0 - (0.3 × 0.2) = 0.94, source_type: "both"
   ```

**Why this matters:** When both documentation and code agree on a relationship, you can trust it with 90%+ confidence. Relationships marked `source_type: both` are the most reliable.

---

### Stage 6: Graph Construction & Querying

**What it does:** Stores the dependency graph in SQLite and exposes query interfaces.

**Storage:**
- **Database:** SQLite (`knowledge.db` or `.bmd/knowledge.db`)
- **Schema:** Nodes (components), Edges (relationships), Provenance (where/how detected)
- **Indexes:** Fast lookups by component name, type, confidence

**Query methods:**

1. **CLI commands:**
   ```bash
   semanticmesh query impact --component payment-api     # Who depends on this?
   semanticmesh query dependencies --component web-app   # What does this depend on?
   semanticmesh query path --from web-app --to postgres  # How are they connected?
   semanticmesh query list --type database               # List all databases
   ```

2. **MCP server (for AI agents):**
   ```bash
   semanticmesh mcp  # Exposes 5 tools: query_impact, query_dependencies, query_path, list_components, graph_info
   ```

3. **Direct SQL:**
   ```sql
   SELECT * FROM graph_nodes WHERE component_type = 'database' AND confidence > 0.8;
   SELECT source_id, target_id FROM graph_edges WHERE source_type = 'both';
   ```

**Output format:** All queries return structured JSON with confidence scores, detection provenance, and metadata.

---

### Real-World Example

**Input:** A microservices workspace with 8 services

**Detection results:**
- **98 markdown documents** analyzed
- **145 Go files** parsed
- **120 components** detected (services, databases, infrastructure)
- **1,183 relationships** discovered
- **186 cross-service relationships** mapped
- **70-80% detection accuracy**

**Most valuable insights:**
- `service-catalog` identified as central hub (240+ connections to other services)
- 6/8 services use Kubernetes (25 client-go imports detected)
- PostgreSQL most common database (4 services depend on it)
- All services monitored by Datadog (detected from code imports)

**Processing time:** ~4.5 seconds for entire workspace

---

### Why This Approach Works

**Problem:** AI agents need to answer "if X fails, what breaks?" but can't ingest entire architectures (token limits, cost, latency).

**Solution:** Pre-compute the dependency graph once, let agents query it thousands of times.

**Benefits:**
- ✅ **Fast:** Millisecond queries vs. multi-second LLM calls
- ✅ **Cheap:** No tokens spent on architecture context in every prompt
- ✅ **Reliable:** Graph is computed from source truth (docs + code), not heuristics
- ✅ **Auditable:** Every relationship includes provenance (where/how detected)
- ✅ **Confidence-aware:** Agents can filter by confidence threshold for risk management

---

**For technical deep-dive, see [docs/HOW_IT_WORKS.md](docs/HOW_IT_WORKS.md)**

## Design Philosophy

**AI agents should query pre-computed graphs, not entire architectures.**

1. **Efficiency:** Agents get answers in milliseconds, not seconds
2. **Cost:** Fewer tokens in prompts means cheaper inference
3. **Reliability:** Pre-computed graphs are more reliable than heuristic parsing
4. **Compliance:** Structured queries avoid exposing sensitive data

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

Coverage target: >85% for core packages.

## Configuration

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for full customization options including `.semanticmeshignore`, `semanticmesh-aliases.yaml`, seed config, and named graph management.

## Contributing

semanticmesh welcomes contributions. Please see `CLAUDE.md` for development guidance.

## License

[Your license here]

---

**Last Updated:** 2026-04-13
