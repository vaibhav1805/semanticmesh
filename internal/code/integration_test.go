package code_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticmesh/semanticmesh/internal/code"
	"github.com/semanticmesh/semanticmesh/internal/code/goparser"
	"github.com/semanticmesh/semanticmesh/internal/code/jsparser"
	"github.com/semanticmesh/semanticmesh/internal/code/pyparser"
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

func TestRunCodeAnalysisPython(t *testing.T) {
	tmpDir := t.TempDir()

	// pyproject.toml
	pyproject := `[project]
name = "my-python-svc"
version = "1.0.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}

	// Python file with HTTP call
	pyFile := `import requests

def call_payment():
    requests.get("http://payment-api:8080/pay")
`
	if err := os.WriteFile(filepath.Join(tmpDir, "client.py"), []byte(pyFile), 0644); err != nil {
		t.Fatal(err)
	}

	signals, err := code.RunCodeAnalysis(tmpDir, pyparser.NewPythonParser())
	if err != nil {
		t.Fatalf("RunCodeAnalysis failed: %v", err)
	}

	if len(signals) < 1 {
		t.Fatalf("expected at least 1 signal, got %d", len(signals))
	}

	found := false
	for _, s := range signals {
		if s.Language == "python" && s.DetectionKind == "http_call" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected python http_call signal, got: %+v", signals)
	}
}

func TestRunCodeAnalysisJavaScript(t *testing.T) {
	tmpDir := t.TempDir()

	// package.json
	pkgJSON := `{"name": "my-js-svc", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// JS file with HTTP call
	jsFile := `import axios from 'axios';

async function getOrders() {
    await axios.get("http://order-api:3000/orders");
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "client.js"), []byte(jsFile), 0644); err != nil {
		t.Fatal(err)
	}

	signals, err := code.RunCodeAnalysis(tmpDir, jsparser.NewJSParser())
	if err != nil {
		t.Fatalf("RunCodeAnalysis failed: %v", err)
	}

	if len(signals) < 1 {
		t.Fatalf("expected at least 1 signal, got %d", len(signals))
	}

	found := false
	for _, s := range signals {
		if s.Language == "javascript" && s.DetectionKind == "http_call" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected javascript http_call signal, got: %+v", signals)
	}
}

func TestRunCodeAnalysisMultiLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	// go.mod
	goMod := `module github.com/example/multi

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Go file
	goFile := `package main

import "net/http"

func main() {
	http.Get("http://go-api/data")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goFile), 0644); err != nil {
		t.Fatal(err)
	}

	// Python file
	pyFile := `import requests

requests.post("http://py-api:5000/submit")
`
	if err := os.WriteFile(filepath.Join(tmpDir, "app.py"), []byte(pyFile), 0644); err != nil {
		t.Fatal(err)
	}

	// JS file
	jsFile := `import axios from 'axios';

axios.get("http://js-api:3000/items");
`
	if err := os.WriteFile(filepath.Join(tmpDir, "index.js"), []byte(jsFile), 0644); err != nil {
		t.Fatal(err)
	}

	signals, err := code.RunCodeAnalysis(tmpDir,
		goparser.NewGoParser(),
		pyparser.NewPythonParser(),
		jsparser.NewJSParser(),
	)
	if err != nil {
		t.Fatalf("RunCodeAnalysis failed: %v", err)
	}

	// Check that we have signals from all three languages
	langs := make(map[string]bool)
	for _, s := range signals {
		langs[s.Language] = true
	}

	for _, lang := range []string{"go", "python", "javascript"} {
		if !langs[lang] {
			t.Errorf("expected signals from %s language, got languages: %v", lang, langs)
		}
	}
}

func TestInferSourceComponentPython(t *testing.T) {
	tmpDir := t.TempDir()

	pyproject := `[project]
name = "my-py-app"
version = "0.1.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		t.Fatal(err)
	}

	source := code.InferSourceComponent(tmpDir)
	if source != "my-py-app" {
		t.Errorf("expected 'my-py-app', got %q", source)
	}
}

func TestInferSourceComponentJS(t *testing.T) {
	tmpDir := t.TempDir()

	pkgJSON := `{"name": "my-js-app", "version": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	source := code.InferSourceComponent(tmpDir)
	if source != "my-js-app" {
		t.Errorf("expected 'my-js-app', got %q", source)
	}
}

func TestBoostKnownComponents(t *testing.T) {
	// Test that RunCodeAnalysis boosts comment_hint signals referencing known components.
	tmpDir := t.TempDir()

	goMod := `module github.com/example/boost-test

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// main.go: HTTP call to payment-api (code-detected) + comment hints
	mainGo := `package main

import "net/http"

func main() {
	// Calls payment-api
	http.Get("http://payment-api:8080/pay")
	// Depends on unknown-svc
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatal(err)
	}

	signals, err := code.RunCodeAnalysis(tmpDir, goparser.NewGoParser())
	if err != nil {
		t.Fatalf("RunCodeAnalysis failed: %v", err)
	}

	// Find comment_hint signals
	var knownHint, unknownHint *code.CodeSignal
	for i, s := range signals {
		if s.DetectionKind == "comment_hint" && s.TargetComponent == "payment-api" {
			knownHint = &signals[i]
		}
		if s.DetectionKind == "comment_hint" && s.TargetComponent == "unknown-svc" {
			unknownHint = &signals[i]
		}
	}

	// payment-api is also detected by http_call, so comment_hint should be boosted to 0.5
	if knownHint == nil {
		t.Fatal("missing comment_hint signal for payment-api")
	}
	if knownHint.Confidence != 0.5 {
		t.Errorf("known component comment_hint confidence = %f, want 0.5", knownHint.Confidence)
	}

	// unknown-svc is only in comments, should stay at 0.4
	if unknownHint == nil {
		t.Fatal("missing comment_hint signal for unknown-svc")
	}
	if unknownHint.Confidence != 0.4 {
		t.Errorf("unknown component comment_hint confidence = %f, want 0.4", unknownHint.Confidence)
	}
}
