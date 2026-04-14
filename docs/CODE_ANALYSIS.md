# Code Analysis

semanticmesh analyzes source code to detect infrastructure dependencies -- HTTP calls, database connections, cache clients, message broker interactions, and more. These signals supplement the relationships discovered from markdown documentation, producing a richer dependency graph.

## Enabling Code Analysis

Add the `--analyze-code` flag to any of the following commands:

```bash
# During export
semanticmesh export --input ./myproject --output graph.zip --analyze-code

# During crawl
semanticmesh crawl --input ./myproject --analyze-code

# During index
semanticmesh index --dir ./myproject --analyze-code
```

When enabled, semanticmesh walks the project directory, dispatches each source file to the appropriate language parser, and merges the resulting signals into the dependency graph alongside documentation-derived relationships.

## Supported Languages

| Language | Parser Type | File Extensions |
|----------|-------------|-----------------|
| Go | AST-based (go/ast) | `.go` |
| Python | Enhanced regex-based | `.py` |
| JavaScript/TypeScript | Enhanced regex-based + optional esbuild | `.js`, `.ts`, `.jsx`, `.tsx` |
| Mendix | mxcli Go API-based catalog analysis | `.mpr` |

The Go parser uses the standard library's `go/ast` package for precise detection. Python and JS/TS parsers use enhanced regex-based pattern matching with improved import resolution. The Mendix parser uses the mxcli Go library to query the application catalog for external dependencies (no external binary installation required).

### Import Resolution Enhancements

**Python:**
- Multi-item imports: `from redis import Redis, StrictRedis, ConnectionPool`
- Multi-line imports: `from package import (A, B, C)`
- Dotted package imports: `from psycopg2.pool import SimpleConnectionPool`
- Aliased multi-item imports: `from requests import get as http_get, post as http_post`

**JavaScript/TypeScript:**
- Dynamic imports: `const { Pool } = await import('pg')`
- ESM and CommonJS mixed in same file
- Scoped packages: `@aws-sdk/client-s3`, `@prisma/client`
- Optional esbuild integration for higher accuracy (auto-detects complex import patterns)

## What Each Parser Detects

All three parsers detect the same categories of infrastructure dependencies:

- **HTTP calls** -- outbound requests to other services
- **Database connections** -- connection establishment to SQL and NoSQL databases
- **Cache clients** -- Redis, Memcached, and similar cache connections
- **Queue/broker interactions** -- Kafka producers/consumers, RabbitMQ, NATS, SQS

Each parser also includes:
- **Comment analysis** -- explicit dependency mentions, TODO/FIXME annotations, URLs in comments
- **Environment variable references** -- connection-related env vars like `DATABASE_URL`, `REDIS_HOST`

### Go Detection Patterns

The Go parser resolves imports (including aliases and versioned modules) and matches function calls against known patterns:

| Pattern | Kind | Target Type | Confidence |
|---------|------|-------------|------------|
| `net/http.Get`, `Post`, `NewRequest` | http_call | service | 0.9 |
| `database/sql.Open` | db_connection | database | 0.85 |
| `github.com/jmoiron/sqlx.Connect`, `Open` | db_connection | database | 0.85 |
| `gorm.io/gorm.Open` | db_connection | database | 0.85 |
| `go.mongodb.org/mongo-driver/mongo.Connect` | db_connection | database | 0.85 |
| `github.com/redis/go-redis/v9.NewClient` | cache_client | cache | 0.85 |
| `github.com/go-redis/redis/v8.NewClient` | cache_client | cache | 0.85 |
| `github.com/bradfitz/gomemcache/memcache.New` | cache_client | cache | 0.85 |
| `github.com/IBM/sarama.NewSyncProducer`, `NewAsyncProducer` | queue_producer | message-broker | 0.85 |
| `github.com/IBM/sarama.NewConsumer`, `NewConsumerGroup` | queue_consumer | message-broker | 0.85 |
| `github.com/rabbitmq/amqp091-go.Dial` | queue_producer | message-broker | 0.85 |
| `github.com/nats-io/nats.go.Connect` | queue_producer | message-broker | 0.85 |
| `github.com/aws/aws-sdk-go-v2/service/sqs.NewFromConfig` | queue_producer | queue | 0.85 |

**Example:**

```go
import "net/http"

func callPaymentService() {
    resp, err := http.Get("https://payment-api.internal:8080/charge")
    // Detected: http_call -> payment-api.internal (confidence 0.9)
}
```

