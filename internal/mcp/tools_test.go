package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestHandleImpact_MissingComponent verifies that an impact query with an
// empty component returns a tool result with IsError=true and MISSING_ARG code.
func TestHandleImpact_MissingComponent(t *testing.T) {
	result, _, err := handleImpact(context.Background(), nil, ImpactArgs{
		Component: "",
	})
	if err != nil {
		t.Fatalf("expected nil error for user error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing component")
	}
	assertErrorCode(t, result, "MISSING_ARG")
}

// TestHandleImpact_InvalidSourceType verifies that an invalid source_type
// returns INVALID_ARG.
func TestHandleImpact_InvalidSourceType(t *testing.T) {
	result, _, err := handleImpact(context.Background(), nil, ImpactArgs{
		Component:  "some-service",
		SourceType: "invalid-type",
	})
	if err != nil {
		t.Fatalf("expected nil error for user error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for invalid source type")
	}
	assertErrorCode(t, result, "INVALID_ARG")
}

// TestHandleDependencies_MissingComponent verifies that a dependencies query
// with an empty component returns MISSING_ARG.
func TestHandleDependencies_MissingComponent(t *testing.T) {
	result, _, err := handleDependencies(context.Background(), nil, DependenciesArgs{
		Component: "",
	})
	if err != nil {
		t.Fatalf("expected nil error for user error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing component")
	}
	assertErrorCode(t, result, "MISSING_ARG")
}

// TestHandlePath_MissingArgs verifies that a path query with empty from/to
// returns MISSING_ARG.
func TestHandlePath_MissingArgs(t *testing.T) {
	result, _, err := handlePath(context.Background(), nil, PathArgs{
		From: "",
		To:   "",
	})
	if err != nil {
		t.Fatalf("expected nil error for user error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for missing args")
	}
	assertErrorCode(t, result, "MISSING_ARG")
}

// TestHandleGraphInfo_NoGraph verifies that querying a non-existent graph
// returns an infrastructure error (no stored graph).
func TestHandleGraphInfo_NoGraph(t *testing.T) {
	result, _, err := handleGraphInfo(context.Background(), nil, GraphInfoArgs{
		Graph: "nonexistent-graph-12345",
	})
	// Infrastructure error: no graph loaded. This should come back as either
	// an error return (infrastructure) or an IsError result.
	if err != nil {
		// Infrastructure error returned as Go error — expected behavior.
		return
	}
	if result != nil && result.IsError {
		// Also acceptable: returned as tool error.
		return
	}
	t.Fatal("expected either an error return or IsError=true result for missing graph")
}

// TestHandleList_InvalidSourceType verifies that list with an invalid
// source_type returns INVALID_ARG.
func TestHandleList_InvalidSourceType(t *testing.T) {
	result, _, err := handleList(context.Background(), nil, ListArgs{
		SourceType: "bad-source",
	})
	if err != nil {
		t.Fatalf("expected nil error for user error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for invalid source type")
	}
	assertErrorCode(t, result, "INVALID_ARG")
}

// TestRegisterTools_Count verifies that exactly 5 tools are registered.
func TestRegisterTools_Count(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "graphmd-test",
		Version: "test",
	}, nil)
	registerTools(server)

	// Connect an in-memory client to list tools.
	ctx := context.Background()
	t1, t2 := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "test-client",
		Version: "test",
	}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	count := 0
	for _, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		count++
	}

	if count != 5 {
		t.Fatalf("expected 5 tools, got %d", count)
	}
}

// --- Test helpers ---

// assertErrorCode extracts the JSON error content from a CallToolResult and
// checks that it contains the expected error code.
func assertErrorCode(t *testing.T, result *mcpsdk.CallToolResult, expectedCode string) {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("expected content in error result")
	}
	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	var errObj queryErrorJSON
	if err := json.Unmarshal([]byte(tc.Text), &errObj); err != nil {
		t.Fatalf("failed to unmarshal error JSON %q: %v", tc.Text, err)
	}
	if errObj.Code != expectedCode {
		t.Fatalf("expected error code %q, got %q", expectedCode, errObj.Code)
	}
}
