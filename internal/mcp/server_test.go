package mcp

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestInitializeResponse verifies that the MCP server responds to an initialize
// request even when stdin EOF arrives immediately (the pipe-close race condition).
// This simulates "echo '...' | semanticmesh mcp" where the pipe closes right after
// sending the message.
func TestInitializeResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create pipes for simulated stdin and stdout.
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()

	// Write a valid JSON-RPC initialize request, then close the writer
	// to simulate a closed pipe (like echo).
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	go func() {
		stdinWriter.Write([]byte(initReq))
		stdinWriter.Close() // EOF immediately, just like echo pipe
	}()

	// Wrap stdin in the same pipe-wrapper pattern used by Run() to hold
	// the transport open until context cancellation.
	pr, pw := io.Pipe()
	go func() {
		io.Copy(pw, stdinReader)
		<-ctx.Done()
		pw.Close()
	}()

	server := NewServer()

	// Run the server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Run(ctx, &mcpsdk.IOTransport{
			Reader: pr,
			Writer: nopWriteCloser{stdoutWriter},
		})
		stdoutWriter.Close()
	}()

	// Read the response from stdout.
	decoder := json.NewDecoder(stdoutReader)
	var response map[string]interface{}
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify it's a valid JSON-RPC response with id=1.
	if id, ok := response["id"]; !ok {
		t.Fatal("response missing 'id' field")
	} else if id != float64(1) {
		t.Fatalf("expected id=1, got %v", id)
	}

	// Verify result contains capabilities.
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing 'result' object, got: %v", response)
	}
	if _, ok := result["capabilities"]; !ok {
		t.Fatalf("result missing 'capabilities', got: %v", result)
	}

	// Verify serverInfo is present.
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing 'serverInfo', got: %v", result)
	}
	if name, _ := serverInfo["name"].(string); name != "semanticmesh" {
		t.Fatalf("expected serverInfo.name='semanticmesh', got '%s'", name)
	}

	// Clean shutdown: cancel context, wait for server.
	cancel()
	if err := <-serverErr; err != nil && err != context.Canceled {
		t.Logf("server returned (expected): %v", err)
	}
}
