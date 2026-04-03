---
phase: 13-mcp-server
plan: 02
subsystem: api
tags: [mcp, go-sdk, stdio, json-rpc, tools, agent-interface]

# Dependency graph
requires:
  - phase: 13-mcp-server/01
    provides: "5 exported query execution functions and QueryError type"
provides:
  - "MCP server package (internal/mcp/) with 5 registered tools"
  - "graphmd mcp CLI command with stdio transport"
  - "Stdout guard and graceful SIGTERM shutdown"
affects: []

# Tech tracking
tech-stack:
  added: ["github.com/modelcontextprotocol/go-sdk v1.4.1"]
  patterns: ["MCP tool handlers as thin adapters calling knowledge.Execute* functions", "QueryError -> IsError tool result, infrastructure error -> Go error for SDK"]

key-files:
  created:
    - internal/mcp/server.go
    - internal/mcp/tools.go
    - internal/mcp/tools_test.go
  modified:
    - cmd/graphmd/main.go
    - go.mod
    - go.sum

key-decisions:
  - "Used ToolHandlerFor generic API with jsonschema struct tags for auto JSON Schema generation"
  - "QueryError returned as explicit CallToolResult with IsError=true and structured JSON; infrastructure errors returned as Go errors for SDK to wrap"
  - "jsonschema struct tags use plain description text (not key=value syntax) per SDK convention"
  - "Stdout guard redirects os.Stdout to os.Stderr during tool registration, restores before server.Run"

patterns-established:
  - "MCP handler pattern: convert args -> call knowledge.Execute* -> handleQueryError or marshalResult"
  - "queryErrorJSON struct for JSON serialization of QueryError (which lacks json tags)"

requirements-completed: [MCP-01]

# Metrics
duration: 5min
completed: 2026-04-03
---

# Phase 13 Plan 02: MCP Server Implementation Summary

**MCP server with 5 tools wrapping graphmd query API via official Go SDK, stdio transport, and graphmd mcp CLI command**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-03T01:02:50Z
- **Completed:** 2026-04-03T01:08:29Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Built MCP server package (internal/mcp/) with 5 registered tools: query_impact, query_dependencies, query_path, list_components, graphmd_graph_info
- Wired `graphmd mcp` CLI command that starts the MCP server on stdio transport with stdout guard and graceful SIGTERM/SIGINT shutdown
- All 7 unit tests pass covering validation errors, error code classification, and tool count verification

## Task Commits

Each task was committed atomically:

1. **Task 1: MCP server package with tool registrations** - `3ec383d` (feat)
2. **Task 2: Wire graphmd mcp CLI command** - `3b377c8` (feat)

## Files Created/Modified
- `internal/mcp/server.go` - NewServer, Run with stdout guard and signal handling
- `internal/mcp/tools.go` - 5 tool registrations with typed arg structs, handler functions, error handling
- `internal/mcp/tools_test.go` - 7 tests covering validation, error codes, and tool count
- `cmd/graphmd/main.go` - Added mcp command case and cmdMCP function
- `go.mod` - Added MCP Go SDK v1.4.1 dependency
- `go.sum` - Updated with transitive dependencies

## Decisions Made
- Used `ToolHandlerFor` generic API (not raw `ToolHandler`) for automatic input schema generation and validation
- QueryError is manually converted to structured JSON in tool results (explicit `CallToolResult` with `IsError: true`) while infrastructure errors are passed through as Go errors for the SDK to handle
- jsonschema struct tags follow SDK convention of plain description text, not `key=value` syntax -- `required` is inferred from absence of `omitempty` on json tag
- Stdout guard sequence: redirect -> create server + register tools -> restore -> Run (SDK captures os.Stdout at Connect time inside Run)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed jsonschema struct tag syntax**
- **Found during:** Task 1 (tool registration tests)
- **Issue:** Plan specified `jsonschema:"required,description=..."` format but SDK's google/jsonschema-go library requires plain description text without key=value prefixes
- **Fix:** Changed all struct tags to `jsonschema:"description text"` format; required fields indicated by non-omitempty json tags
- **Files modified:** internal/mcp/tools.go
- **Verification:** TestRegisterTools_Count passes, all 5 tools register without panic
- **Committed in:** 3ec383d (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for SDK compatibility. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- MCP server fully functional with all 5 tools
- Phase 13 (MCP Server) is complete -- this was the final plan
- Project v2.0 is now feature-complete

---
*Phase: 13-mcp-server*
*Completed: 2026-04-03*
