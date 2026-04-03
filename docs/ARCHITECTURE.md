# Architecture

semanticmesh builds a dependency graph from infrastructure documentation and source code, then exposes it for querying by AI agents. This document describes the system architecture, data flow, and key abstractions.

## Design Goal

AI agents need to answer questions like "if this fails, what breaks?" without being fed entire architecture documentation via prompts. semanticmesh pre-computes a dependency graph from markdown docs and source code, stores it in SQLite, and exposes it through a CLI and MCP server interface.

## Pipeline Overview

The core pipeline transforms raw documentation and source code into a queryable dependency graph:

```
  Markdown files     Source code files
       |                    |
       v                    v
    [Scan]           [Code Analysis]
       |                    |
       v                    v
   [Detect]          [Signal Detection]
       |                    |
       v                    v
  [Discover]                |
       |                    |
       +--------+-----------+
                |
                v
         [Signal Merge]
                |
                v
           [Export/Save]
                |
                v
           [Import/Load]
                |
                v
             [Query]
```

### Scan

The scanner (`internal/knowledge/scanner.go`) walks a directory tree collecting markdown files. Each file becomes a `Document` with an ID (relative path), title (first H1 heading or filename), and raw content. The scanner respects `.semanticmeshignore` patterns for excluding files.

### Detect (Link Extraction)

The extractor (`internal/knowledge/extractor.go`) parses each document for explicit relationships:

- **Markdown links** — `[text](target.md)` produces `references` edges at confidence 1.0
- **Prose mentions** — patterns like "depends on", "requires", "integrates with" produce `depends-on` or `mentions` edges at confidence 0.7
- **Code blocks** — import/call patterns produce `calls` edges at confidence 0.9

### Discover (Implicit Relationships)

The discovery layer (`internal/knowledge/discovery.go`, `discovery_orchestration.go`) finds relationships not explicitly linked:

- **Co-occurrence** — documents that frequently mention the same entities (`cooccurrence.go`)
- **Structural** — heading patterns and document organization (`structural.go`)
- **NER + SVO** — named entity recognition and subject-verb-object extraction (`ner.go`, `svo.go`)
- **Semantic** — BM25/TF-IDF similarity between documents (`semantic.go`)
- **LLM** — optional LLM-based discovery for deeper reasoning (`llm_discovery.go`)

Discovery produces `DiscoveredEdge` values with `Signal` metadata tracking which algorithms contributed. A quality filter (`DiscoveryFilterConfig`) gates edges by confidence and signal count to reduce false positives.

### Code Analysis

The code analysis subsystem (`internal/code/`) detects infrastructure dependencies directly from source code:

- **Analyzer** (`analyzer.go`) — orchestrates file dispatch based on extension
- **Go parser** (`goparser/`) — detects HTTP clients, database drivers, cache clients, queue producers/consumers in Go code
- **Python parser** (`pyparser/`) — detects similar patterns in Python (requests, SQLAlchemy, redis-py, etc.)
- **JS parser** (`jsparser/`) — detects patterns in JavaScript/TypeScript (fetch, pg, ioredis, etc.)
- **Connection string parser** (`connstring/`) — extracts hostnames and types from URLs and DSNs
- **Comment extraction** (`comments/`) — parses `@semanticmesh` hint comments from source

Each parser produces `CodeSignal` values identifying the target component, type, detection kind, and confidence.

### Signal Merge

When both markdown analysis and code analysis detect the same relationship, the merge layer (`internal/knowledge/signal_convert.go`) combines them:

1. Code signals are converted to `DiscoveredEdge` values grouped by (source, target, type)
2. Edges from both sources are merged via `MergeDiscoveredEdges` — same source+target+type edges aggregate their signals
3. **Probabilistic OR** boosts confidence for dual-source edges: `merged = 1.0 - (1.0 - mdConf) * (1.0 - codeConf)`
4. Each edge receives a `source_type` label: `"markdown"`, `"code"`, or `"both"`

### Export

The export command (`internal/knowledge/export.go`) packages the graph into a portable ZIP archive containing:

- `knowledge.db` — the SQLite database
- `metadata.json` — version, component/relationship counts, checksums

