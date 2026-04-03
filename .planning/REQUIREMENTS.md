# Requirements: graphmd

**Defined:** 2026-03-16
**Core Value:** AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph instead of being fed entire architecture via prompts.

## v1 Requirements

Dependency graph export/import pipeline with accurate relationship inference from markdown.

### Component Model

- [ ] **COMP-01**: Define 12-type component taxonomy (service, database, cache, queue, message-broker, load-balancer, gateway, storage, container-registry, config-server, monitoring, log-aggregator)
- [ ] **COMP-02**: Persist component type in graph structure and SQLite schema
- [ ] **COMP-03**: Support querying by component type

### Relationship Accuracy & Consistency

- [x] **REL-01**: Implement relationship confidence tiers (7-tier system: 1.0 explicit link → 0.4 semantic similarity)
- [x] **REL-02**: Add metadata provenance to SQLite schema (source file, extraction method, last-modified timestamp)
- [x] **REL-03**: Handle circular dependencies without infinite loops (visited set tracking in traversal)
- [x] **REL-04**: Avoid transitive closure misinterpretation (return direct edges only in impact queries)
- [x] **REL-05**: Make pageindex a hard dependency for relationship location tracking and deduplication

### Extract & Export Pipeline

- [x] **EXTRACT-01**: Crawl markdown files recursively with component extraction
- [x] **EXTRACT-02**: Infer relationships from markdown references (service names, not just explicit links)
- [x] **EXTRACT-03**: Apply multiple discovery algorithms (co-occurrence, structural, NER, semantic) with signal aggregation
- [x] **EXPORT-01**: Implement `export` command: build graph → create SQLite database with indexed schema → package as ZIP
- [x] **EXPORT-02**: ZIP package includes: database file, metadata file (graph version, extraction timestamp, component count)

### Import & Query Pipeline (Production)

- [ ] **IMPORT-01**: Implement `import` command: unzip → load SQLite → validate schema
- [ ] **IMPORT-02**: CLI query interface for four core patterns:
  - Impact analysis: "if component X fails, what breaks?" (transitive closure, excludes cycles)
  - Dependencies: "what does component X depend on?" (direct edges only)
  - Path: "what's the connection between X and Y?" (shortest path)
  - List: "list all components of type T"
- [ ] **IMPORT-03**: Return results with confidence scores and metadata (provenance)

### Local Exploration

- [ ] **CRAWL-01**: Implement `crawl` command for local graph exploration (build in-memory graph, display stats)
- [ ] **CRAWL-02**: Display relationship confidence distribution and summary statistics

## v2 Requirements

Deferred features for future releases. Not in roadmap.

### Code Analysis Integration

- **CODE-01**: Code-based dependency inference (service imports, database connections in code)
- **CODE-02**: Axon-style codebase analysis integration
- **CODE-03**: Merge code signals with markdown signals (confidence-weighted)

### Advanced Features

- **FEAT-01**: Feature flag extraction from markdown/code
- **FEAT-02**: Environment-specific graphs (dev/staging/prod)
- **FEAT-03**: Connection string detection and parsing (high-value, low-effort for v2)
- **MCP-01**: MCP server wrapper around query interface (for LLM agent integration)

## Out of Scope

Explicitly excluded from this project. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Terraform/CloudFormation/K8s manifest parsing | v2+ complexity; markdown-only is sufficient for v1 |
| Docker Compose parsing | Same as IaC; defer to v2 |
| Human-facing visualization | Optimized for AI agents, not humans; visualization adds scope |
| Real-time sync or incremental updates | Batch export/import model is sufficient; real-time is v2+ |
| Access control & multi-tenancy | Single-user container model; not needed for v1 |
| Natural language query interface | LLM agent is the NLP layer; don't build another |
| Observability platform integration (Datadog, New Relic, etc.) | Out of scope; use standard SQLite queries |
| Bayesian signal fusion | 7-tier confidence system is sufficient; Bayesian is future |
| Dependency version tracking | Not applicable (components, not packages) |

## Traceability

Requirement mapping to roadmap phases. Updated during roadmap creation.

| Requirement | Phase | Phase Name | Status | Success Criteria |
|-------------|-------|------------|--------|-----------------|
| COMP-01 | 1 | Component Model | Not started | Type persistence, 12 types defined |
| COMP-02 | 1 | Component Model | Not started | Schema includes component_type column |
| COMP-03 | 1 | Component Model | Not started | list --type filter works |
| REL-01 | 2 | Accuracy Foundation | Complete | 7-tier confidence on all edges |
| REL-02 | 2 | Accuracy Foundation | Complete | Provenance fields non-null |
| REL-03 | 2 | Accuracy Foundation | Complete | Cyclic graph traversal completes |
| REL-04 | 2 | Accuracy Foundation | Complete | Impact returns direct edges only |
| REL-05 | 2 | Accuracy Foundation | Complete | Pageindex deduplication working |
| EXTRACT-01 | 3 | Extract & Export Pipeline | Not started | Recursive crawl + component extraction |
| EXTRACT-02 | 3 | Extract & Export Pipeline | Not started | Service name inference from mentions |
| EXTRACT-03 | 3 | Extract & Export Pipeline | Not started | Multi-algorithm signals aggregated |
| EXPORT-01 | 3 | Extract & Export Pipeline | Not started | Export produces valid ZIP |
| EXPORT-02 | 3 | Extract & Export Pipeline | Not started | metadata.json in ZIP |
| IMPORT-01 | 4 | Import & Query Pipeline | Not started | Import + schema validation |
| IMPORT-02 | 4 | Import & Query Pipeline | Not started | 4 query patterns via CLI |
| IMPORT-03 | 4 | Import & Query Pipeline | Not started | Confidence + provenance in results |
| CRAWL-01 | 5 | Crawl Exploration | Not started | Stats display on crawl |
| CRAWL-02 | 5 | Crawl Exploration | Not started | Confidence distribution shown |

**Coverage:**
- v1 requirements: 18 total
- Mapped to phases: 18
- Unmapped: 0

**Phase dependencies:**
- Phase 1 -> Phase 2 -> Phase 3 -> Phase 4
- Phase 3 -> Phase 5 (parallel with Phase 4)

---
*Requirements defined: 2026-03-16*
*Last updated: 2026-03-16 after roadmap creation*
