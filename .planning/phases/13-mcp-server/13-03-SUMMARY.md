---
phase: 13-mcp-server
plan: 03
subsystem: mcp
tags: [mcp, jsonrpc2, stdio, io-pipe, race-condition]

# Dependency graph
requires:
  - phase: 13-mcp-server
    provides: MCP server with StdioTransport and tool handlers
provides:
  - IOTransport pipe wrapper fixing stdin EOF race condition
  - Integration test proving piped initialize works
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: [io.Pipe wrapper to decouple stdin EOF from transport EOF]

key-files:
  created:
    - internal/mcp/server_test.go
  modified:
    - internal/mcp/server.go

key-decisions:
  - "io.Pipe wrapper blocks on ctx.Done after stdin EOF, keeping transport open until graceful shutdown"
  - "nopWriteCloser avoids closing stdout when IOTransport calls Close"

patterns-established:
  - "Pipe wrapper pattern: copy stdin to pipe, block on ctx.Done, close pipe writer — decouples stdin lifecycle from transport lifecycle"

requirements-completed: [MCP-01]

# Metrics
duration: 2min
completed: 2026-04-03
---

# Phase 13 Plan 03: MCP Stdin EOF Race Fix Summary

**io.Pipe wrapper replacing StdioTransport to prevent jsonrpc2 from seeing stdin EOF before writing initialize response**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-03T04:19:43Z
- **Completed:** 2026-04-03T04:21:40Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Fixed stdin EOF race condition that prevented MCP server from responding to piped initialize requests
- Replaced StdioTransport with IOTransport using io.Pipe wrapper that holds stdin open until context cancellation
- Added integration test proving the fix works: initialize response is written even when stdin EOF arrives immediately

## Task Commits

Each task was committed atomically:

1. **Task 1: Replace StdioTransport with IOTransport pipe wrapper** - `b8052ea` (fix)

## Files Created/Modified
- `internal/mcp/server.go` - Replaced StdioTransport with IOTransport + pipe wrapper; added nopWriteCloser type
- `internal/mcp/server_test.go` - Integration test simulating echo-pipe scenario proving initialize response is written

## Decisions Made
- Used io.Pipe wrapper pattern (copy stdin, block on ctx.Done, then close) rather than a custom noEOFReader, because it cleanly decouples stdin lifecycle from transport lifecycle using standard library primitives
- Added nopWriteCloser to wrap os.Stdout since IOTransport requires io.WriteCloser but we must not close stdout

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- MCP server now works correctly with both interactive clients (Claude Desktop, Cursor) and piped input (echo, scripts)
- All 8 MCP tests pass (7 existing tool handler tests + 1 new integration test)
- UAT blocker resolved

## Self-Check: PASSED

- FOUND: internal/mcp/server.go
- FOUND: internal/mcp/server_test.go
- FOUND: 13-03-SUMMARY.md
- FOUND: commit b8052ea

---
*Phase: 13-mcp-server*
*Completed: 2026-04-03*