Export runs the full pipeline (scan, detect, discover, merge) and writes the result. It supports `--analyze-code` to include source code signals and `--git-version` for automatic version tagging.

### Import

The import command (`internal/knowledge/import.go`) loads an exported ZIP archive into the persistent graph registry (`~/.local/share/semanticmesh/graphs/`). Named graphs allow multiple projects to coexist. Both ZIP and legacy tar.gz formats are supported.

### Query

The query layer (`internal/knowledge/query.go`, `query_exec.go`, `query_cli.go`) provides four query types:

- **Impact** (`query impact`) — "if X fails, what breaks?" Depth-limited DFS with confidence filtering and traverse modes (direct, cascade, full)
- **Dependencies** (`query deps`) — "what does X depend on?" Reverse traversal of incoming edges
- **Path** (`query path`) — "how does X connect to Y?" Finds all simple paths between two components
- **List** (`query list`) — "what components exist?" Filterable by type, confidence, and source type

All queries return structured JSON (`QueryResult`) containing affected nodes with distance metrics, edges with full provenance, and execution metadata.

## Package Structure

```
semanticmesh/
├── cmd/semanticmesh/            CLI entry point — command dispatch
│   └── main.go             Parses subcommands, delegates to internal packages
│
├── internal/
│   ├── knowledge/          Core graph engine
│   │   ├── db.go           SQLite schema, migrations, SaveGraph/LoadGraph
│   │   ├── graph.go        Graph, Node structs, traversal algorithms (BFS, DFS, cycle detection)
│   │   ├── edge.go         Edge struct, EdgeType constants, confidence tiers, validation
│   │   ├── extractor.go    Link/mention/code extraction from documents
│   │   ├── discovery.go    Multi-algorithm relationship discovery and edge merging
│   │   ├── discovery_orchestration.go  Quality filtering and orchestration
│   │   ├── signal_convert.go  Code signal → graph edge conversion, probabilistic merge
│   │   ├── confidence.go   Confidence tier system (explicit → threshold)
│   │   ├── components.go   Component type detection pipeline
│   │   ├── types.go        ComponentType constants (12-type taxonomy)
│   │   ├── export.go       ZIP archive packaging (CmdExport)
│   │   ├── import.go       Graph registry and import (CmdImport)
│   │   ├── query.go        Query types (ImpactQuery, CrawlQuery)
│   │   ├── query_exec.go   Query execution logic
│   │   ├── query_cli.go    CLI argument parsing for query subcommands
│   │   ├── crawl.go        Multi-start graph traversal
│   │   ├── crawl_cmd.go    Crawl command implementation
│   │   ├── scanner.go      Directory walking, document collection
│   │   ├── bm25.go         BM25 scoring for semantic search
│   │   ├── aliases.go      Component name aliases
│   │   ├── semanticmeshignore.go  .semanticmeshignore pattern matching
│   │   └── ...
│   │
│   ├── code/               Source code analysis
│   │   ├── analyzer.go     CodeAnalyzer, LanguageParser interface, directory walking
│   │   ├── signal.go       CodeSignal struct
│   │   ├── integration.go  RunCodeAnalysis entry point
│   │   ├── goparser/       Go-specific parser (HTTP, DB, cache, queue patterns)
│   │   ├── pyparser/       Python-specific parser
│   │   ├── jsparser/       JavaScript/TypeScript-specific parser
│   │   ├── connstring/     URL/DSN connection string parsing
│   │   └── comments/       @semanticmesh hint comment extraction
│   │
│   └── mcp/                MCP server adapter
│       ├── server.go       MCP server creation, stdio transport, signal handling
│       └── tools.go        Tool registration (query_impact, query_dependencies, query_path, list_components, semanticmesh_graph_info)
│
├── docs/                   User-facing documentation
└── .planning/              Internal development workflow (not user-facing)
```

## Key Types

### Graph (`internal/knowledge/graph.go`)

The in-memory graph uses four maps for O(1) lookups:

- `Nodes map[string]*Node` — node ID to node
- `Edges map[string]*Edge` — edge ID to edge
- `BySource map[string][]*Edge` — source node ID to outgoing edges
- `ByTarget map[string][]*Edge` — target node ID to incoming edges

