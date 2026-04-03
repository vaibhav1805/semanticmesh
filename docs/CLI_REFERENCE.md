# CLI Reference

Complete reference documentation for the semanticmesh command-line interface.

---

## Commands Overview

| Command | Description |
|---------|-------------|
| `semanticmesh export` | Scan, detect, and package a dependency graph as a ZIP archive |
| `semanticmesh import` | Load an exported graph ZIP into persistent storage |
| `semanticmesh query impact` | Query downstream impact of a component failure |
| `semanticmesh query dependencies` | Query what a component depends on |
| `semanticmesh query path` | Find dependency paths between two components |
| `semanticmesh query list` | List components with optional filters |
| `semanticmesh crawl` | Preview graph statistics before exporting |
| `semanticmesh mcp` | Start MCP server for LLM agent access (stdio transport) |
| `semanticmesh index` | Build component graph from markdown (legacy) |
| `semanticmesh list` | List components filtered by type (legacy) |
| `semanticmesh clean` | Remove all BMD artifacts from a directory |

---

## semanticmesh export — Package Dependency Graph

Runs the full export pipeline: scan markdown, detect components, apply aliases, discover relationships, optionally analyze source code, and package everything as a ZIP archive containing `graph.db` and `metadata.json`.

### Syntax

```bash
semanticmesh export [FLAGS]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--input PATH` | string | `.` | Source directory to scan. Alias: `--from`. |
| `--output FILE` | string | `graph.zip` | Output ZIP file path. `.zip` extension added automatically if missing. |
| `--analyze-code` | boolean | false | Analyze source code (Go, Python, JavaScript) for infrastructure dependencies. |
| `--skip-discovery` | boolean | false | Skip relationship discovery algorithms. |
| `--llm-discovery` | boolean | false | Enable LLM-based discovery (opt-in, off by default). |
| `--min-confidence F` | float | `0.5` | Minimum confidence threshold for discovered edges. |
| `--version STRING` | string | `1.0.0` | Semantic version tag embedded in the archive metadata. |
| `--git-version` | boolean | false | Auto-detect version from `git describe --tags`. |
| `--publish URI` | string | (none) | S3 URI to publish the artifact (e.g., `s3://bucket/prefix`). Requires AWS CLI. |
| `--db PATH` | string | (none) | Database path override (advanced). |

### Examples

#### Export from documentation

```bash
semanticmesh export --input ./docs --output graph.zip
```

#### Export with code analysis

```bash
semanticmesh export --input ./project --output graph.zip --analyze-code
```

#### Export with version tagging and S3 publish

```bash
semanticmesh export --input ./docs --output graph.zip --git-version --publish s3://my-bucket/graphs/
```

### Output

The ZIP archive contains:

- **`graph.db`** — SQLite database with the full component graph (nodes, edges, component mentions, code signals)
- **`metadata.json`** — Archive metadata: version, schema version, component/relationship counts, checksum, ignore patterns, aliases applied

---

## semanticmesh import — Load Exported Graph

Extracts a graph ZIP archive into XDG-compliant persistent storage (`$XDG_DATA_HOME/semanticmesh/` or `~/.local/share/semanticmesh/`). The imported graph becomes the default for subsequent queries.

### Syntax

```bash
semanticmesh import <file.zip> [--name NAME]
```

### Arguments

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `<file.zip>` | positional | Yes | Path to the ZIP archive to import. |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name NAME` | string | derived from filename | Name for the imported graph. Used to select graphs in queries via `--graph`. |

### Examples

#### Import with auto-derived name

```bash
semanticmesh import graph.zip
# Graph name: "graph" (derived from filename)
```

#### Import with explicit name

```bash
semanticmesh import prod-infra-v2.zip --name prod-infra
```

#### Replace an existing graph

Importing with the same name as an existing graph replaces it:

```bash
semanticmesh import updated-graph.zip --name prod-infra
# Replacing existing graph "prod-infra"
```

---

## semanticmesh query — Query the Dependency Graph

All query subcommands operate on imported graphs. They share a set of global flags and return structured JSON by default.

### Global Query Flags

These flags are available on all `semanticmesh query` subcommands:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--graph NAME` | string | most recent import | Select a named graph to query. |
| `--min-confidence F` | float | `0` | Filter relationships below this confidence threshold. |
| `--source-type S` | string | (all) | Filter by detection source: `markdown`, `code`, or `both`. |
| `--format json\|table` | string | `json` | Output format. |

The `--min-confidence` and `--source-type` filters compose independently: an edge must pass both filters to appear in results.

### JSON Envelope

All query responses use a consistent envelope:

