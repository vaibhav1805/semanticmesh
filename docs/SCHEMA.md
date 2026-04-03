# SQLite Schema Reference

semanticmesh stores its dependency graph and search index in a SQLite database (schema version 6). This document describes every table, column, index, and the migration system.

## Database Location

- **Project database:** `<project-root>/.bmd/knowledge.db` (created by `semanticmesh index`)
- **Graph registry:** `~/.local/share/semanticmesh/graphs/<name>/knowledge.db` (created by `semanticmesh import`)

## Database Configuration

The database is opened with these SQLite pragmas:

- `PRAGMA journal_mode=WAL` — Write-Ahead Logging for concurrent read access
- `PRAGMA foreign_keys=ON` — Foreign key constraints enforced (disabled by default in SQLite)

All multi-step writes are wrapped in transactions to guarantee atomicity.

## Tables

### graph_nodes

Vertices in the knowledge graph. Each node represents a document or infrastructure component.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | TEXT | PRIMARY KEY | Node identifier (relative path for documents, component name for infrastructure) |
| `type` | TEXT | NOT NULL | Node category: `"document"` (from markdown) or `"infrastructure"` (from code analysis) |
| `file` | TEXT | NOT NULL | Source file path |
| `title` | TEXT | | Display name (first H1 heading or filename stem) |
| `content` | TEXT | | Raw document content (optional) |
| `metadata` | TEXT | | JSON object with additional properties (e.g., `{"heading_level": 1, "line_range": [1,40]}`) |
| `component_type` | TEXT | NOT NULL DEFAULT 'unknown' | Classification from the 12-type taxonomy: service, database, cache, queue, api-gateway, load-balancer, cdn, storage, monitoring, ci-cd, auth, unknown |

**Indexes:**
- `idx_nodes_type` on `type` — filter by node category
- `idx_nodes_component_type` on `component_type` — filter by component classification
- `idx_nodes_title` on `title` — fast component name lookups

### graph_edges

Directed edges representing relationships between nodes. Each edge carries a confidence score and full provenance metadata.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | TEXT | PRIMARY KEY | Deterministic edge ID: `source\x00target\x00type` |
| `source_id` | TEXT | NOT NULL, FK → graph_nodes(id) ON DELETE CASCADE | Source node identifier |
| `target_id` | TEXT | NOT NULL, FK → graph_nodes(id) ON DELETE CASCADE | Target node identifier |
| `type` | TEXT | NOT NULL | Relationship type: `references`, `depends-on`, `calls`, `implements`, `mentions`, `related` |
| `confidence` | REAL | NOT NULL, CHECK (>= 0.0 AND <= 1.0) | Detection certainty score |
| `evidence` | TEXT | | Human-readable description of how the relationship was found |
| `source_file` | TEXT | | Relative path where the relationship was detected |
| `extraction_method` | TEXT | | Algorithm that detected the edge: `explicit-link`, `co-occurrence`, `structural`, `NER`, `semantic`, `LLM`, `code-analysis` |
| `detection_evidence` | TEXT | | Contextual snippet (~200 chars) explaining what was found |
| `evidence_pointer` | TEXT | | Precise location reference (e.g., `"service.yaml:42"`) |
| `last_modified` | INTEGER | | Unix timestamp of detection time or source file mtime |
| `source_type` | TEXT | NOT NULL DEFAULT 'markdown' | How the edge was discovered: `"markdown"`, `"code"`, or `"both"` |

**Indexes:**
- `idx_edges_source` on `source_id` — outgoing edge lookups
- `idx_edges_target` on `target_id` — incoming edge lookups
- `idx_edges_confidence` on `confidence` — confidence-based filtering

### component_mentions

Provenance tracking for component type detection. Records where each component was detected and by which method.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Auto-increment row ID |
| `component_id` | TEXT | NOT NULL, FK → graph_nodes(id) ON DELETE CASCADE | The component this mention belongs to |
| `file_path` | TEXT | NOT NULL | File where the component was detected |
| `heading_hierarchy` | TEXT | | Markdown heading context (e.g., `"## Services > ### Payment API"`) |
| `detected_by` | TEXT | NOT NULL | Detection method(s), comma-separated (e.g., `"title-keyword,filename-pattern"`) |
| `confidence` | REAL | NOT NULL, CHECK (>= 0.0 AND <= 1.0) | Detection confidence |

**Indexes:**
- `idx_mentions_component` on `component_id` — find all mentions of a component
- `idx_mentions_file` on `file_path` — find all components mentioned in a file

### code_signals

Raw provenance from source code analysis. Stores individual detection signals before they are merged into graph edges.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY AUTOINCREMENT | Auto-increment row ID |
| `source_component` | TEXT | NOT NULL | The component whose source code was analyzed |
| `target_component` | TEXT | NOT NULL | The detected dependency target |
| `signal_type` | TEXT | NOT NULL | Detection kind: `http_call`, `db_connection`, `cache_client`, `queue_producer`, `queue_consumer`, `comment_hint` |
| `confidence` | REAL | NOT NULL | Detection confidence in [0.4, 1.0] |
| `evidence` | TEXT | NOT NULL | Source line snippet that triggered detection (max 200 chars) |
| `file_path` | TEXT | NOT NULL | Source file path |
| `line_number` | INTEGER | NOT NULL | Line number in source file |
| `language` | TEXT | NOT NULL | Programming language: `go`, `python`, `javascript` |
| `created_at` | TEXT | NOT NULL DEFAULT (datetime('now')) | Insertion timestamp (ISO 8601) |