### Python Detection Patterns

The Python parser resolves `import X`, `import X as Y`, and `from X import Y` statements, then matches function calls:

| Pattern | Kind | Target Type | Confidence |
|---------|------|-------------|------------|
| `requests.get`, `post`, `put`, `delete`, `patch` | http_call | service | 0.9 |
| `httpx.get`, `post`, `put`, `delete` | http_call | service | 0.9 |
| `psycopg2.connect` | db_connection | database | 0.85 |
| `asyncpg.connect`, `create_pool` | db_connection | database | 0.85 |
| `pymongo.MongoClient` | db_connection | database | 0.85 |
| `sqlalchemy.create_engine` | db_connection | database | 0.85 |
| `pymysql.connect` | db_connection | database | 0.85 |
| `redis.Redis`, `StrictRedis`, `from_url` | cache_client | cache | 0.85 |
| `pymemcache.client.Client` | cache_client | cache | 0.85 |
| `kafka.KafkaProducer` | queue_producer | message-broker | 0.85 |
| `kafka.KafkaConsumer` | queue_consumer | message-broker | 0.85 |
| `pika.BlockingConnection` | queue_producer | message-broker | 0.85 |
| `boto3.client("sqs")` | queue_producer | queue | 0.85 |
| `flask.Flask`, `@app.route` decorators | http_server | service | 0.9 |
| `fastapi.FastAPI`, `@app.get/@app.post` decorators | http_server | service | 0.85-0.9 |
| `psycopg2.pool.SimpleConnectionPool`, `ThreadedConnectionPool` | db_connection | database | 0.85 |
| `redis.ConnectionPool` | cache_client | cache | 0.85 |
| `sqlalchemy.sessionmaker`, `sqlalchemy.orm.Session` | db_connection | database | 0.85 |
| `boto3.resource("dynamodb")` | db_connection | database | 0.85 |
| `elasticsearch.Elasticsearch` | search_client | search | 0.85 |
| `celery.Celery` | queue_producer | message-broker | 0.85 |

**Example:**

```python
import requests
from redis import Redis

def process_order(order_id):
    resp = requests.post("https://inventory-service.internal/reserve", json={"id": order_id})
    # Detected: http_call -> inventory-service.internal (confidence 0.9)

    cache = Redis(host="redis-primary.internal")
    # Detected: cache_client -> redis-primary.internal (confidence 0.85)
```

### JavaScript/TypeScript Detection Patterns

The JS parser resolves ESM imports (`import X from 'pkg'`, `import { X } from 'pkg'`), CommonJS require (`const X = require('pkg')`), and destructured requires:

| Pattern | Kind | Target Type | Confidence |
|---------|------|-------------|------------|
| `axios.get`, `post`, `put`, `delete`, `patch`, `request` | http_call | service | 0.9 |
| `fetch(url)` (global, no import needed) | http_call | service | 0.9 |
| `got.get`, `post` | http_call | service | 0.9 |
| `new Pool()`, `new Client()` (pg) | db_connection | database | 0.85 |
| `mysql2.createConnection`, `createPool` | db_connection | database | 0.85 |
| `mongoose.connect` | db_connection | database | 0.85 |
| `new MongoClient()` (mongodb) | db_connection | database | 0.85 |
| `new PrismaClient()` (@prisma/client) | db_connection | database | 0.85 |
| `new Sequelize()` | db_connection | database | 0.85 |
| `new Redis()` (ioredis) | cache_client | cache | 0.85 |
| `redis.createClient` | cache_client | cache | 0.85 |
| `new Kafka()` (kafkajs) | queue_producer | message-broker | 0.85 |
| `amqplib.connect` | queue_producer | message-broker | 0.85 |
| `new SQSClient()` (@aws-sdk/client-sqs) | queue_producer | queue | 0.85 |
| `express.Router` | http_server | service | 0.9 |
| `fastify()` | http_server | service | 0.9 |
| `new Koa()` (koa) | http_server | service | 0.9 |
| `typeorm.createConnection`, `new DataSource()` | db_connection | database | 0.85 |
| `knex.knex()` | db_connection | database | 0.85 |
| `new NodeCache()` (node-cache) | cache_client | cache | 0.85 |
| `new Client()` (@elastic/elasticsearch) | search_client | search | 0.85 |
| `new S3Client()` (@aws-sdk/client-s3) | storage_client | storage | 0.85 |
| `new DynamoDBClient()` (@aws-sdk/client-dynamodb) | db_connection | database | 0.85 |
| `new ApolloServer()` (apollo-server, @apollo/server) | http_server | service | 0.9 |