```json
{
  "query": {
    "type": "impact",
    "component": "primary-db",
    "depth": 100,
    "min_confidence": 0,
    "source_type": ""
  },
  "results": { ... },
  "metadata": {
    "execution_time_ms": 2,
    "node_count": 42,
    "edge_count": 67,
    "graph_name": "prod-infra",
    "graph_version": "2.0.0",
    "created_at": "2026-04-01T10:30:00Z",
    "component_count": 42,
    "cycles_detected": []
  }
}
```

---

### semanticmesh query impact

Query downstream impact of a component failure. Answers: "if this fails, what breaks?"

Traverses incoming edges (reverse BFS) to find all components that depend on the target.

#### Syntax

```bash
semanticmesh query impact --component NAME [FLAGS]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--component NAME` | string | (required) | Component to analyze. |
| `--depth N\|all` | string | `1` | Traversal depth. Integer or `"all"` for unlimited. |
| `--include-provenance` | boolean | false | Include detection provenance (mentions) for each affected node. |
| `--max-mentions N` | int | `5` | Maximum mentions per component. `0` for unlimited. |

Plus [global query flags](#global-query-flags).

#### Examples

```bash
# Direct dependents only
semanticmesh query impact --component primary-db

# Full transitive impact
semanticmesh query impact --component primary-db --depth all

# With provenance details
semanticmesh query impact --component primary-db --include-provenance --max-mentions 3

# Only code-detected relationships
semanticmesh query impact --component redis-cache --source-type code

# Table output
semanticmesh query impact --component primary-db --format table
```

#### Results Schema

```json
{
  "affected_nodes": [
    {
      "name": "payment-api",
      "type": "service",
      "distance": 1,
      "confidence_tier": "high",
      "mentions": [...],
      "mention_count": 3
    }
  ],
  "relationships": [
    {
      "from": "payment-api",
      "to": "primary-db",
      "confidence": 0.85,
      "confidence_tier": "high",
      "type": "depends_on",
      "source_file": "architecture.md",
      "extraction_method": "link",
      "source_type": "markdown"
    }
  ]
}
```

---

### semanticmesh query dependencies

Query what a component depends on. Answers: "what does this need to work?"

Traverses outgoing edges (forward BFS) to find upstream dependencies.

#### Syntax

```bash
semanticmesh query dependencies --component NAME [FLAGS]
```

Alias: `semanticmesh query deps`

#### Flags

Same as [query impact](#semanticmesh-query-impact).

#### Examples

```bash
# Direct dependencies
semanticmesh query dependencies --component web-frontend

# All transitive dependencies
semanticmesh query deps --component web-frontend --depth all

# Only code-detected dependencies
semanticmesh query dependencies --component payment-api --source-type code
```

---

### semanticmesh query path

Find dependency paths between two components. Answers: "how does X connect to Y?"

Returns up to `--limit` paths with per-hop confidence scores.

#### Syntax

```bash
semanticmesh query path --from NAME --to NAME [FLAGS]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from NAME` | string | (required) | Source component. |
| `--to NAME` | string | (required) | Target component. |
| `--limit N` | int | `10` | Maximum number of paths to return. |

Plus [global query flags](#global-query-flags).

#### Examples

```bash
semanticmesh query path --from web-frontend --to primary-db
semanticmesh query path --from web-frontend --to primary-db --min-confidence 0.7
```

#### Results Schema

```json
{
  "paths": [
    {
      "nodes": ["web-frontend", "api-gateway", "primary-db"],
      "hops": [
        {
          "from": "web-frontend",
          "to": "api-gateway",
          "confidence": 0.9,
          "confidence_tier": "high",
          "source_file": "architecture.md",
          "extraction_method": "link"
        },
        {
          "from": "api-gateway",
          "to": "primary-db",
          "confidence": 0.85,
          "confidence_tier": "high",
          "source_file": "api-gateway.md",
          "extraction_method": "link"
        }
      ],
      "total_confidence": 0.765
    }
  ],
  "count": 1
}
```

---

### semanticmesh query list

List components with optional filters. Useful for exploring the graph before targeted queries.

#### Syntax

```bash
semanticmesh query list [FLAGS]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type TYPE` | string | (all) | Filter by component type (e.g., `service`, `database`). |

Plus [global query flags](#global-query-flags).

#### Examples

```bash
# List all components
semanticmesh query list

# List only services
semanticmesh query list --type service

# List services with high-confidence edges
semanticmesh query list --type service --min-confidence 0.7

# List from a specific named graph
semanticmesh query list --graph prod-infra --type database
```

#### Results Schema

```json
{
  "components": [
    {
      "name": "payment-api",
      "type": "service",
      "incoming_edges": 3,
      "outgoing_edges": 5
    }
  ],
  "count": 12
}
```

---

## semanticmesh crawl — Pre-Export Graph Diagnostic

Runs the same pipeline as `export` (scan, ignore, alias, detect, discover) but instead of packaging a ZIP, computes and displays graph statistics. Use this to preview what an export would produce.

### Syntax

```bash
semanticmesh crawl [FLAGS]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--input PATH` | string | `.` | Source directory to crawl. |
| `--format text\|json` | string | `text` | Output format. |
| `--analyze-code` | boolean | false | Include source code analysis. |
| `--from-multiple FILES` | string | (none) | Legacy targeted traversal mode (comma-separated starting files). |

### Examples

```bash
# Text summary
semanticmesh crawl --input ./docs

# JSON output for programmatic use
semanticmesh crawl --input ./project --analyze-code --format json
```

### JSON Output

```json
{
  "summary": {
    "component_count": 42,
    "relationship_count": 67,
    "quality_score": 85.2,
    "input_path": "/path/to/project"
  },
  "components": {
    "by_type": {
      "service": ["api-gateway", "payment-api"],
      "database": ["primary-db", "replica-db"]
    }
  },
  "confidence": {
    "tiers": [
      {"tier": "high", "range": [0.8, 1.0], "count": 30, "percentage": 44.8}
    ]
  },
  "quality_warnings": []
}
```

---

## semanticmesh mcp — MCP Server

Starts an MCP (Model Context Protocol) server on stdio transport. LLM agents connect to this server to query the dependency graph via tool calls.

### Syntax

```bash
semanticmesh mcp
```

No flags. The server runs until interrupted (SIGTERM/SIGINT).

### Exposed Tools

| Tool | Description |
|------|-------------|
| `query_impact` | Analyze downstream impact of a component failure |
| `query_dependencies` | Find what a component depends on |
| `query_path` | Find dependency paths between two components |
| `list_components` | List all components with optional type/confidence filters |
| `semanticmesh_graph_info` | Get metadata about the loaded graph |

All tools accept a `graph` parameter to select a specific named graph.

### MCP Client Configuration

Example configuration for Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "semanticmesh": {
      "command": "/path/to/semanticmesh",
      "args": ["mcp"]
    }
  }
}
```

---

## semanticmesh index — Build Component Graph (Legacy)

Index markdown documents, detect component types, and persist results to a local SQLite database. This is the original indexing command; for portable graphs, prefer `export` + `import`.

### Syntax

```bash
semanticmesh index [FLAGS]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir PATH` | string | `.` | Directory to index. |
| `--skip-discovery` | boolean | false | Skip relationship discovery algorithms. |
| `--llm-discovery` | boolean | false | Enable LLM-based discovery. |
| `--min-confidence F` | float | `0.5` | Minimum confidence threshold. |
| `--analyze-code` | boolean | false | Analyze source code for infrastructure dependencies. |

### Examples

```bash
semanticmesh index --dir ./docs
semanticmesh index --dir ./project --analyze-code
```

---

## semanticmesh list — Query Components by Type (Legacy)

List components from the indexed graph, filtered by type. For querying imported graphs, prefer `semanticmesh query list`.

### Syntax

```bash
semanticmesh list --type TYPE [FLAGS]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type TYPE` | string | (required) | Filter by component type. |
| `--include-tags` | boolean | false | Include tag-based matches in addition to primary type matches. |
| `--dir PATH` | string | `.` | Directory that was indexed. |

### Examples

```bash
semanticmesh list --type service --dir ./docs
semanticmesh list --type database --include-tags --dir ./docs
```

---

## semanticmesh clean — Remove Artifacts

Remove the `.bmd/` directory and all indexed data from a directory.

### Syntax

```bash
semanticmesh clean [--dir PATH]
```

---

## Other Commands

| Command | Description |
|---------|-------------|
| `semanticmesh depends --service NAME` | Show service dependencies (direct or `--transitive`). |
| `semanticmesh components --dir PATH` | List all discovered component names. |
| `semanticmesh context QUERY --dir PATH` | Assemble RAG context sections for a query. |
| `semanticmesh relationships --dir PATH` | List discovered relationships with confidence scores. |
| `semanticmesh graph --dir PATH` | Export the full graph as JSON or DOT format. |

---

## Confidence Tiers

All query results include human-readable confidence tiers:

| Tier | Range | Meaning |
|------|-------|---------|
| `very_high` | 0.95--1.0 | Explicit naming or seed config |
| `high` | 0.80--0.94 | Clear but not explicit |
| `moderate` | 0.65--0.79 | Some ambiguity |
| `threshold` | 0.40--0.64 | Weak signal |

---

## Error Handling

Query commands return structured JSON errors on stdout for machine parsing:

```json
{
  "error": "component \"payment-api\" not found in graph",
  "code": "NOT_FOUND",
  "suggestions": ["payment-service", "payment-gateway"]
}
```

Error codes: `NOT_FOUND`, `NO_GRAPH`, `INVALID_ARG`.

---

**Last updated:** 2026-04-03
