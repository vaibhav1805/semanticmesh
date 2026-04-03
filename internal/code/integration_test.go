package code_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/graphmd/graphmd/internal/code"
	"github.com/graphmd/graphmd/internal/code/goparser"
)

func TestRunCodeAnalysis(t *testing.T) {
	// Create a temp directory with Go source fixtures
	tmpDir := t.TempDir()

	// go.mod file
	goMod := `module github.com/example/myservice

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// main.go with HTTP call, DB connection, and comment hint
	mainGo := `package main

import (
	"database/sql"
	"net/http"
)

func main() {
	http.Get("http://payment-api/pay")
	sql.Open("postgres", "postgres://mydb:5432/app")
	// Depends on cache-cluster
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatal(err)
	}

	// handler_test.go with sql.Open calls (should be skipped)
	testGo := `package main

import "database/sql"

func TestHandler(t *testing.T) {
	sql.Open("postgres", "postgres://testdb:5432/test")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "handler_test.go"), []byte(testGo), 0644); err != nil {
		t.Fatal(err)
	}

	// Run the analysis
	signals, err := code.RunCodeAnalysis(tmpDir, goparser.NewGoParser())
	if err != nil {
		t.Fatalf("RunCodeAnalysis failed: %v", err)
	}

	// Assert: 3 signals returned (http_call, db_connection, comment_hint)
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d: %+v", len(signals), signals)
	}

	// Build a map by detection_kind for easier assertions
	byKind := make(map[string]code.CodeSignal)
	for _, s := range signals {
		byKind[s.DetectionKind] = s
	}

	// Assert: http_call signal
	httpSig, ok := byKind["http_call"]
	if !ok {
		t.Fatal("missing http_call signal")
	}
	if httpSig.TargetComponent != "payment-api" {
		t.Errorf("http_call target: expected 'payment-api', got %q", httpSig.TargetComponent)
	}

	// Assert: db_connection signal
	dbSig, ok := byKind["db_connection"]
	if !ok {
		t.Fatal("missing db_connection signal")
	}
	if dbSig.TargetComponent != "mydb" {
		t.Errorf("db_connection target: expected 'mydb', got %q", dbSig.TargetComponent)
	}

	// Assert: comment_hint signal
	commentSig, ok := byKind["comment_hint"]
	if !ok {
		t.Fatal("missing comment_hint signal")
	}
	if commentSig.TargetComponent != "cache-cluster" {
		t.Errorf("comment_hint target: expected 'cache-cluster', got %q", commentSig.TargetComponent)
	}

	// Assert: all signals have SourceFile set to main.go (relative)
	for _, s := range signals {
		if s.SourceFile != "main.go" {
			t.Errorf("expected SourceFile 'main.go', got %q (kind: %s)", s.SourceFile, s.DetectionKind)
		}
	}
}

func TestRunCodeAnalysisSourceComponent(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module github.com/example/myservice

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	source := code.InferSourceComponent(tmpDir)
	if source != "github.com/example/myservice" {
		t.Errorf("expected source component 'github.com/example/myservice', got %q", source)
	}
}

func TestPrintCodeSignalsSummary(t *testing.T) {
	signals := []code.CodeSignal{
		{DetectionKind: "http_call", TargetComponent: "api"},
		{DetectionKind: "http_call", TargetComponent: "auth"},
		{DetectionKind: "db_connection", TargetComponent: "db"},
		{DetectionKind: "comment_hint", TargetComponent: "cache"},
	}

	var buf bytes.Buffer
	code.PrintCodeSignalsSummary(&buf, signals)

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output")
	}

	expected := "1 comment hint, 1 db connection, 2 http call"
	if !bytes.Contains(buf.Bytes(), []byte(expected)) {
		t.Errorf("expected output to contain %q, got: %s", expected, output)
	}
}

func TestPrintCodeSignalsSummaryEmpty(t *testing.T) {
	var buf bytes.Buffer
	code.PrintCodeSignalsSummary(&buf, nil)

	output := buf.String()
	if output == "" {
		t.Fatal("expected non-empty output for zero signals")
	}
}