**Example:**

```typescript
import axios from 'axios';
import { Pool } from 'pg';

const pool = new Pool({ connectionString: 'postgresql://orders-db.internal:5432/orders' });
// Detected: db_connection -> orders-db.internal (confidence 0.85)

const resp = await axios.get('https://auth-service.internal/verify');
// Detected: http_call -> auth-service.internal (confidence 0.9)
```

## Connection String Detection

When a parser finds a string argument in a detected call, semanticmesh attempts to parse it as a connection string to extract the target hostname. Supported formats:

| Format | Example | Inferred Type |
|--------|---------|---------------|
| URL with scheme | `postgresql://db-host:5432/mydb` | database |
| URL with scheme | `redis://cache-01:6379` | cache |
| URL with scheme | `amqp://broker.internal:5672` | message-broker |
| URL with scheme | `https://api.internal/v1` | service |
| MySQL DSN | `user:pass@tcp(db-host:3306)/mydb` | database |
| PostgreSQL key-value | `host=db-host port=5432 dbname=mydb` | database |
| Bare host:port | `kafka-broker:9092` | unknown |

Recognized URL schemes: `postgres`, `postgresql`, `mysql`, `mongodb`, `mongodb+srv`, `redis`, `rediss`, `amqp`, `amqps`, `nats`, `http`, `https`.

Loopback addresses (`localhost`, `127.0.0.1`, `0.0.0.0`), documentation domains (`example.com`), and common reference sites (`github.com`, `stackoverflow.com`, etc.) are automatically filtered out.

## Comment Analysis

semanticmesh scans comments in all supported languages for dependency hints. Three patterns are recognized:

### Explicit Dependency Mentions

Comments containing verbs like `calls`, `depends on`, `uses`, `connects to`, `talks to`, `sends to`, `reads from`, or `writes to` followed by a component name:

```go
// This handler calls payment-service to process refunds
// Detected: comment_hint -> payment-service (confidence 0.4)
```

### TODO/FIXME Annotations

TODO, FIXME, HACK, and XXX annotations that reference component names with infrastructure suffixes (`-service`, `-api`, `-db`, `-cache`, `-queue`, `-broker`, `-cluster`):

```python
# TODO: migrate from legacy-db to new orders-db
# Detected: comment_hint -> orders-db (confidence 0.3)
```

### URLs in Comments

URLs found in comments are parsed for hostnames. Documentation and reference URLs are filtered out:

```javascript
// Dashboard endpoint: https://metrics-api.internal/dashboard
// Detected: comment_hint -> metrics-api.internal (confidence 0.4)
```

### Confidence Boosting

Comment-based signals that reference components also detected by code-level analysis (HTTP calls, DB connections, etc.) receive a confidence boost from 0.4 to 0.5. This rewards comment mentions that corroborate what the code already demonstrates.

## Environment Variable References

All parsers detect references to environment variables that appear to hold connection information. Supported reference styles:

| Language | Syntax |
|----------|--------|
| Go | `os.Getenv("VAR")`, `os.LookupEnv("VAR")` |
| Python | `os.environ["VAR"]`, `os.environ.get("VAR")`, `os.getenv("VAR")` |
| JavaScript | `process.env.VAR`, `process.env["VAR"]` |
| Shell | `${VAR}`, `$VAR` |

An env var is classified as connection-related if it matches either:

- **Suffixes:** `_URL`, `_DSN`, `_URI`, `_HOST`, `_ADDR`, `_ENDPOINT`, `_CONNECTION`
- **Prefixes:** `DATABASE_`, `DB_`, `REDIS_`, `MONGO_`, `RABBIT_`, `KAFKA_`, `NATS_`, `AMQP_`

Env var references are detected with confidence **0.7** and their target type is inferred from the prefix (e.g., `REDIS_URL` -> cache, `DATABASE_URL` -> database).

## Confidence Scoring

Each detection type carries a default confidence score:

