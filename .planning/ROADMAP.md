# Roadmap: graphmd

## Milestones

- ✅ **v1.0 graphmd MVP** — Phases 1-5 (shipped 2026-03-28)
- ✅ **v1.1 Tech Debt Cleanup** — Phases 6-8 (shipped 2026-03-29)
- 🚧 **v2.0 Code Analysis + MCP** — Phases 9-13 (in progress)

## Phases

<details>
<summary>✅ v1.0 graphmd MVP (Phases 1-5) — SHIPPED 2026-03-28</summary>

- [x] Phase 1: Component Model (3 plans) — 12-type taxonomy, schema persistence, type queries
- [x] Phase 2: Accuracy Foundation (5 plans) — confidence tiers, provenance, cycle safety, pageindex
- [x] Phase 3: Extract & Export Pipeline (2 plans) — .graphmdignore, aliases, ZIP export
- [x] Phase 4: Import & Query Pipeline (3 plans) — XDG import, 4 query patterns, JSON envelope
- [x] Phase 5: Crawl Exploration (2 plans) — stats engine, text/JSON formatters, quality warnings

**18 requirements delivered. Full details: `.planning/milestones/v1.0-ROADMAP.md`**

</details>

<details>
<summary>✅ v1.1 Tech Debt Cleanup (Phases 6-8) — SHIPPED 2026-03-29</summary>

- [x] Phase 6: Dead Code Removal (1 plan) — removed ~696 lines orphaned query code
- [x] Phase 7: Silent Loss Reporting (2 plans) — cycle detection in queries, edge drop warnings
- [x] Phase 8: Provenance Access (1 plan) — --include-provenance with volume control

**4 requirements delivered. Full details: `.planning/milestones/v1.1-ROADMAP.md`**

</details>

### 🚧 v2.0 Code Analysis + MCP (In Progress)

**Milestone Goal:** Add code-based dependency detection (Go, Python, JS/TS) and an MCP server so LLM agents can query graphs directly via tool use.

- [ ] **Phase 9: Code Analysis Foundation** - CodeSignal type, LanguageParser interface, Go parser with import/HTTP/DB/queue/cache detection
- [x] **Phase 10: Python + JS/TS Parsers** - Python and JavaScript/TypeScript parsers validating the LanguageParser interface generalizes (completed 2026-03-31)
- [x] **Phase 11: Connection Strings + Comment Analysis** - Cross-language connection string parsing and code comment dependency extraction (completed 2026-04-01)
- [x] **Phase 12: Signal Integration** - Schema v6 migration, code as 5th discovery source, confidence-weighted signal merging with provenance (completed 2026-04-02)
- [ ] **Phase 13: MCP Server** - MCP server with stdio transport wrapping query interface as 5 tools for LLM agent access

## Phase Details

### Phase 9: Code Analysis Foundation
**Goal**: Operators can analyze Go source code to detect infrastructure dependencies (HTTP calls, DB connections, queue/cache clients) with the same precision as markdown analysis
**Depends on**: Phase 8 (v1.1 complete)
**Requirements**: CODE-01
**Success Criteria** (what must be TRUE):
  1. Running code analysis on a Go project produces CodeSignal output identifying HTTP client calls, database connections, message queue producers/consumers, and cache client usage
  2. Go import paths resolve correctly against go.mod module declarations
  3. The LanguageParser interface is defined and the Go parser implements it cleanly (no Go-specific leaks in the interface)
  4. Detection precision is above 90% on a test corpus of real Go code (few false positives)
**Plans:** 2 plans
Plans:
- [ ] 09-01-PLAN.md — CodeSignal type, LanguageParser interface, Go parser with TDD
- [ ] 09-02-PLAN.md — CLI integration (--analyze-code flag on export, crawl, index)

### Phase 10: Python + JS/TS Parsers
**Goal**: Operators can analyze Python and JavaScript/TypeScript source code to detect the same categories of infrastructure dependencies as Go
**Depends on**: Phase 9
**Requirements**: CODE-02, CODE-03
**Success Criteria** (what must be TRUE):
  1. Running code analysis on a Python project detects imports, HTTP calls (requests/httpx), DB connections (SQLAlchemy/psycopg), queue clients (kafka/pika/boto3 SQS), and cache clients (redis/memcache)
  2. Running code analysis on a JS/TS project detects imports, HTTP calls (fetch/axios), DB connections (pg/mysql2/mongoose), queue clients, and cache clients
  3. Both parsers implement the same LanguageParser interface as the Go parser without interface changes
  4. False positives from regex-based detection are controlled via context filtering (comments, test files excluded) and confidence scoring below 0.5 for import-only detections
