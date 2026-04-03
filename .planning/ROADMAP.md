# Roadmap: graphmd v1

**Created:** 2026-03-16
**Depth:** Standard (5 phases)
**Target user:** AI agents (not humans)
**Core value:** AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph.

## Phase Overview

| Phase | Name | Requirements | Focus |
|-------|------|-------------|-------|
| 1 | Component Model | COMP-01, COMP-02, COMP-03 | Define types, persist to schema, enable type queries |
| 2 | Accuracy Foundation | REL-01, REL-02, REL-03, REL-04, REL-05 | Confidence tiers, provenance, cycle safety, pageindex |
| 3 | Extract & Export Pipeline | EXTRACT-01, EXTRACT-02, EXTRACT-03, EXPORT-01, EXPORT-02 | Crawl + discover + build SQLite + ZIP |
| 4 | Import & Query Pipeline | IMPORT-01, IMPORT-02, IMPORT-03 | Load ZIP, four query patterns, confidence in results |
| 5 | Crawl Exploration | CRAWL-01, CRAWL-02 | Local graph exploration, stats display |

---

## Phase 1: Component Model

**Goal:** Refine the component model so every graph node carries a typed classification that AI agents can filter and query.

**Brownfield context:** Component detection exists (`components.go`, `components_discovery.go`, `registry.go`) but types are not persisted -- all nodes are type "document". This phase wires detection to persistence.

### Requirements

| ID | Requirement | Notes |
|----|------------|-------|
| COMP-01 | Define 12-type component taxonomy (service, database, cache, queue, message-broker, load-balancer, gateway, storage, container-registry, config-server, monitoring, log-aggregator) | Taxonomy based on Backstage/Cartography patterns |
| COMP-02 | Persist component type in graph structure and SQLite schema | Add `component_type` column to nodes table |
| COMP-03 | Support querying by component type | `list --type database` returns all database components |

### Success Criteria

1. **Type persistence:** After export, `SELECT DISTINCT component_type FROM graph_nodes` returns at least 3 distinct types from a test corpus containing services, databases, and caches.
2. **Schema validation:** SQLite schema includes `component_type TEXT NOT NULL DEFAULT 'unknown'` on the graph_nodes table.
3. **Type query:** CLI command `graphmd list --type service` returns only service-typed components (zero false positives from other types).
4. **Coverage:** All 12 taxonomy types are defined as constants in code; unknown/unclassified components default to `unknown`.

### Plans

| # | Plan | Status |
|---|------|--------|
| 1 | Component Type System Definition & Persistence | Complete (2026-03-16) |
| 2 | User-Facing Documentation & Guide | Complete (2026-03-19) |
| 3 | Quality Assurance & Validation | Complete (2026-03-19) |

---

## Phase 2: Accuracy Foundation

**Goal:** Establish the confidence and provenance infrastructure that prevents false edges from misleading AI agents during incident response.

**Brownfield context:** Discovery algorithms exist (co-occurrence, structural, NER, semantic, LLM) with signal aggregation (`algo_aggregator.go`). Pageindex exists (`pageindex.go`). This phase adds confidence tiers, provenance metadata, and cycle-safe traversal.

**Hard dependency:** REL-05 (pageindex) must be completed first -- it provides location tracking and deduplication that other REL requirements depend on.

### Requirements

| ID | Requirement | Notes |
|----|------------|-------|
| REL-01 | Implement relationship confidence tiers (7-tier: 1.0 explicit link to 0.4 semantic similarity) | Replaces raw scores with interpretable tiers |
| REL-02 | Add metadata provenance to SQLite schema (source file, extraction method, last-modified timestamp) | Every edge must be traceable |
| REL-03 | Handle circular dependencies without infinite loops (visited set tracking) | Safety constraint for all traversal |
| REL-04 | Avoid transitive closure misinterpretation (direct edges only in impact queries) | Prevent false impact reporting |
| REL-05 | Make pageindex a hard dependency for relationship location tracking and deduplication | Must complete first in this phase |

### Success Criteria

