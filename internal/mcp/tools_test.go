package mcp

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
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

// TestRegisterTools_Count verifies that exactly 6 tools are registered.
func TestRegisterTools_Count(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "semanticmesh-test",
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

	if count != 6 {
		t.Fatalf("expected 6 tools, got %d", count)
	}
}

// TestMarshalResult_SmallResponse verifies that small responses remain uncompressed.
func TestMarshalResult_SmallResponse(t *testing.T) {
	// Create a small payload (< 20 KB)
	data := map[string]any{
		"component": "test-service",
		"type":      "service",
		"data":      strings.Repeat("x", 100),
	}

	result, _, err := marshalResult(data)
	if err != nil {
		t.Fatalf("marshalResult failed: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-nil result with content")
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	// Verify the content is valid JSON (not compressed)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &decoded); err != nil {
		t.Fatalf("expected valid JSON, got: %v", err)
	}

	// Verify no compression metadata
	if tc.Meta != nil && tc.Meta["encoding"] != nil {
		t.Fatal("expected no compression metadata for small response")
	}
}

// TestMarshalResult_LargeResponse verifies that large responses are compressed.
func TestMarshalResult_LargeResponse(t *testing.T) {
	// Create a large payload (> 20 KB)
	largeData := make([]map[string]any, 100)
	for i := 0; i < 100; i++ {
		largeData[i] = map[string]any{
			"component":  "test-service-" + strings.Repeat("x", 100),
			"type":       "service",
			"confidence": 0.95,
			"metadata":   strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 20),
		}
	}

	data := map[string]any{
		"query":      "test-query",
		"components": largeData,
	}

	result, _, err := marshalResult(data)
	if err != nil {
		t.Fatalf("marshalResult failed: %v", err)
	}

	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected non-nil result with content")
	}

	tc, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	// Verify compression metadata exists
	if tc.Meta == nil || tc.Meta["encoding"] == nil {
		t.Fatal("expected compression metadata for large response")
	}

	if tc.Meta["encoding"] != "gzip+base64" {
		t.Fatalf("expected encoding=gzip+base64, got %v", tc.Meta["encoding"])
	}

	// Verify compression stats
	originalSize, ok := tc.Meta["original_size"].(int)
	if !ok || originalSize < compressionThreshold {
		t.Fatalf("expected original_size >= %d, got %v", compressionThreshold, originalSize)
	}

	compressedSize, ok := tc.Meta["compressed_size"].(int)
	if !ok || compressedSize >= originalSize {
		t.Fatalf("expected compressed_size < original_size, got compressed=%v, original=%v", compressedSize, originalSize)
	}

	compressionRatio, ok := tc.Meta["compression_ratio"].(float64)
	if !ok || compressionRatio <= 1.0 {
		t.Fatalf("expected compression_ratio > 1.0, got %v", compressionRatio)
	}

	// Verify the compressed data can be decompressed and decoded
	decoded, err := base64.StdEncoding.DecodeString(tc.Text)
	if err != nil {
		t.Fatalf("failed to base64 decode: %v", err)
	}

	gr, err := gzip.NewReader(strings.NewReader(string(decoded)))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	var decodedData map[string]any
	if err := json.Unmarshal(decompressed, &decodedData); err != nil {
		t.Fatalf("failed to unmarshal decompressed data: %v", err)
	}

	// Verify the decompressed data matches the original structure
	if decodedData["query"] != "test-query" {
		t.Fatal("decompressed data does not match original")
	}
}

// TestCompressGzip verifies the gzip compression helper.
func TestCompressGzip(t *testing.T) {
	input := []byte(strings.Repeat("test data ", 1000))

	compressed, err := compressGzip(input)
	if err != nil {
		t.Fatalf("compressGzip failed: %v", err)
	}

	if len(compressed) >= len(input) {
		t.Fatalf("expected compressed size < input size, got compressed=%d, input=%d",
			len(compressed), len(input))
	}

	// Verify the compressed data can be decompressed
	gr, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if string(decompressed) != string(input) {
		t.Fatal("decompressed data does not match original")
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
