package comments

import (
	"testing"

	"github.com/semanticmesh/semanticmesh/internal/code"
)

func TestAnalyze_GoSingleLineComments(t *testing.T) {
	lines := []string{
		"package main",
		"",
		"// Calls payment-api",
		"// Depends on auth-service",
		"// Uses redis-cache",
		"// Connects to primary-db",
		"func main() {}",
	}

	signals := Analyze(lines, SyntaxGo, nil)

	expected := []struct {
		target     string
		confidence float64
	}{
		{"payment-api", 0.4},
		{"auth-service", 0.4},
		{"redis-cache", 0.4},
		{"primary-db", 0.4},
	}

	if len(signals) != len(expected) {
		t.Fatalf("expected %d signals, got %d: %+v", len(expected), len(signals), signals)
	}

	for i, e := range expected {
		if signals[i].TargetComponent != e.target {
			t.Errorf("signal %d: expected target %q, got %q", i, e.target, signals[i].TargetComponent)
		}
		if signals[i].Confidence != e.confidence {
			t.Errorf("signal %d: expected confidence %.1f, got %.1f", i, e.confidence, signals[i].Confidence)
		}
		if signals[i].DetectionKind != "comment_hint" {
			t.Errorf("signal %d: expected kind comment_hint, got %q", i, signals[i].DetectionKind)
		}
	}
}

