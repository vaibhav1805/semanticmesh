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
| Python | Regex-based | `.py` |
| JavaScript/TypeScript | Regex-based | `.js`, `.ts`, `.jsx`, `.tsx` |

The Go parser uses the standard library's `go/ast` package for precise detection. Python and JS/TS parsers use regex-based pattern matching against known library call signatures.

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
