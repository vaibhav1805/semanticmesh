# Claude Code Guidance for semanticmesh

**Last updated:** 2026-03-16

## Project Context

`semanticmesh` is a dependency graph analyzer for infrastructure documentation.

**Core value:** AI agents can answer "if this fails, what breaks?" by querying a pre-computed dependency graph instead of being fed entire architecture via prompts.

**Target users:** AI agents (not humans), though operators configure and deploy the system.

---

## Code Style & Patterns

### Component Detection & Type System

- Component types are defined as Go constants in `internal/knowledge/types.go`
- Detection pipeline returns `(name, type, confidence, detection_methods)`
- Confidence scores range [0.4, 1.0] — never NULL
- Unknown/ambiguous components default to `type: unknown` gracefully

### SQLite Schema Conventions

- `component_type TEXT NOT NULL DEFAULT 'unknown'` on components table
- Separate `component_mentions` table for provenance (file path, detection method, confidence)
- All foreign keys properly constrained
- Migrations tested against existing test data

### CLI Commands

- JSON output preferred (parseable by agents)
- Include match metadata: `{name, type, confidence, match_type: "primary"|"tag"}`
- Support flags that control behavior: `--include-tags`, `--output json|table`
- Error messages clear and actionable (avoid internal jargon)

---

## Testing

- Unit tests in `*_test.go` alongside implementation
- Test data in `test-data/` — real-world, diverse examples
- Integration tests verify end-to-end: detect → export → query
- Coverage target: >85% for core packages

---

## File Organization

```
semanticmesh/
├── README.md                   # Public overview, use cases
├── CLAUDE.md                   # This file — project guidance
├── docs/
│   ├── COMPONENT_TYPES.md      # Component type taxonomy & guide
│   ├── CLI_REFERENCE.md        # CLI command documentation
│   ├── ADR_*.md                # Architecture decision records
│   └── CONFIGURATION.md        # Configuration & seed config guide
├── internal/
│   └── knowledge/
│       ├── types.go            # Component type constants
│       ├── components.go       # Detection pipeline
│       └── ...
└── cmd/
    └── semanticmesh/                # CLI entry point
```

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-03-16 | Component types + confidence scores | Enable AI agents to assess detection reliability |
| 2026-03-16 | Seed config for type customization | Allow users to extend taxonomy without code changes |
| 2026-03-16 | Provenance tracking (component_mentions table) | Support "where was this component found?" queries |

---

## Contact & Questions

For user feature guidance, see `docs/` — public documentation.

---

*Created: 2026-03-16*