| Detection Type | Confidence | Rationale |
|----------------|------------|-----------|
| HTTP call (API call) | 0.9 | High certainty -- explicit outbound request |
| Database connection | 0.85 | High certainty -- explicit connection establishment |
| Cache client | 0.85 | High certainty -- explicit client creation |
| Queue producer/consumer | 0.85 | High certainty -- explicit broker interaction |
| Connection string parse | 0.85 | High certainty -- structured connection format |
| Environment variable ref | 0.7 | Medium certainty -- indirection through env var |
| Comment hint (corroborated) | 0.5 | Low-medium -- text mention backed by code signal |
| Comment hint (explicit verb) | 0.4 | Low -- text mention only, no code corroboration |
| Comment hint (URL) | 0.4 | Low -- URL in comment, may be documentation |
| Comment hint (TODO/FIXME) | 0.3 | Lowest -- aspirational or incomplete reference |

## Source Component Inference

semanticmesh automatically infers the name of the component whose source code is being analyzed by searching for project manifests. It walks up the directory tree checking for:

1. **go.mod** -- uses the module path (e.g., `github.com/myorg/payment-service`)
2. **pyproject.toml** -- uses the `name` field from the `[project]` section
3. **setup.py** -- uses the `name` argument in `setup()`
4. **package.json** -- uses the `name` field

If no manifest is found, the base directory name is used as fallback.

## Excluding Files with .semanticmeshignore

Place a `.semanticmeshignore` file in the project root to control which files and directories are excluded from scanning. The format follows a simple pattern:

```
# Comments start with #
# Blank lines are ignored

# Directory patterns end with /
vendor/
node_modules/
.git/
__pycache__/
dist/
build/

# File patterns (glob wildcards supported)
*.lock
temp_*
```

If no `.semanticmeshignore` file exists, these directories are excluded by default:
`vendor`, `node_modules`, `.git`, `__pycache__`, `.venv`, `dist`, `build`, `target`, `.gradle`, `.next`, `out`, `.cache`, `bin`, `obj`, `.bmd`, `.planning`

The code analyzer also skips test files automatically:
- Go: `*_test.go`
- Python: `test_*.py`, `*_test.py`, `conftest.py`
- JavaScript/TypeScript: `*.test.*`, `*.spec.*`

## Component Name Normalization with semanticmesh-aliases.yaml

When different parts of your codebase refer to the same component by different names, use `semanticmesh-aliases.yaml` to normalize them to a single canonical name:

```yaml
aliases:
  postgres-primary:
    - pg-main
    - primary-db
    - pgdb
  redis-cache:
    - redis-01
    - cache-server
  payment-service:
    - payment-api
    - payments
```

Place this file in the project root. During graph construction, all alias names resolve to their canonical name, preventing duplicate nodes for the same component.

## Example Workflows

### Analyze a Go microservice

```bash
# Export with code analysis enabled
semanticmesh export --input ./payment-service --output payment.zip --analyze-code

# Import and query
semanticmesh import payment.zip --name payment
semanticmesh query impact --component payment-service --graph payment
```

### Analyze a Python project

```bash
semanticmesh export --input ./order-processor --output orders.zip --analyze-code
semanticmesh import orders.zip --name orders

# See what the order processor depends on
semanticmesh query deps --component order-processor --graph orders
```

### Combine documentation and code signals

```bash
# Index documentation first, then overlay code signals
semanticmesh index --dir ./docs --analyze-code

# Code signals (source_type: code) and documentation signals (source_type: markdown)
# are combined in the same graph. Filter by source:
semanticmesh query list --type database --source-type code --graph myproject
```

## Mendix Application Analysis

semanticmesh performs comprehensive catalog extraction from Mendix applications using the mxcli Go library. The parser extracts 14+ catalog tables covering published APIs, domain models, business logic, UI structure, and configuration—delivering complete architectural analysis in ~1.5-2.3 seconds. The mxcli library is bundled as a Go module dependency—no external binary installation required.

### Overview

The Mendix parser provides three configurable extraction profiles for different use cases:

- **Minimal Profile (~1.6s):** Fast scans for CI/CD pipelines—extracts external dependencies and published APIs
- **Standard Profile (~2.3s, default):** Recommended for impact analysis—includes business logic and UI structure
- **Comprehensive Profile (~1.5s):** Complete architectural analysis with internal dependency tracking

All profiles can extract from typical Mendix projects with 20-50 modules and hundreds of architectural elements.

### What Gets Extracted

#### Tier 1: External Dependencies & Published Services

**Published Services (NEW):**
- REST APIs (multiple per application)
- OData APIs
- Published operations with HTTP methods, paths, and backing microflows

**Domain Model (NEW):**
- Entities (100-300 in typical apps)
- Attributes and data types
- External entities (OData/REST consumed)

#### Tier 2: Internal Structure