Graph provides traversal algorithms: `TraverseBFS`, `TraverseDFS` (cycle-aware), `TransitiveDeps`, `FindPaths`, `DetectCycles`, `GetSubgraph`, and `GetImpact`.

### Node (`internal/knowledge/graph.go`)

```go
type Node struct {
    ID            string        // relative path (forward slashes)
    Title         string        // display name from H1 or filename
    Type          string        // "document" or "infrastructure"
    ComponentType ComponentType // 12-type taxonomy classification
}
```

### Edge (`internal/knowledge/edge.go`)

```go
type Edge struct {
    ID               string    // deterministic: source\x00target\x00type
    Source            string    // source node ID
    Target            string    // target node ID
    Type              EdgeType  // references, depends-on, calls, implements, mentions, related
    Confidence        float64   // [0.0, 1.0]
    Evidence          string    // human-readable detection context
    RelationshipType  EdgeType  // direct-dependency or cyclic-dependency (set during traversal)
    SourceType        string    // "markdown", "code", or "both"
    // Provenance fields
    SourceFile        string    // file where relationship was detected
    ExtractionMethod  string    // algorithm that detected the edge
    DetectionEvidence string    // contextual snippet (~200 chars)
    EvidencePointer   string    // file:line reference
    LastModified      int64     // detection timestamp
}
```

### CodeSignal (`internal/code/signal.go`)

```go
type CodeSignal struct {
    SourceFile      string  // file where detected
    LineNumber      int     // line number
    TargetComponent string  // inferred dependency name
    TargetType      string  // service, database, cache, message-broker, queue, unknown
    DetectionKind   string  // http_call, db_connection, cache_client, queue_producer, etc.
    Evidence        string  // source line snippet (max 200 chars)
    Language        string  // go, python, javascript
    Confidence      float64 // [0.4, 1.0]
}
```

### QueryResult (`internal/knowledge/query_exec.go`)

Query results follow a uniform JSON envelope:

```json
{
  "query_type": "impact",
  "root": "payment-api",
  "affected_nodes": [...],
  "edges": [...],
  "metadata": {
    "execution_time_ms": 12,
    "nodes_visited": 5,
    "edges_traversed": 8
  }
}
```

## Confidence System

Every relationship carries a confidence score in [0.0, 1.0] reflecting detection certainty:

| Tier | Score Range | Meaning |
|------|-------------|---------|
| explicit | >= 0.95 | From manifests, config files, explicit declarations |
| strong-inference | >= 0.75 | Code analysis, structural patterns, multi-algorithm agreement |
| moderate | >= 0.55 | Two or more discovery algorithms with reasonable confidence |
| weak | >= 0.45 | Single algorithm, lower signal confidence |
| semantic | >= 0.42 | NLP/LLM similarity or speculative analysis |
| threshold | >= 0.40 | Minimum for inclusion in the graph |

Extraction method constants: `explicit-link`, `co-occurrence`, `structural`, `NER`, `semantic`, `LLM`, `code-analysis`.

## MCP Server

The MCP (Model Context Protocol) server (`internal/mcp/`) exposes semanticmesh as a tool provider for LLM agents via stdio transport. It registers five tools:

| Tool | Description |
|------|-------------|
| `query_impact` | Downstream impact analysis ("if X fails, what breaks?") |
| `query_dependencies` | Upstream dependency analysis ("what does X need?") |
| `query_path` | Path finding between components |
| `list_components` | Component listing with filters |
| `semanticmesh_graph_info` | Graph metadata and statistics |

The server uses the official `github.com/modelcontextprotocol/go-sdk/mcp` SDK and handles graceful shutdown via SIGTERM/SIGINT.

## Storage

All persistent data is stored in SQLite databases:

- **Project database** — `<project>/.bmd/knowledge.db` created by `semanticmesh index`
- **Graph registry** — `~/.local/share/semanticmesh/graphs/<name>/knowledge.db` managed by `semanticmesh import`

The database uses WAL mode for concurrent read access and enforces foreign key constraints. Schema migrations run automatically when opening older databases. See [SCHEMA.md](SCHEMA.md) for the complete schema reference.