**Indexes:**
- `idx_code_signals_source` on `source_component` — find signals from a component
- `idx_code_signals_target` on `target_component` — find signals targeting a component

### documents

One row per markdown file in the indexed corpus. Used by the search index.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | TEXT | PRIMARY KEY | Relative file path (forward slashes) |
| `path` | TEXT | NOT NULL, UNIQUE | Absolute or relative file path |
| `title` | TEXT | | Document title |
| `content_hash` | TEXT | NOT NULL | MD5 hash for change detection |
| `last_modified` | INTEGER | NOT NULL | File modification time (Unix nanos) |
| `indexed_at` | INTEGER | NOT NULL | Indexing timestamp (Unix nanos) |

**Indexes:**
- `idx_documents_path` on `path`

### index_entries

Inverted index for BM25 text search. One row per (term, document/chunk) pair.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Auto-increment row ID |
| `term` | TEXT | NOT NULL | Indexed term (lowercased, stemmed) |
| `doc_id` | TEXT | NOT NULL, FK → documents(id) ON DELETE CASCADE | File-level document reference |
| `positions` | TEXT | | JSON array of positions (reserved for phrase search) |
| `frequency` | INTEGER | NOT NULL | Term frequency in this document/chunk |
| `chunk_id` | TEXT | | Full chunk ID (e.g., `"relPath#HeadingPath:L{startLine}"`) |
| `heading_path` | TEXT | | Heading hierarchy for chunk context |
| `start_line` | INTEGER | | Chunk start line in source document |
| `end_line` | INTEGER | | Chunk end line in source document |

**Indexes:**
- `idx_index_terms` on `term`
- `idx_index_docs` on `doc_id`

### bm25_stats

Corpus-level statistics for BM25 scoring.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `param` | TEXT | PRIMARY KEY | Parameter name: `"N"`, `"avg_doc_len"`, `"term_docs"`, `"k1"`, `"b"` |
| `value` | TEXT | NOT NULL | Parameter value (JSON for `term_docs`, numeric string for others) |

### metadata

Key-value store for database-level metadata.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `key` | TEXT | PRIMARY KEY | Metadata key |
| `value` | TEXT | NOT NULL | Metadata value |

Currently used keys:
- `schema_version` — current schema version number (e.g., `"6"`)

## Schema Versioning

The schema version is stored in the `metadata` table under the key `schema_version`. When `OpenDB()` is called, it runs `Initialize()` (idempotent table creation) followed by `Migrate()` (sequential version upgrades).

### Migration History

| Version | Changes |
|---------|---------|
| 1 | Initial schema: documents, index_entries, bm25_stats, graph_nodes, graph_edges, metadata |
| 2 | Added chunk metadata columns to index_entries: `chunk_id`, `heading_path`, `start_line`, `end_line` |
| 3 | Added `component_type` column to graph_nodes; created `component_mentions` table |
| 4 | Added provenance columns to graph_edges: `source_file`, `extraction_method`, `detection_evidence`, `evidence_pointer`, `last_modified` |
| 5 | Added indexes: `idx_nodes_title`, `idx_edges_confidence` |
| 6 | Added `source_type` column to graph_edges; created `code_signals` table |

Migrations are idempotent — `ALTER TABLE ADD COLUMN` errors for duplicate columns are silently ignored. This allows `Initialize()` + `Migrate()` to run safely on both fresh and existing databases.

## Example SQL Queries

### Find all dependencies of a component

```sql
SELECT e.target_id, e.type, e.confidence, e.source_type
FROM graph_edges e
WHERE e.source_id = 'payment-api'
ORDER BY e.confidence DESC;
```

### Find what depends on a component (reverse impact)

```sql
SELECT e.source_id, e.type, e.confidence, e.evidence
FROM graph_edges e
WHERE e.target_id = 'primary-db'
ORDER BY e.confidence DESC;
```

### List all components of a specific type

```sql
SELECT id, title, component_type
FROM graph_nodes
WHERE component_type = 'database';
```

### Find high-confidence code-detected relationships

```sql
SELECT source_id, target_id, type, confidence, extraction_method
FROM graph_edges
WHERE source_type IN ('code', 'both')
  AND confidence >= 0.8
ORDER BY confidence DESC;
```

### Get detection provenance for an edge

```sql
SELECT source_file, extraction_method, detection_evidence,
       evidence_pointer, last_modified, source_type
FROM graph_edges
WHERE source_id = 'web-frontend' AND target_id = 'api-gateway';
```

### Find where a component was detected (component mentions)

```sql
SELECT file_path, detected_by, confidence, heading_hierarchy
FROM component_mentions
WHERE component_id = 'redis-cache'
ORDER BY confidence DESC;
```

### Get raw code analysis signals for a target

```sql
SELECT source_component, signal_type, evidence,
       file_path, line_number, language, confidence
FROM code_signals
WHERE target_component = 'primary-db'
ORDER BY confidence DESC;
```

### Graph summary statistics

```sql
SELECT
  (SELECT COUNT(*) FROM graph_nodes) AS node_count,
  (SELECT COUNT(*) FROM graph_edges) AS edge_count,
  (SELECT COUNT(DISTINCT component_type) FROM graph_nodes) AS type_count,
  (SELECT AVG(confidence) FROM graph_edges) AS avg_confidence,
  (SELECT value FROM metadata WHERE key = 'schema_version') AS schema_version;
```

### Components with edges from multiple detection sources

```sql
SELECT source_id, target_id, confidence, source_type
FROM graph_edges
WHERE source_type = 'both'
ORDER BY confidence DESC;
```