**Business Logic (NEW):**
- Microflows (200-500 in typical apps)
- Java Actions (100-200 in typical apps)
- Complexity metrics and activity counts

**UI Structure (NEW):**
- Pages (100+ in typical apps)
- Navigation entry points
- Page templates and widgets

**Configuration (NEW):**
- Constants (dozens to hundreds)
- Settings and environment variables

#### External Dependencies

- REST clients (consumed REST services)
- OData clients (consumed OData feeds)
- External entities (external database tables)

#### Internal Structure

- Modules (20-50 in typical apps)
- Module dependencies (cross-module references)

### Extraction Profiles

semanticmesh offers three extraction profiles for different use cases:

#### Minimal Profile (~1-2s)

**What it extracts:** Tier 1 only—external dependencies and published APIs

**Use for:**
- CI/CD pipelines requiring fast scans
- Quick dependency checks ("what does this app depend on?")
- Initial architecture discovery

**Tables extracted:** `modules`, `published_rest_services`, `published_rest_operations`, `entities`

**Example:** 20-30 modules, multiple published APIs, 100-200 entities

#### Standard Profile (~2-3s, default)

**What it extracts:** Tier 1 + Tier 2—business logic and UI structure

**Use for:**
- Impact analysis ("if X fails, what breaks?")
- Architecture documentation
- Service dependency mapping
- Change impact assessment

**Tables extracted:** `modules`, `published_rest_services`, `published_rest_operations`, `entities`, `microflows`, `java_actions`, `pages`, `constants`

**Example:** 500-1000 total items (modules, APIs, entities, microflows, Java actions, pages, constants)

#### Comprehensive Profile (~1-2s)

**What it extracts:** All tiers + internal dependencies (module-to-module, microflow call graphs)

**Use for:**
- Complete architectural analysis
- Refactoring planning
- Deep investigation of internal structure
- Module coupling analysis

**Tables extracted:** All Standard tables + `module_dependencies`, `microflow_calls`

**Example:** 500-1000+ items with internal dependency tracking

### Configuration

Three ways to configure extraction profiles:

#### 1. Profile Presets (Recommended)

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extraction_profile: "standard"  # minimal, standard, or comprehensive
```

#### 2. Fine-Grained Control

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extract_published_apis: true     # Extract REST/OData APIs this app exposes
    extract_domain_model: true       # Extract entities and attributes
    extract_business_logic: true     # Extract microflows and Java actions
    extract_ui_structure: true       # Extract pages and navigation
    extract_configuration: true      # Extract constants and settings
    include_internal_deps: false     # Include module and microflow dependencies
    detect_modules_as_components: false  # Create component for each module
```

#### 3. Legacy Configuration (Still Supported)

```yaml
# semanticmesh.yaml
mendix:
  enabled: true
  catalog_refresh: true              # Build catalog before analysis (recommended)
  include_internal_deps: false       # Include module-to-module dependencies
  detect_modules_as_components: false  # Create separate components for each module
```

### Detection Patterns

| Pattern Type | Example | Target Type | Confidence |
|--------------|---------|-------------|------------|
| MPR File | `MyApp.mpr` | service | 0.95 |
| REST API (Published) | `PRS_OrderAPI` | rest-api | 0.95 |
| OData API (Published) | `PublicDataAPI` | odata-api | 0.95 |
| Page | `Login.page.xml` | page | 0.90 |
| Microflow | `ACT_ProcessOrder` | microflow | 0.90 |
| Entity | `Customer.Entity` | entity | 0.85 |
| REST Client (Consumed) | External REST service | service | 0.90 |
| External Entity | External OData feed | database | 0.85 |
| Java Action | `ExportToExcel` | java-action | 0.85 |
| Module Reference | Cross-module call | service (internal) | 0.80 |
| Microflow Call | Microflow dependency | service (internal) | 0.75 |

### Detection Methods

1. **File Detection:** Scans for `.mpr` files or `mprcontents/` folders
2. **Catalog Build:** Opens MPR file and builds in-memory catalog using mxcli Go API
3. **Profile-Based Extraction:** Queries catalog tables based on selected profile
4. **Component Generation:** Creates typed components (rest-api, entity, microflow, page) with confidence scores
5. **Dependency Mapping:** Links components via module structure and internal calls (Comprehensive profile only)

**Performance:** Extracts 700+ items in ~1.5-2.3 seconds depending on profile. Direct Go API integration eliminates subprocess overhead.

### Example Output

