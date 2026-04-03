# Requirements: graphmd

**Defined:** 2026-03-29
**Core Value:** AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph instead of being fed entire architecture via prompts.

## v2.0 Requirements

Code analysis integration and MCP server for LLM agent access.

### Code Analysis

- [x] **CODE-01**: Go language parser detecting imports, HTTP client calls, database connections, message queue producers/consumers, and cache client usage
- [x] **CODE-02**: Python language parser detecting imports, HTTP calls (requests/httpx), DB connections (SQLAlchemy/psycopg), queue clients (kafka/pika/boto3 SQS), and cache clients (redis/memcache)
- [x] **CODE-03**: JavaScript/TypeScript language parser detecting imports, HTTP calls (fetch/axios), DB connections (pg/mysql2/mongoose), queue clients, and cache clients
- [x] **CODE-04**: Connection string detection and parsing across all languages — URLs, DSNs, environment variable references, config file patterns
- [x] **CODE-05**: Code comment analysis extracting dependency hints and component references from inline comments and docstrings

### Signal Integration

- [x] **SIG-01**: Merge code-detected dependency signals with markdown-detected signals using confidence-weighted aggregation (code as 5th discovery source)
- [x] **SIG-02**: Schema v6 migration supporting multi-source provenance per relationship (which sources detected each edge)

### MCP Server

- [x] **MCP-01**: MCP server with stdio transport wrapping query interface — 5 tools: impact, dependencies, path, list, graph_info

## v2.1 Requirements

Deferred features for next release. Not in current roadmap.

### Code Flows

- **FLOW-01**: Function call chain extraction within components
- **FLOW-02**: Code path mapping from component relationships to specific source locations
- **FLOW-03**: Cross-service call tracing (HTTP handler → client → remote handler)

### Advanced MCP

- **MCP-02**: Streamable HTTP transport for MCP server
- **MCP-03**: Structured outputSchema for each MCP tool

## Out of Scope

| Feature | Reason |
|---------|--------|
| Terraform/CloudFormation/K8s manifest parsing | IaC is a different domain; code + markdown sufficient |
| Docker Compose parsing | Same as IaC |
| Full AST type resolution | Over-engineering for dependency detection; pattern matching sufficient |
| Runtime/dynamic analysis | Static analysis only for v2.0 |
| Real-time file watching | Batch analysis sufficient |
| CGo-based tree-sitter | Breaks pure-Go build; regex-first for Python/JS |
| Natural language query via MCP | LLM is the NLP layer; tools provide structured access |
| SSE/HTTP MCP transport | Stdio sufficient for v2.0; HTTP in v2.1 |

## Traceability

| Requirement | Phase | Phase Name | Status |
|-------------|-------|------------|--------|
| CODE-01 | 9 | Code Analysis Foundation | Pending |
| CODE-02 | 10 | Python + JS/TS Parsers | Pending |
| CODE-03 | 10 | Python + JS/TS Parsers | Pending |
| CODE-04 | 11 | Connection Strings + Comment Analysis | Complete |
| CODE-05 | 11 | Connection Strings + Comment Analysis | Complete |
| SIG-01 | 12 | Signal Integration | Pending |
| SIG-02 | 12 | Signal Integration | Pending |
| MCP-01 | 13 | MCP Server | Pending |

**Coverage:**
- v2.0 requirements: 8 total
- Mapped to phases: 8
- Unmapped: 0

**Phase dependencies:**
- Phase 9: None (first v2.0 phase; builds on v1.1 foundation)
- Phase 10: Phase 9 (LanguageParser interface defined there)
- Phase 11: Phase 10 (connection strings work across all 3 language parsers)
- Phase 12: Phase 11 (all code signals must exist before merging with markdown)
- Phase 13: Phase 12 (MCP queries enriched hybrid graph)

---
*Requirements defined: 2026-03-29*
*Last updated: 2026-03-29 after roadmap creation*