func TestAnalyze_GoExtraVerbs(t *testing.T) {
	lines := []string{
		"// Talks to notification-service",
		"// Sends to event-queue",
		"// Reads from data-store",
		"// Writes to audit-log",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 4 {
		t.Fatalf("expected 4 signals, got %d: %+v", len(signals), signals)
	}

	targets := []string{"notification-service", "event-queue", "data-store", "audit-log"}
	for i, target := range targets {
		if signals[i].TargetComponent != target {
			t.Errorf("signal %d: expected %q, got %q", i, target, signals[i].TargetComponent)
		}
	}
}

func TestAnalyze_PythonComments(t *testing.T) {
	lines := []string{
		"import os",
		"# Uses primary-db",
		"# Connects to cache-cluster",
		"def main():",
		"    pass",
	}

	signals := Analyze(lines, SyntaxPython, nil)

	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "primary-db" {
		t.Errorf("expected target primary-db, got %q", signals[0].TargetComponent)
	}
	if signals[1].TargetComponent != "cache-cluster" {
		t.Errorf("expected target cache-cluster, got %q", signals[1].TargetComponent)
	}
}

func TestAnalyze_JavaScriptComments(t *testing.T) {
	lines := []string{
		"const x = 1;",
		"// Talks to user-service",
		"console.log(x);",
	}

	signals := Analyze(lines, SyntaxJavaScript, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].TargetComponent != "user-service" {
		t.Errorf("expected target user-service, got %q", signals[0].TargetComponent)
	}
}

func TestAnalyze_CaseInsensitive(t *testing.T) {
	lines := []string{
		"// calls payment-api",
		"// DEPENDS ON auth-service",
		"// uses Redis",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 3 {
		t.Fatalf("expected 3 signals, got %d: %+v", len(signals), signals)
	}
}

func TestAnalyze_GoBlockComments(t *testing.T) {
	lines := []string{
		"package main",
		"/* This service",
		"   Calls payment-api",
		"   and depends on auth-service",
		"*/",
		"func main() {}",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals from block comment, got %d: %+v", len(signals), signals)
	}

	foundPayment := false
	foundAuth := false
	for _, s := range signals {
		if s.TargetComponent == "payment-api" {
			foundPayment = true
		}
		if s.TargetComponent == "auth-service" {
			foundAuth = true
		}
	}
	if !foundPayment {
		t.Error("missing signal for payment-api in block comment")
	}
	if !foundAuth {
		t.Error("missing signal for auth-service in block comment")
	}
}

func TestAnalyze_JSDocBlockComments(t *testing.T) {
	lines := []string{
		"/**",
		" * This function calls order-service",
		" * and uses cache-layer",
		" */",
		"function handler() {}",
	}

	signals := Analyze(lines, SyntaxJavaScript, nil)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals from JSDoc, got %d: %+v", len(signals), signals)
	}

	found := map[string]bool{}
	for _, s := range signals {
		found[s.TargetComponent] = true
	}
	if !found["order-service"] {
		t.Error("missing signal for order-service in JSDoc")
	}
	if !found["cache-layer"] {
		t.Error("missing signal for cache-layer in JSDoc")
	}
}

func TestAnalyze_PythonDocstrings(t *testing.T) {
	lines := []string{
		"def process():",
		`    """`,
		"    Process data. Calls data-pipeline",
		"    and depends on storage-service.",
		`    """`,
		"    pass",
	}

	signals := Analyze(lines, SyntaxPython, nil)
	if len(signals) < 2 {
		t.Fatalf("expected at least 2 signals from docstring, got %d: %+v", len(signals), signals)
	}

	found := map[string]bool{}
	for _, s := range signals {
		found[s.TargetComponent] = true
	}
	if !found["data-pipeline"] {
		t.Error("missing signal for data-pipeline in docstring")
	}
	if !found["storage-service"] {
		t.Error("missing signal for storage-service in docstring")
	}
}

func TestAnalyze_PythonSingleQuoteDocstrings(t *testing.T) {
	lines := []string{
		"def process():",
		"    '''",
		"    Uses cache-service for speed.",
		"    '''",
		"    pass",
	}

	signals := Analyze(lines, SyntaxPython, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal from single-quote docstring, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "cache-service" {
		t.Errorf("expected cache-service, got %q", signals[0].TargetComponent)
	}
}

func TestAnalyze_TODOWithComponentName(t *testing.T) {
	lines := []string{
		"// TODO: migrate to new-db-service",
		"# FIXME: redis-cache timeout issue",
		"// HACK: workaround for auth-api bug",
		"// XXX: queue-broker connection flaky",
	}

	signals := Analyze(lines, SyntaxGo, nil)

	// These should produce signals at 0.3 confidence for component-like names
	foundDB := false
	foundRedis := false
	foundAuth := false
	foundQueue := false
	for _, s := range signals {
		switch s.TargetComponent {
		case "new-db-service":
			foundDB = true
			if s.Confidence != 0.3 {
				t.Errorf("TODO signal should be 0.3, got %.1f", s.Confidence)
			}
		case "redis-cache":
			foundRedis = true
		case "auth-api":
			foundAuth = true
		case "queue-broker":
			foundQueue = true
		}
	}

	// Go syntax uses // comments; # is not a Go comment so FIXME with # won't match
	if !foundDB {
		t.Error("missing TODO signal for new-db-service")
	}
	if !foundAuth {
		t.Error("missing HACK signal for auth-api")
	}
	if !foundQueue {
		t.Error("missing XXX signal for queue-broker")
	}
	// redis-cache is in a # comment, which Go syntax doesn't recognize
	if foundRedis {
		t.Error("should NOT find redis-cache in # comment when using Go syntax")
	}
}

func TestAnalyze_TODOPythonSyntax(t *testing.T) {
	lines := []string{
		"# FIXME: redis-cache timeout issue",
	}

	signals := Analyze(lines, SyntaxPython, nil)
	found := false
	for _, s := range signals {
		if s.TargetComponent == "redis-cache" {
			found = true
			if s.Confidence != 0.3 {
				t.Errorf("expected confidence 0.3, got %.1f", s.Confidence)
			}
		}
	}
	if !found {
		t.Error("missing TODO signal for redis-cache in Python comment")
	}
}

func TestAnalyze_TODOWithoutComponentSuffix(t *testing.T) {
	// TODO without component-like name should NOT produce a signal
	lines := []string{
		"// TODO: fix this later",
		"// FIXME: handle error",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 0 {
		t.Errorf("expected no signals for generic TODOs, got %d: %+v", len(signals), signals)
	}
}

func TestAnalyze_URLInComments(t *testing.T) {
	lines := []string{
		"// endpoint: http://user-service/api/v1",
		"# redis://cache:6379",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	// Only the // line should match for Go syntax
	found := false
	for _, s := range signals {
		if s.TargetComponent == "user-service" {
			found = true
			if s.Confidence != 0.4 {
				t.Errorf("expected confidence 0.4, got %.1f", s.Confidence)
			}
		}
	}
	if !found {
		t.Error("missing URL signal for user-service")
	}
}

func TestAnalyze_URLInPythonComment(t *testing.T) {
	lines := []string{
		"# redis://cache:6379",
	}

	signals := Analyze(lines, SyntaxPython, nil)
	found := false
	for _, s := range signals {
		if s.TargetComponent == "cache" {
			found = true
			if s.TargetType != "cache" {
				t.Errorf("expected target type cache, got %q", s.TargetType)
			}
		}
	}
	if !found {
		t.Error("missing URL signal for cache in Python comment")
	}
}

func TestAnalyze_FilteredURLs(t *testing.T) {
	lines := []string{
		"// see http://example.com",
		"// docs at https://docs.python.org",
		"// ref: http://localhost:8080/test",
		"// visit https://127.0.0.1/api",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	for _, s := range signals {
		// None of these should produce URL-based signals
		if s.TargetComponent == "example.com" || s.TargetComponent == "docs.python.org" ||
			s.TargetComponent == "localhost" || s.TargetComponent == "127.0.0.1" {
			t.Errorf("should filter out documentation/example URL: %q", s.TargetComponent)
		}
	}
}

func TestAnalyze_KnownComponentBoosting(t *testing.T) {
	lines := []string{
		"// Calls payment-api",
		"// Calls unknown-thing",
	}
	known := map[string]bool{
		"payment-api": true,
	}

	signals := Analyze(lines, SyntaxGo, known)
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	for _, s := range signals {
		switch s.TargetComponent {
		case "payment-api":
			if s.Confidence != 0.5 {
				t.Errorf("known component should get 0.5, got %.1f", s.Confidence)
			}
		case "unknown-thing":
			if s.Confidence != 0.4 {
				t.Errorf("unknown component should get 0.4, got %.1f", s.Confidence)
			}
		}
	}
}

func TestAnalyze_KnownComponentBoostingURL(t *testing.T) {
	lines := []string{
		"// endpoint: http://user-service/api/v1",
	}
	known := map[string]bool{
		"user-service": true,
	}

	signals := Analyze(lines, SyntaxGo, known)
	found := false
	for _, s := range signals {
		if s.TargetComponent == "user-service" {
			found = true
			if s.Confidence != 0.5 {
				t.Errorf("known URL component should get 0.5, got %.1f", s.Confidence)
			}
		}
	}
	if !found {
		t.Error("missing boosted URL signal for user-service")
	}
}

func TestAnalyze_MixedContent(t *testing.T) {
	// Regular code lines should NOT produce signals; only comments
	lines := []string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"// Calls payment-api",
		"func main() {",
		"    fmt.Println(\"Calls fake-service\")", // string literal, not a comment
		"    x := 42",
		"    // Uses cache-layer",
		"}",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals from comments only, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "payment-api" {
		t.Errorf("signal 0: expected payment-api, got %q", signals[0].TargetComponent)
	}
	if signals[1].TargetComponent != "cache-layer" {
		t.Errorf("signal 1: expected cache-layer, got %q", signals[1].TargetComponent)
	}
}

func TestAnalyze_LineNumbers(t *testing.T) {
	lines := []string{
		"package main",    // line 1
		"",                // line 2
		"// Calls foo-api", // line 3
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].LineNumber != 3 {
		t.Errorf("expected line number 3, got %d", signals[0].LineNumber)
	}
}

func TestAnalyze_Evidence(t *testing.T) {
	lines := []string{
		"// Calls payment-api for processing orders",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].Evidence == "" {
		t.Error("evidence should not be empty")
	}
	if len(signals[0].Evidence) > 200 {
		t.Errorf("evidence too long: %d chars", len(signals[0].Evidence))
	}
}

func TestAnalyze_SourceFileAndLanguageEmpty(t *testing.T) {
	lines := []string{
		"// Calls payment-api",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	if signals[0].SourceFile != "" {
		t.Errorf("SourceFile should be empty (caller sets it), got %q", signals[0].SourceFile)
	}
	if signals[0].Language != "" {
		t.Errorf("Language should be empty (caller sets it), got %q", signals[0].Language)
	}
}

func TestAnalyze_EmptyInput(t *testing.T) {
	signals := Analyze(nil, SyntaxGo, nil)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for nil input, got %d", len(signals))
	}

	signals = Analyze([]string{}, SyntaxGo, nil)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for empty input, got %d", len(signals))
	}
}

func TestAnalyze_NoMatchingComments(t *testing.T) {
	lines := []string{
		"// This is a regular comment",
		"// No dependency info here",
		"// Just documentation",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for non-dependency comments, got %d: %+v", len(signals), signals)
	}
}

func TestAnalyze_MultiLineDocstring(t *testing.T) {
	lines := []string{
		`"""`,
		"This module calls order-service",
		"for processing.",
		"It also depends on inventory-db.",
		`"""`,
		"import os",
	}

	signals := Analyze(lines, SyntaxPython, nil)
	found := map[string]bool{}
	for _, s := range signals {
		found[s.TargetComponent] = true
	}
	if !found["order-service"] {
		t.Error("missing signal for order-service in multi-line docstring")
	}
	if !found["inventory-db"] {
		t.Error("missing signal for inventory-db in multi-line docstring")
	}
}

func TestAnalyze_InlineCommentAfterCode(t *testing.T) {
	lines := []string{
		"x := 42 // Calls payment-api",
	}

	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal from inline comment, got %d: %+v", len(signals), signals)
	}
	if signals[0].TargetComponent != "payment-api" {
		t.Errorf("expected payment-api, got %q", signals[0].TargetComponent)
	}
}

func TestAnalyze_URLSchemeTypeInference(t *testing.T) {
	lines := []string{
		"// redis://my-cache:6379/0",
		"// postgres://my-db:5432/app",
		"// amqp://my-broker:5672",
	}

	signals := Analyze(lines, SyntaxGo, nil)

	typeMap := map[string]string{}
	for _, s := range signals {
		typeMap[s.TargetComponent] = s.TargetType
	}

	if typeMap["my-cache"] != "cache" {
		t.Errorf("redis URL should infer cache type, got %q", typeMap["my-cache"])
	}
	if typeMap["my-db"] != "database" {
		t.Errorf("postgres URL should infer database type, got %q", typeMap["my-db"])
	}
	if typeMap["my-broker"] != "message-broker" {
		t.Errorf("amqp URL should infer message-broker type, got %q", typeMap["my-broker"])
	}
}

func TestCommentSyntaxConstants(t *testing.T) {
	// Verify the enum values exist and are distinct
	if SyntaxGo == SyntaxPython {
		t.Error("SyntaxGo and SyntaxPython should be different")
	}
	if SyntaxPython == SyntaxJavaScript {
		t.Error("SyntaxPython and SyntaxJavaScript should be different")
	}
}

// Verify signals implement code.CodeSignal type
func TestAnalyze_ReturnsCodeSignals(t *testing.T) {
	lines := []string{"// Calls foo-api"}
	signals := Analyze(lines, SyntaxGo, nil)
	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}
	// This assignment verifies the return type at compile time
	var _ []code.CodeSignal = signals
}
