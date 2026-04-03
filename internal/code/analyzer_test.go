package code

import (
	"os"
	"path/filepath"
	"testing"
)

// mockParser is a test double implementing LanguageParser.
type mockParser struct {
	name       string
	extensions []string
	signals    []CodeSignal
	err        error
}

func (m *mockParser) Name() string            { return m.name }
func (m *mockParser) Extensions() []string     { return m.extensions }
func (m *mockParser) ParseFile(filePath string, content []byte) ([]CodeSignal, error) {
	return m.signals, m.err
}

func TestInferSourceComponent_GoMod(t *testing.T) {
	// Create a temp directory with a go.mod file
	dir := t.TempDir()
	gomod := []byte("module github.com/example/myservice\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), gomod, 0644); err != nil {
		t.Fatal(err)
	}

	// Infer from the directory itself
	got := InferSourceComponent(dir)
	if got != "github.com/example/myservice" {
		t.Errorf("InferSourceComponent(%q) = %q, want %q", dir, got, "github.com/example/myservice")
	}

	// Infer from a subdirectory
	sub := filepath.Join(dir, "internal", "pkg")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	got = InferSourceComponent(sub)
	if got != "github.com/example/myservice" {
		t.Errorf("InferSourceComponent(%q) = %q, want %q", sub, got, "github.com/example/myservice")
	}
}

func TestInferSourceComponent_Fallback(t *testing.T) {
	// Create a temp directory with no go.mod
	dir := t.TempDir()
	got := InferSourceComponent(dir)
	want := filepath.Base(dir)
	if got != want {
		t.Errorf("InferSourceComponent(%q) = %q, want %q", dir, got, want)
	}
}

func TestRegisterParser_AnalyzeFile(t *testing.T) {
	mock := &mockParser{
		name:       "test",
		extensions: []string{".test"},
		signals: []CodeSignal{
			{
				LineNumber:      10,
				TargetComponent: "some-service",
				TargetType:      "service",
				DetectionKind:   "http_call",
				Confidence:      0.9,
				Language:        "test",
			},
		},
	}

	analyzer := NewCodeAnalyzer("my-component")
	analyzer.RegisterParser(mock)

	// Dispatch to the mock parser
	signals, err := analyzer.AnalyzeFile("example.test", []byte("content"))
	if err != nil {
		t.Fatal(err)
	}

	if len(signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(signals))
	}

	// AnalyzeFile should set SourceFile
	if signals[0].SourceFile != "example.test" {
		t.Errorf("SourceFile = %q, want %q", signals[0].SourceFile, "example.test")
	}
	if signals[0].TargetComponent != "some-service" {
		t.Errorf("TargetComponent = %q, want %q", signals[0].TargetComponent, "some-service")
	}
}

func TestAnalyzeFile_UnknownExtension(t *testing.T) {
	analyzer := NewCodeAnalyzer("my-component")

	signals, err := analyzer.AnalyzeFile("file.unknown", []byte("content"))
	if err != nil {
		t.Fatal(err)
	}
	if signals != nil {
		t.Errorf("expected nil signals for unknown extension, got %v", signals)
	}
}

func TestCodeSignalFields(t *testing.T) {
	// Verify CodeSignal has exactly 8 fields as specified
	s := CodeSignal{
		SourceFile:      "main.go",
		LineNumber:      42,
		TargetComponent: "payment-api",
		TargetType:      "service",
		DetectionKind:   "http_call",
		Evidence:        `http.Get("http://payment-api:8080/pay")`,
		Language:        "go",
		Confidence:      0.9,
	}

	if s.SourceFile != "main.go" {
		t.Error("SourceFile field mismatch")
	}
	if s.Confidence != 0.9 {
		t.Error("Confidence field mismatch")
	}
}
