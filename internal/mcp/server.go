package mcp

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// nopWriteCloser wraps an io.Writer with a no-op Close method.
// IOTransport requires io.WriteCloser but we do not want to close stdout.
type nopWriteCloser struct{ w io.Writer }

func (n nopWriteCloser) Write(p []byte) (int, error) { return n.w.Write(p) }
func (n nopWriteCloser) Close() error                { return nil }

// NewServer creates an MCP server with all graphmd tools registered.
func NewServer() *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "graphmd",
		Version: "2.0.0",
	}, nil)

	registerTools(server)
	return server
}

// Run starts the MCP server on stdio transport with signal handling.
// This is the main entry point called by the CLI's "mcp" command.
func Run() error {
	// Redirect Go's default logger to stderr to prevent stdout pollution.
	log.SetOutput(os.Stderr)

	// Guard stdout: redirect os.Stdout to os.Stderr during server setup
	// so any stray fmt.Println or library init code goes to stderr.
	origStdout := os.Stdout
	os.Stdout = os.Stderr

	// Create the server (tool registration happens here).
	server := NewServer()

	// Restore stdout for the SDK's StdioTransport, which captures
	// os.Stdout at Connect time (inside Run).
	os.Stdout = origStdout

	// Create a context that cancels on SIGTERM or SIGINT for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	fmt.Fprintf(os.Stderr, "graphmd MCP server starting on stdio...\n")

	// Wrap stdin in a pipe that stays open until context cancellation.
	// This prevents the SDK's jsonrpc2 layer from seeing EOF (and entering
	// shuttingDown state) before it finishes writing responses.
	// Without this, piped input (echo '...' | graphmd mcp) races: stdin
	// EOF arrives before the initialize response is written, causing it
	// to be silently dropped.
	pr, pw := io.Pipe()
	go func() {
		io.Copy(pw, os.Stdin)
		<-ctx.Done()
		pw.Close()
	}()

	return server.Run(ctx, &mcpsdk.IOTransport{
		Reader: pr,
		Writer: nopWriteCloser{origStdout},
	})
}
