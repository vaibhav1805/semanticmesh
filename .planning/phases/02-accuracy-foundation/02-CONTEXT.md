# Phase 2: Accuracy Foundation - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish the confidence and provenance infrastructure that prevents false edges from misleading AI agents during incident response. Add confidence tiers to relationships, track provenance metadata (source file, detection method, evidence), implement cycle-safe traversal, and support deduplication via pageindex integration.

This phase is about **reliability**: agents must trust the graph data when investigating incidents.

</domain>

<decisions>
## Implementation Decisions

### Confidence Representation

- **Format:** Both raw scores [0.4–1.0] AND 7 named semantic tiers
- **Tiers (7-level scale):**
  - Explicit link (1.0) — from service manifests/configs
  - Strong inference (0.8) — from code analysis, structural patterns
  - Moderate (0.6) — validated by multiple algorithms
  - Weak (0.5) — single algorithm, lower signal
  - Semantic (0.45) — NLP/LLM similarity, less certain
  - Threshold (0.4) — minimum accepted confidence
  - (Below 0.4 excluded from export)

- **Aggregation:** When same relationship detected by multiple algorithms, use weighted average:
  - Algorithm weights: Co-occurrence=0.3x, NER=0.5x, Structural=0.6x, Semantic=0.7x, LLM=1.0x
  - Result: aggregated confidence reflects combined signal strength

- **Query interface:** Agents can filter by both:
  - `--min-confidence 0.7` (numeric threshold)
  - `--min-tier strong` (named tier for readability)

### Provenance Tracking

- **Tracked fields:**
  - `source_file` — Path to where relationship was detected (markdown doc, source file, API spec, manifest)
  - `detection_method` — How it was found (explicit-link, co-occurrence, structural, NER, semantic, LLM)
  - `detection_evidence` — Actual text/snippet that revealed the relationship (~200 chars contextual snippet)
  - `evidence_pointer` — Line number / byte offset for original lookup (optional, when applicable)

- **Evidence snippets:** Complete, standalone context so agents understand relationships without file access
  - Example: "calls primary-db for transaction storage"
  - Works across all source types: code, markdown docs, API specs, service manifests, runtime discovery

- **File path semantics:** `source_file` is audit metadata (where relationship originated), NOT a fetchable path in runtime containers. Evidence snippet is the agent-usable content.

- **Query patterns:**
  - **Crawl command:** Exploratory, returns all relationships with full confidence/provenance. Agents see everything, make judgment calls.
  - **Impact command:** Focused incident response, filtered by confidence threshold. Returns relationships agents likely need.
  - Both support JSON output with full subgraph topology (nodes + edges)

### Cycle Detection & Safety

- **Cycle handling:** Circular dependencies are valid real-world patterns (A→B→C→A where C calls back to A for response)

- **Representation:**
  - Detect cycles during traversal
  - Include cyclic edges in all query results
  - Mark edges explicitly as `relationship_type: cyclic-dependency`
  - Agents see mutual dependencies clearly

- **Safety mechanism:**
  - Use visited set tracking to prevent infinite loops during traversal
  - Guarantee query termination even with complex cycles

- **Agent visibility:** Cycles are implicit in results (marked as cyclic-dependency type), not a special query mode

- **Example:** Impact query on payment-api shows:
  ```
  primary-db (confidence: 1.0, type: cyclic-dependency)
    Note: Also calls back to payment-api for credential validation
  ```

### Edge Traversal Semantics & Query Results

- **Impact query default:** Return direct edges (distance=1) + one hop downstream (distance=2)
  - Balances completeness vs. information overload
  - Agents can request more with flags

- **Depth control:** Both syntaxes supported:
  - `--depth N` (numeric: 1=direct only, 2=direct+1hop, 3+=transitive)
  - `--traverse-mode {direct|cascade|full}` (named: intuitive for agents)

- **Result format:** All queries return **JSON with complete subgraph structure**
  ```json
  {
    "query": "impact",
    "root": "payment-api",
    "depth": 2,
    "affected_nodes": [
      {
        "name": "primary-db",
        "confidence": 1.0,
        "relationship_type": "direct-dependency",
        "distance": 1,
        "source_file": "service.yaml",
        "detection_method": "explicit-link"
      }
    ],
    "edges": [
      {
        "from": "payment-api",
        "to": "primary-db",
        "confidence": 1.0,
        "type": "direct-dependency",
        "evidence": "calls primary-db for transaction storage",
        "evidence_pointer": {"file": "service.yaml", "line": 42}
      }
    ]
  }
  ```

- **Structure benefits:**
  - Agents reconstruct full subgraph topology
  - Programmatic filtering/analysis (e.g., find all services >0.8 confidence)
  - Visualizable as graph
  - Queryable with jq or JSON parsers

- **Evidence display:** Evidence snippets embedded in edges so agents understand *why* the relationship exists without file access

</decisions>

<specifics>
## Specific Ideas

- Incident response use case drives all decisions: agents need to trust relationships during crisis (can't second-guess confidence, can't fetch files that don't exist in containers)
- Real-world cycles are common and important; treat as features, not errors
- Confidence is not just a number; semantic tier names (explicit, strong, moderate, weak) make it readable for human review of decisions
- JSON as first-class output format (agents parse and analyze, not just read)

</specifics>

<deferred>
## Deferred Ideas

- Scheduled confidence updates / confidence decay over time — Phase 3+ work (export pipeline)
- Custom confidence weights per algorithm — future configuration phase
- Visualization tools for subgraph exploration — Phase 5+ (Crawl Exploration)
- Confidence-based automatic filtering/recommendations — future quality improvement phase

</deferred>

---

*Phase: 02-accuracy-foundation*
*Context gathered: 2026-03-19*
