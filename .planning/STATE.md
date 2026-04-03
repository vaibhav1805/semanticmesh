# Project State: graphmd v2.0

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-29)

**Core value:** AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph
**Current focus:** Phase 10 - Python/JS Parsers (next)

## Current Position

Phase: 9 of 13 (Code Analysis Foundation) — COMPLETE
Plan: 2 of 2 complete
Status: Phase complete
Last activity: 2026-03-30 — Phase 9 complete (CLI integration with --analyze-code flag)

Progress: [█████████████████░░░] 84% (21/~25 plans)

## Performance Metrics

**Velocity:**
- Total plans completed: 21 (v1.0: 15, v1.1: 4, v2.0: 2)
- Total execution time: see milestone records

**By Phase (v1.1 — most recent):**

| Phase | Plans | Status |
|-------|-------|--------|
| 6. Dead Code Removal | 1 | Complete |
| 7. Silent Loss Reporting | 2 | Complete |
| 8. Provenance Access | 1 | Complete |
| 9. Code Analysis Foundation | 2/2 | Complete |

## Accumulated Context

### Decisions

- v1.0 shipped 2026-03-28: 18 requirements, 5 phases, 15 plans
- v1.1 shipped 2026-03-29: 4 requirements, 3 phases, 4 plans
- Code flows (function call chains) deferred to v2.1
- Go parser uses stdlib go/ast (no CGo) — proves LanguageParser interface before Python/JS
- Python/JS use regex-first approach; tree-sitter deferred to v2.1 if needed
- Signal merging isolated in Phase 12 (highest risk)
- MCP server last (Phase 13) — queries enriched hybrid graph
- Schema v6: source_type column + code_signals table
- CGo-free build maintained throughout v2.0
- Version-aware import path resolution for Go modules (vN -> parent package name)
- Pattern table keyed by importPath.Function for O(1) lookup during AST walk
- Comment hints at 0.4 confidence to distinguish from code-level detection
- Variadic parser args on RunCodeAnalysis to avoid import cycle (code <-> goparser)
- Code signals printed to stderr as diagnostic output; graph integration deferred to Phase 12

### Pending Todos

None yet.

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-30
Stopped at: Completed 09-02-PLAN.md (CLI integration with --analyze-code flag) — Phase 9 complete
Resume file: None
