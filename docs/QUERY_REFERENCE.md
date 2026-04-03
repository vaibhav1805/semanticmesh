# Query Reference

Complete reference for `semanticmesh query` commands. All query commands operate on imported graphs stored in `$XDG_DATA_HOME/semanticmesh/graphs/`.

---

## Overview

```
semanticmesh query <subcommand> [flags]
```

| Subcommand | Purpose |
|------------|---------|
| `impact` | Find what breaks if a component fails (reverse traversal) |
| `dependencies` (alias: `deps`) | Find what a component depends on (forward traversal) |
| `path` | Find paths between two components |
| `list` | List components with optional filters |

All commands return a consistent JSON envelope by default, designed for reliable parsing by AI agents.

---

## Common Flags

These flags are accepted by all query subcommands:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--graph <name>` | string | most recent import | Select a named graph to query |
| `--min-confidence <f>` | float | 0 (no filter) | Exclude relationships below this confidence threshold |
| `--source-type <s>` | string | (all) | Filter by detection source: `markdown`, `code`, or `both` |
| `--format json\|table` | string | `json` | Output format |

The `--min-confidence` and `--source-type` filters compose independently: an edge must pass **both** filters to appear in results.

---

## semanticmesh query impact

Find downstream dependents of a component — answers "if this component fails, what breaks?"

Performs a **reverse traversal** (follows incoming edges) from the target component, discovering everything that depends on it directly or transitively.

### Syntax

```bash
semanticmesh query impact --component <name> [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--component <name>` | string | **(required)** | Component to analyze |
| `--depth <n\|all>` | string | `1` | Traversal depth. Integer or `"all"` (caps at 100) |
| `--min-confidence <f>` | float | 0 | Minimum confidence threshold for edges |
| `--source-type <s>` | string | (all) | Filter by detection source |
| `--graph <name>` | string | current | Named graph to query |
| `--format json\|table` | string | `json` | Output format |
| `--include-provenance` | bool | `false` | Include detection provenance (file paths, methods) for each node |
| `--max-mentions <n>` | int | `5` | Maximum provenance mentions per node (0 = unlimited) |

### Example

```bash
semanticmesh query impact --component primary-db --depth all
```

```json
{
  "query": {
    "type": "impact",
    "component": "primary-db",
    "depth": 100
  },
  "results": {
    "affected_nodes": [
      {
        "name": "payment-api",
        "type": "service",
        "distance": 1,
        "confidence_tier": "strong-inference"
      },
      {
        "name": "web-frontend",
        "type": "service",
        "distance": 2,
        "confidence_tier": "moderate"
      }
    ],
    "relationships": [
      {
        "from": "payment-api",
        "to": "primary-db",
        "confidence": 0.85,
        "confidence_tier": "strong-inference",
        "type": "depends-on",
        "source_file": "docs/payment-api.md",
        "extraction_method": "structural",
        "source_type": "markdown"
      }
    ]
  },
  "metadata": {
    "execution_time_ms": 3,
    "node_count": 42,
    "edge_count": 67,
    "graph_name": "my-infra",
    "graph_version": "1.0.0",
    "created_at": "2026-03-15T10:30:00Z",
    "component_count": 42
  }
}
```

### With Provenance

```bash
semanticmesh query impact --component primary-db --include-provenance --max-mentions 2
```

When `--include-provenance` is set, each affected node includes detection details:

```json
{
  "name": "payment-api",
  "type": "service",
  "distance": 1,
  "confidence_tier": "strong-inference",
  "mentions": [
    {
      "file_path": "docs/payment-api.md",
      "detection_method": "structural",
      "confidence": 0.88,
      "context": "Architecture > Services"
    }
  ],
  "mention_count": 3
}
```

The `mention_count` field shows the total number of mentions even when `--max-mentions` truncates the array.

---

## semanticmesh query dependencies

Find what a component depends on — answers "what does this component need to work?"

Performs a **forward traversal** (follows outgoing edges) from the target component.

### Syntax

```bash
semanticmesh query dependencies --component <name> [flags]
semanticmesh query deps --component <name> [flags]
```

### Flags

Identical to `impact`:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--component <name>` | string | **(required)** | Component to analyze |
| `--depth <n\|all>` | string | `1` | Traversal depth |
| `--min-confidence <f>` | float | 0 | Minimum confidence threshold |
| `--source-type <s>` | string | (all) | Filter by detection source |
| `--graph <name>` | string | current | Named graph to query |
| `--format json\|table` | string | `json` | Output format |
| `--include-provenance` | bool | `false` | Include detection provenance |
| `--max-mentions <n>` | int | `5` | Maximum mentions per node |

### Example

```bash
semanticmesh query deps --component web-frontend --source-type code
```

```json
{
  "query": {
    "type": "dependencies",
    "component": "web-frontend",
    "depth": 1,
    "source_type": "code"
  },
  "results": {
    "affected_nodes": [
      {
        "name": "auth-service",
        "type": "service",
        "distance": 1,
        "confidence_tier": "strong-inference"
      }
    ],
    "relationships": [
      {
        "from": "web-frontend",
        "to": "auth-service",
        "confidence": 0.82,
        "confidence_tier": "strong-inference",
        "type": "depends-on",
        "source_file": "src/api/client.go",
        "extraction_method": "code-analysis",
        "source_type": "code"
      }
    ]
  },
  "metadata": {
    "execution_time_ms": 2,
    "node_count": 42,
    "edge_count": 67,
    "graph_name": "my-infra",
    "graph_version": "1.0.0",
    "created_at": "2026-03-15T10:30:00Z",
    "component_count": 42
  }
}
```

---

## semanticmesh query path

Find paths between two components, ranked by total confidence.

### Syntax

```bash
semanticmesh query path --from <name> --to <name> [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--from <name>` | string | **(required)** | Source component |
| `--to <name>` | string | **(required)** | Target component |
| `--limit <n>` | int | `10` | Maximum number of paths to return |
| `--min-confidence <f>` | float | 0 | Minimum confidence **per hop** |
| `--source-type <s>` | string | (all) | Filter by detection source |
| `--graph <name>` | string | current | Named graph to query |
| `--format json\|table` | string | `json` | Output format |

### Example

```bash
semanticmesh query path --from web-frontend --to primary-db
```

```json
{
  "query": {
    "type": "path",
    "from": "web-frontend",
    "to": "primary-db"
  },
  "results": {
    "paths": [
      {
        "nodes": ["web-frontend", "payment-api", "primary-db"],
        "hops": [
          {
            "from": "web-frontend",
            "to": "payment-api",
            "confidence": 0.85,
            "confidence_tier": "strong-inference",
            "source_file": "docs/frontend.md",
            "extraction_method": "structural"
          },
          {
            "from": "payment-api",
            "to": "primary-db",
            "confidence": 0.92,
            "confidence_tier": "explicit",
            "source_file": "docs/payment-api.md",
            "extraction_method": "explicit-link"
          }
        ],
        "total_confidence": 0.782
      }
    ],
    "count": 1
  },
  "metadata": { ... }
}
```

Paths are sorted by `total_confidence` descending. Total confidence is the product of all hop confidences along the path.

When no path exists, the response is still a success (exit 0) with an empty paths array and a `reason` field:

```json
{
  "results": {
    "paths": [],
    "count": 0,
    "reason": "no path found between web-frontend and primary-db"
  }
}
```

---

## semanticmesh query list

List all components in the graph with optional filtering.

### Syntax

```bash
semanticmesh query list [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type <name>` | string | (all) | Filter by component type (e.g., `service`, `database`) |
| `--min-confidence <f>` | float | 0 | Include only nodes with at least one connected edge meeting this threshold |
| `--source-type <s>` | string | (all) | Include only nodes with at least one connected edge matching this source type |
| `--graph <name>` | string | current | Named graph to query |
| `--format json\|table` | string | `json` | Output format |

### Example

```bash
semanticmesh query list --type service --min-confidence 0.7
```

```json
{
  "query": {
    "type": "list",
    "min_confidence": 0.7
  },
  "results": {
    "components": [
      {
        "name": "auth-service",
        "type": "service",
        "incoming_edges": 3,
        "outgoing_edges": 2
      },
      {
        "name": "payment-api",
        "type": "service",
        "incoming_edges": 1,
        "outgoing_edges": 4
      }
    ],
    "count": 2
  },
  "metadata": { ... }
}
```

Components are sorted alphabetically by name.

---

## JSON Envelope Structure

All query commands return a consistent envelope:

```json
{
  "query": { ... },
  "results": { ... },
  "metadata": { ... }
}
```

### query

Echoes back the parameters used for the query:

| Field | Type | Present |
|-------|------|---------|
| `type` | string | Always. One of: `impact`, `dependencies`, `path`, `list` |
| `component` | string | `impact`, `dependencies` |
| `from` | string | `path` |
| `to` | string | `path` |
| `depth` | int | `impact`, `dependencies` |
| `min_confidence` | float | When non-zero |
| `source_type` | string | When specified |

### results

Shape varies by query type:

| Query | Results type | Key fields |
|-------|-------------|------------|
| `impact` | `ImpactResult` | `affected_nodes[]`, `relationships[]` |
| `dependencies` | `ImpactResult` | `affected_nodes[]`, `relationships[]` |
| `path` | `PathResult` | `paths[]`, `count`, `reason` (when empty) |
| `list` | `ListResult` | `components[]`, `count` |

### metadata

Graph-level information included with every response:

| Field | Type | Description |
|-------|------|-------------|
| `execution_time_ms` | int | Query execution time in milliseconds |
| `node_count` | int | Total nodes in the graph |
| `edge_count` | int | Total edges in the graph |
| `component_count` | int | Total components in the graph |
| `graph_name` | string | Name of the queried graph |
| `graph_version` | string | Graph export version |
| `created_at` | string | Graph creation timestamp |
| `cycles_detected` | array | Cycles found during traversal (impact/dependencies only) |

---

## Result Types

### ImpactNode

Returned in `affected_nodes` for `impact` and `dependencies` queries.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Component identifier |
| `type` | string | Component type (e.g., `service`, `database`, `unknown`) |
| `distance` | int | Hop count from the queried component |
| `confidence_tier` | string | Human-readable confidence classification |
| `mentions` | array | Detection provenance (only with `--include-provenance`) |
| `mention_count` | int | Total mention count (only with `--include-provenance`) |

### EnrichedRelationship

Returned in `relationships` for `impact` and `dependencies` queries, and describes each traversed edge.

| Field | Type | Description |
|-------|------|-------------|
| `from` | string | Source component |
| `to` | string | Target component |
| `confidence` | float | Confidence score (0.4--1.0) |
| `confidence_tier` | string | Human-readable tier |
| `type` | string | Relationship type (e.g., `depends-on`, `references`, `calls`) |
| `source_file` | string | File where the relationship was detected |
| `extraction_method` | string | Detection algorithm |
| `source_type` | string | Detection source: `markdown`, `code`, or `both` |

### PathInfo

Returned in `paths[]` for `path` queries.

| Field | Type | Description |
|-------|------|-------------|
| `nodes` | string[] | Ordered list of component names from source to target |
| `hops` | HopInfo[] | Details for each edge in the path |
| `total_confidence` | float | Product of all hop confidences |

### HopInfo

Each hop within a path.

| Field | Type | Description |
|-------|------|-------------|
| `from` | string | Source component for this hop |
| `to` | string | Target component for this hop |
| `confidence` | float | Edge confidence |
| `confidence_tier` | string | Human-readable tier |
| `source_file` | string | Detection source file |
| `extraction_method` | string | Detection algorithm |

### ListComponent

Returned in `components[]` for `list` queries.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Component identifier |
| `type` | string | Component type |
| `incoming_edges` | int | Number of edges pointing to this component |
| `outgoing_edges` | int | Number of edges originating from this component |

### MentionDetail

Detection provenance for a component (with `--include-provenance`).

| Field | Type | Description |
|-------|------|-------------|
| `file_path` | string | File where the component was detected |
| `detection_method` | string | Algorithm that found it |
| `confidence` | float | Detection confidence |
| `context` | string | Heading hierarchy context (optional) |

---

## Confidence Tiers

Every relationship and node in query results includes a `confidence_tier` field mapping the numeric score to a human-readable classification:

| Tier | Score Range | Meaning |
|------|------------|---------|
| `explicit` | >= 0.95 | From service manifests, config files, or explicit declarations |
| `strong-inference` | >= 0.75 | Code analysis, structural patterns, or multi-algorithm agreement |
| `moderate` | >= 0.55 | Validated by two or more discovery algorithms |
| `weak` | >= 0.45 | Single algorithm with lower confidence |
| `semantic` | >= 0.42 | NLP/LLM similarity or speculative analysis |
| `threshold` | >= 0.40 | Minimum acceptable confidence |

---

## Cycle Detection

Impact and dependencies queries automatically detect cycles in the dependency graph. When cycles are found among the traversed nodes, they appear in `metadata.cycles_detected`:

```json
{
  "metadata": {
    "cycles_detected": [
      { "from": "service-a", "to": "service-b" },
      { "from": "service-b", "to": "service-a" }
    ]
  }
}
```

Each entry represents a back-edge that closes a cycle. Cycles are real-world patterns (e.g., mutual dependencies) and are reported rather than suppressed.

---

## Named Graphs

semanticmesh supports multiple named graphs stored under `$XDG_DATA_HOME/semanticmesh/graphs/`. Each graph is an independent snapshot created by `semanticmesh import`.

- **Default graph:** When `--graph` is omitted, the most recently imported graph is used (tracked by a `current` marker file).
- **Explicit selection:** Use `--graph <name>` to query a specific graph.

```bash
# Import with explicit name
semanticmesh import my-infra-v2.zip --name prod-2026

# Query that specific graph
semanticmesh query impact --component primary-db --graph prod-2026
```

---

## Error Responses

Errors are returned as structured JSON on stdout with a non-zero exit code.

### NOT_FOUND — Component does not exist

```json
{
  "error": "component \"payment-apii\" not found",
  "code": "NOT_FOUND",
  "suggestions": ["payment-api", "payment-service"]
}
```

Suggestions are fuzzy-matched component names (up to 5) ranked by similarity. Matching uses substring overlap, prefix matching, and word-level comparison.

### NO_GRAPH — No graph imported

```json
{
  "error": "no graph imported — run 'semanticmesh import <file.zip>' first",
  "code": "NO_GRAPH",
  "action": "run 'semanticmesh import <file.zip>' to import a graph first"
}
```

### MISSING_ARG — Required flag missing

```json
{
  "error": "--component is required",
  "code": "MISSING_ARG"
}
```

### INVALID_ARG — Invalid flag value

```json
{
  "error": "invalid --source-type \"unknown\": must be markdown, code, or both",
  "code": "INVALID_ARG"
}
```

---

## Table Output

All commands support `--format table` for human-readable output.

### Impact / Dependencies table

```
NAME              TYPE       DISTANCE  CONFIDENCE  TIER
payment-api       service    1         0.85        strong-inference
web-frontend      service    2         0.65        moderate
```

### Path table

```
Path 1 (confidence: 0.7820):
  FROM              TO              CONFIDENCE  TIER
  web-frontend      payment-api     0.85        strong-inference
  payment-api       primary-db      0.92        explicit
```

### List table

```
NAME              TYPE       INCOMING  OUTGOING
auth-service      service    3         2
payment-api       service    1         4
primary-db        database   5         0
```

---

## Extraction Methods

The `extraction_method` field on relationships indicates which algorithm detected the edge:

| Method | Description |
|--------|-------------|
| `explicit-link` | Markdown hyperlink between documents |
| `co-occurrence` | Co-occurrence analysis in document text |
| `structural` | Structural patterns (headings, lists, tables) |
| `NER` | Named Entity Recognition and SVO extraction |
| `semantic` | TF-IDF vector similarity |
| `LLM` | LLM-based semantic relationship inference |
| `code-analysis` | Source code import/call/connection detection |

---

**Last updated:** 2026-04-03
