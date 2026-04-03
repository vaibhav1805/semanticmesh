# semanticmesh

A dependency graph analyzer for infrastructure documentation and source code. Query component relationships and answer critical questions like "if this fails, what breaks?" without feeding entire architecture to AI agents.

## What Is semanticmesh?

semanticmesh scans your infrastructure documentation (markdown files) and source code (Go, Python, JavaScript), automatically detects components, classifies them by type, and builds a queryable dependency graph. This enables AI agents and operators to:

- **Query impact:** "If this database fails, which services break?"
- **Trace dependencies:** "What does payment-api depend on?"
- **Find paths:** "How does the web frontend connect to the primary database?"
- **List components:** "Show all services with confidence above 0.7"
- **Analyze code:** Detect infrastructure dependencies from connection strings, SDK calls, and imports

Instead of feeding AI agents your entire architecture, they query the pre-computed graph — faster, cheaper, and more reliable.

## Features

### Dual-Signal Detection

semanticmesh merges signals from two sources into a single hybrid dependency graph:

- **Markdown analysis:** Extracts components and relationships from infrastructure documentation
- **Code analysis:** Detects database connections, service URLs, message queue bindings, and cache clients from Go, Python, and JavaScript source code

When both sources corroborate a relationship, the edge is tagged `source_type: both`, giving higher confidence.

### Component Classification

Every component carries a type classification (one of 12 core types: `service`, `database`, `cache`, `queue`, `message-broker`, `load-balancer`, `gateway`, `storage`, `container-registry`, `config-server`, `monitoring`, `log-aggregator`) with a confidence score (0.4--1.0) reflecting detection reliability.

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

The server exposes five tools over stdio transport: `query_impact`, `query_dependencies`, `query_path`, `list_components`, and `semanticmesh_graph_info`. Configure it in your MCP client (e.g., Claude Desktop) to give agents on-demand access to your infrastructure graph.

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
- Infrastructure documented in markdown and/or source code in Go, Python, or JavaScript

### Installation

```bash
git clone https://github.com/your-org/semanticmesh
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

- **[docs/COMPONENT_TYPES.md](docs/COMPONENT_TYPES.md)** — Complete reference for all 12 component types, detection patterns, and confidence scoring
- **[docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md)** — Full command reference for all semanticmesh commands with examples and JSON output formats
- **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)** — Guide to `.semanticmeshignore`, `semanticmesh-aliases.yaml`, seed configuration, and named graph management
- **[docs/ADR_COMPONENT_TYPES.md](docs/ADR_COMPONENT_TYPES.md)** — Architecture decision record explaining design choices

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

### 1. Scanning

semanticmesh scans markdown documentation and extracts file paths, headings (component names), keywords (type detection signals), and relationships between components.

### 2. Code Analysis (Optional)

When `--analyze-code` is enabled, semanticmesh also parses Go, Python, and JavaScript source files to detect:
- Database connection strings (PostgreSQL, MySQL, Redis, MongoDB)
- HTTP client URLs and service calls
- Message queue bindings (RabbitMQ, Kafka, SQS)
- SDK client instantiations (AWS, GCP, etc.)

### 3. Component Type Detection

For each detected component, semanticmesh applies multiple detection algorithms — naming patterns, file paths, keywords, and code signals — producing `(name, type, confidence, detection_methods)`.

### 4. Signal Merging

Markdown-derived and code-derived edges are merged. When both sources find the same relationship, the edge is promoted to `source_type: both` with boosted confidence.

### 5. Packaging

The `export` command packages the graph as a ZIP archive containing `graph.db` (SQLite) and `metadata.json`. This archive is portable and versioned.

### 6. Querying

After importing a graph, query it via the CLI (`semanticmesh query`) or MCP server (`semanticmesh mcp`). All output is structured JSON designed for machine consumption.

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

**Last Updated:** 2026-04-03