1. **Confidence tiers:** Every relationship in the exported SQLite has a `confidence` value mapping to one of 7 defined tiers; no edge has confidence outside [0.4, 1.0].
2. **Provenance:** Every relationship row includes non-null `source_file`, `extraction_method`, and `last_modified` fields; an agent can query `SELECT source_file FROM relationships WHERE source='payment-api'` and get a file path.
3. **Cycle safety:** A test graph with A->B->C->A cycles completes impact analysis for node A without hanging, returning results within 1 second.
4. **Direct edges:** Impact query for a node returns only direct dependents by default (not transitive closure); transitive traversal is opt-in with depth parameter.
5. **Pageindex integration:** Duplicate relationships from different discovery algorithms are deduplicated using pageindex location data; a relationship appearing in 3 algorithms produces 1 edge with aggregated confidence.

### Plans

| # | Plan | Status |
|---|------|--------|
| 1 | Pageindex Integration & Deduplication (REL-05 hard dependency) | Complete (2026-03-19) |
| 2 | Confidence Tier System (REL-01) | Complete (2026-03-19) |
| 3 | Edge Provenance Schema & Persistence (REL-02) | Complete (2026-03-19) |
| 4 | Cycle Detection & Traversal (REL-03, REL-04) | Complete (2026-03-19) |
| 5 | Query Interface: Crawl vs Impact (REL-04) | Complete (2026-03-19) |

---

## Phase 3: Extract & Export Pipeline

**Goal:** Complete the end-to-end pipeline from markdown scanning through SQLite packaging as a portable ZIP archive.

**Brownfield context:** Scanner, extractor, relationship engine, and export logic exist (`export.go`, `command_helpers.go`). The `cmdExport` in `main.go` is a stub not wired to `CmdExport`. Discovery algorithms exist but are not all integrated into the main flow. This phase wires everything together and produces the ZIP artifact.

### Requirements

| ID | Requirement | Notes |
|----|------------|-------|
| EXTRACT-01 | Crawl markdown files recursively with component extraction | Scanner exists; wire to component model |
| EXTRACT-02 | Infer relationships from markdown references (service names, not just explicit links) | Mention extraction exists; integrate with confidence tiers |
| EXTRACT-03 | Apply multiple discovery algorithms with signal aggregation | Algorithms exist; wire aggregator to main pipeline |
| EXPORT-01 | Build graph, create SQLite database with indexed schema, package as ZIP | Wire cmdExport stub to CmdExport |
| EXPORT-02 | ZIP includes database file + metadata (version, timestamp, component count) | Metadata file alongside SQLite in archive |

### Success Criteria

1. **End-to-end:** Running `graphmd export --input ./docs --output graph.zip` on a test corpus produces a valid ZIP containing `graph.db` and `metadata.json`.
2. **Component extraction:** The exported database contains components extracted from markdown headings, service mentions, and NER -- not just document titles.
3. **Multi-algorithm discovery:** Relationships in the export reflect signals from at least 2 discovery algorithms (verified by `extraction_method` provenance field).
4. **Metadata completeness:** `metadata.json` in ZIP contains `version`, `created_at` (ISO 8601), `component_count`, and `relationship_count` fields, all non-null.

**Plans:** 2 plans

### Plans

| # | Plan | Status |
|---|------|--------|
| 1 | .graphmdignore, aliasing, and schema v5 indexes | Not started |
| 2 | Full export pipeline wiring + ZIP packaging + integration test | Not started |

---

## Phase 4: Import & Query Pipeline

**Goal:** Enable production containers to load an exported graph and serve the four core query patterns that AI agents need for incident response.

**Brownfield context:** `importtar.go` implements `ImportKnowledgeTar`/`LoadFromKnowledgeTar` but no CLI command exists. No structured query interface exists -- this is the highest-value unfinished work.

### Requirements

| ID | Requirement | Notes |
|----|------------|-------|
| IMPORT-01 | Implement `import` command: unzip, load SQLite, validate schema | Wire existing import logic to CLI |
| IMPORT-02 | CLI query interface for four core patterns: impact, dependencies, path, list | The product for AI agents |
| IMPORT-03 | Return results with confidence scores and metadata (provenance) | Agents must assess trustworthiness |

### Success Criteria