**Plans:** 3/3 plans complete
Plans:
- [ ] 10-01-PLAN.md — Python parser with TDD (HTTP, DB, cache, queue detection)
- [ ] 10-02-PLAN.md — JS/TS parser with TDD (HTTP, DB, cache, queue detection)
- [ ] 10-03-PLAN.md — Integration (register parsers, extend InferSourceComponent, extend skipDirs)

### Phase 11: Connection Strings + Comment Analysis
**Goal**: Code analysis extracts dependency targets from connection strings, URLs, DSNs, and code comments across all supported languages
**Depends on**: Phase 10
**Requirements**: CODE-04, CODE-05
**Success Criteria** (what must be TRUE):
  1. Connection strings in URL format (postgres://host/db, redis://host:6379), DSN format, and environment variable references are detected and parsed into component names
  2. Code comments containing dependency hints (e.g., "// Calls user-service API", docstrings referencing infrastructure) produce CodeSignal output
  3. Connection string detection works across Go, Python, and JS/TS source files using shared parsing utilities
**Plans:** 3/3 plans complete
Plans:
- [ ] 11-01-PLAN.md — Connection string parser (shared connstring package with TDD)
- [ ] 11-02-PLAN.md — Comment analyzer (shared comments package with TDD)
- [ ] 11-03-PLAN.md — Parser integration (replace per-parser duplication, add boost pass)

### Phase 12: Signal Integration
**Goal**: Code-detected dependencies merge with markdown-detected dependencies into a single graph with per-source provenance preserved
**Depends on**: Phase 11
**Requirements**: SIG-01, SIG-02
**Success Criteria** (what must be TRUE):
  1. Running export on a directory containing both markdown docs and source code produces a single graph with edges from both sources
  2. Each edge in the exported graph carries source_type metadata indicating whether it was detected from markdown, code, or both
  3. When both markdown and code detect the same relationship, confidence is boosted using probabilistic OR (two independent 0.6 signals produce ~0.84, not 0.6 or 0.6 average)
  4. Schema v6 migration succeeds on existing v5 graphs without data loss (ALTER TABLE adds source_type with DEFAULT 'markdown')
  5. The code_signals provenance table records file path, line number, language, and evidence for every code-detected signal
**Plans:** 2/2 plans complete
Plans:
- [ ] 12-01-PLAN.md — Schema v6 migration, Edge.SourceType, signal conversion with TDD
- [ ] 12-02-PLAN.md — Pipeline integration (export/crawl), --source-type query filter

### Phase 13: MCP Server
**Goal**: LLM agents can query graphmd dependency graphs via MCP tool use instead of CLI invocation
**Depends on**: Phase 12
**Requirements**: MCP-01
**Success Criteria** (what must be TRUE):
  1. Running `graphmd mcp` starts an MCP server on stdio transport that responds to tool calls
  2. Five MCP tools are available: query_impact, query_dependencies, query_path, list_components, and graphmd_graph_info
  3. MCP tool responses contain the same data as equivalent CLI queries (JSON envelope with query, results, metadata)
  4. No stdout pollution — all logging goes to stderr, only MCP protocol messages on stdout
  5. Server shuts down gracefully on SIGTERM without corrupting state
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 9 -> 10 -> 11 -> 12 -> 13

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Component Model | v1.0 | 3 | Complete | 2026-03-16 |
| 2. Accuracy Foundation | v1.0 | 5 | Complete | 2026-03-19 |
| 3. Extract & Export | v1.0 | 2 | Complete | 2026-03-19 |
| 4. Import & Query | v1.0 | 3 | Complete | 2026-03-23 |
| 5. Crawl Exploration | v1.0 | 2 | Complete | 2026-03-24 |
| 6. Dead Code Removal | v1.1 | 1 | Complete | 2026-03-29 |
| 7. Silent Loss Reporting | v1.1 | 2 | Complete | 2026-03-29 |
| 8. Provenance Access | v1.1 | 1 | Complete | 2026-03-29 |
| 9. Code Analysis Foundation | v2.0 | 2 | Planning complete | - |
| 10. Python + JS/TS Parsers | 3/3 | Complete    | 2026-03-31 | - |
| 11. Connection Strings + Comment Analysis | 3/3 | Complete    | 2026-04-01 | - |
| 12. Signal Integration | 2/2 | Complete   | 2026-04-02 | - |
| 13. MCP Server | v2.0 | TBD | Not started | - |

---
*Created: 2026-03-16*
*Last updated: 2026-03-29 (v2.0 roadmap created)*