```json
{
  "app_name": "YourMendixApp",
  "extraction_profile": "standard",
  "extraction_time": "2.3s",
  "tables_extracted": ["modules", "published_rest_services", "published_rest_operations", "entities", "microflows", "java_actions", "pages", "constants"],
  "published_apis": [
    {
      "name": "PRS_OrderAPI",
      "type": "rest",
      "path": "rest/orders",
      "module_name": "OrderModule",
      "operations": [
        {
          "resource_name": "OrderResource",
          "http_method": "POST",
          "path": "orders/{orderId}",
          "microflow": "OrderModule.ProcessOrder"
        }
      ]
    }
  ],
  "entities": [
    {
      "name": "Customer",
      "qualified_name": "CustomerModule.Customer",
      "module_name": "CustomerModule",
      "entity_type": "Entity",
      "attribute_count": 12,
      "is_external": false
    }
  ],
  "microflows": [
    {
      "name": "ACT_ProcessOrder",
      "qualified_name": "OrderModule.ACT_ProcessOrder",
      "module_name": "OrderModule",
      "type": "Microflow",
      "activity_count": 15,
      "complexity": 8,
      "is_scheduled": false
    }
  ],
  "pages": [
    {
      "name": "Login",
      "qualified_name": "AuthModule.Login",
      "module_name": "AuthModule"
    }
  ],
  "constants": [
    {
      "name": "API_Timeout",
      "qualified_name": "ConfigModule.API_Timeout",
      "module_name": "ConfigModule",
      "value": "30000"
    }
  ],
  "components": [
    {"name": "PRS_OrderAPI", "type": "rest-api", "confidence": 0.95},
    {"name": "Customer", "type": "entity", "confidence": 0.85},
    {"name": "ACT_ProcessOrder", "type": "microflow", "confidence": 0.90}
  ]
}
```

### Examples

**Basic Mendix app analysis (Standard profile):**
```bash
semanticmesh export --input ./MyMendixApp --output app.zip --analyze-code
```

**Fast CI/CD scan (Minimal profile):**
```bash
# Add to semanticmesh.yaml:
# code_analysis:
#   mendix:
#     extraction_profile: "minimal"

semanticmesh export --input ./MyMendixApp --output app.zip --analyze-code
```

**Deep architectural analysis (Comprehensive profile):**
```bash
# Add to semanticmesh.yaml:
# code_analysis:
#   mendix:
#     extraction_profile: "comprehensive"

semanticmesh export --input ./MyMendixApp --output app.zip --analyze-code
```

**Multi-app workspace:**
```bash
# Analyze multiple Mendix apps together
semanticmesh export --input ./workspace --output workspace.zip --analyze-code
```

**Query Mendix dependencies:**
```bash
# Query external dependencies
semanticmesh query dependencies --component YourMendixApp --type database

# Find impact of a published API
semanticmesh query impact --component PRS_OrderAPI --depth all

# List all published APIs
semanticmesh query list --type rest-api --graph myproject

# Find all microflows in a module
semanticmesh query list --type microflow --filter "module:OrderModule" --graph myproject
```

### Performance Metrics

| Profile | Time | Items Extracted | Tables | Use Case |
|---------|------|-----------------|--------|----------|
| Minimal | ~1-2s | 150-200 | 4 | CI/CD pipelines, quick scans |
| Standard | ~2-3s | 500-1000 | 8 | Impact analysis, architecture docs |
| Comprehensive | ~1-2s | 500-1000+ | 10+ | Deep investigation, refactoring |

**Example:** Typical mid-sized Mendix app (20-30 modules, 100-200 entities, 200-300 microflows, multiple REST APIs)

**Improvement over legacy approach:** 3x more data extracted (14+ tables vs 5 tables), similar performance

### Troubleshooting

**Cannot analyze apps open in Mendix Studio Pro:**
- Close the MPR file in Studio Pro before running semanticmesh
- MPR file must not be locked by another process

**Large apps (>50 modules) take 30-60 seconds:**
- This is expected for catalog build on first analysis
- Subsequent analyses with `catalog_refresh: false` are faster
- Use Minimal profile for faster scans

**Analysis fails with "invalid MPR file":**
- Ensure the `.mpr` file is a valid Mendix project file
- Check that the mxcli library supports your Mendix version
- Try opening the app in Mendix Studio Pro to verify it's not corrupted

**Missing expected components:**
- Check extraction profile—Minimal profile excludes business logic and UI
- Verify `catalog_refresh: true` for up-to-date analysis
- Use Comprehensive profile for maximum coverage