1. **Import round-trip:** `graphmd export` followed by `graphmd import` on the output ZIP succeeds with schema validation passing and all components/relationships accessible.
2. **Impact query:** `graphmd query impact --component payment-api` returns downstream dependents with confidence scores, completing in under 2 seconds for graphs with 500+ components.
3. **Path query:** `graphmd query path --from payment-api --to primary-db` returns the shortest path with per-hop confidence scores.
4. **Machine-readable output:** All query results are valid JSON (parseable by `jq .`) with consistent schema: `{components: [...], relationships: [...], metadata: {...}}`.
5. **Provenance in results:** Each relationship in query output includes `source_file`, `extraction_method`, and `confidence` fields.

### Plans

1. Implement `import` CLI command wrapping existing ImportKnowledgeTar
2. Build query router: impact / dependencies / path / list subcommands
3. Implement impact analysis with depth-limited transitive traversal and cycle detection
4. Implement shortest-path query between two components
5. Add JSON output formatter with consistent schema for all query types

---

## Phase 5: Crawl Exploration

**Goal:** Provide a local exploration mode for engineers to inspect the graph before export, displaying statistics and relationship quality metrics.

**Brownfield context:** Crawl command exists (`crawl.go`) with graph traversal. This phase adds confidence distribution display and summary statistics so users can assess graph quality before exporting.

### Requirements

| ID | Requirement | Notes |
|----|------------|-------|
| CRAWL-01 | Implement `crawl` command for local graph exploration (build in-memory graph, display stats) | Extend existing crawl with stats |
| CRAWL-02 | Display relationship confidence distribution and summary statistics | Quality assessment before export |

### Success Criteria

1. **Stats output:** `graphmd crawl --input ./docs` displays component count, relationship count, and type distribution (e.g., "services: 12, databases: 4, caches: 2").
2. **Confidence distribution:** Output includes confidence tier breakdown (e.g., "explicit: 15, strong: 23, moderate: 8, weak: 3") so engineers can assess graph quality.
3. **Fast feedback:** Crawl completes on a 100-document corpus in under 10 seconds.

### Plans

1. Extend crawl command to build in-memory graph with component types
2. Add confidence distribution calculator and display
3. Add summary statistics output (counts by type, relationship quality metrics)

---

## Requirements Coverage Matrix

| Requirement | Phase | Description |
|-------------|-------|-------------|
| COMP-01 | 1 | 12-type component taxonomy |
| COMP-02 | 1 | Persist type in schema |
| COMP-03 | 1 | Query by type |
| REL-01 | 2 | 7-tier confidence |
| REL-02 | 2 | Provenance metadata |
| REL-03 | 2 | Cycle-safe traversal |
| REL-04 | 2 | Direct edges only |
| REL-05 | 2 | Pageindex hard dependency |
| EXTRACT-01 | 3 | Recursive crawl + extraction |
| EXTRACT-02 | 3 | Service name inference |
| EXTRACT-03 | 3 | Multi-algorithm discovery |
| EXPORT-01 | 3 | SQLite + ZIP export |
| EXPORT-02 | 3 | Metadata in ZIP |
| IMPORT-01 | 4 | Import command |
| IMPORT-02 | 4 | Four query patterns |
| IMPORT-03 | 4 | Confidence + provenance in results |
| CRAWL-01 | 5 | Local exploration |
| CRAWL-02 | 5 | Confidence distribution |

**Total v1 requirements: 18**
**Mapped to phases: 18**
**Unmapped: 0**

---

## Dependency Graph (Phase Order)

```
Phase 1 (Component Model)
    |
    v
Phase 2 (Accuracy Foundation)  -- REL-05/pageindex is hard dependency
    |
    v
Phase 3 (Extract & Export)     -- depends on types + confidence from P1/P2
    |
    v
Phase 4 (Import & Query)      -- depends on export artifact from P3
    |
    v
Phase 5 (Crawl Exploration)   -- depends on types + confidence; can parallelize with P4
```

Phases 4 and 5 can be worked in parallel once Phase 3 is complete. Phase 5 has no dependency on Phase 4.

---
*Created: 2026-03-16*
*Last updated: 2026-03-19 (Phase 2 complete - all 5 plans done)*
