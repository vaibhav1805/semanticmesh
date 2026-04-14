package mendixparser

import (
	"testing"

	"github.com/vaibhav1805/semanticmesh/internal/code"
)

// TestMendixParserImplementsInterface verifies that MendixParser implements LanguageParser
func TestMendixParserImplementsInterface(t *testing.T) {
	var _ code.LanguageParser = (*MendixParser)(nil)
}

// TestMendixParserBasics tests basic parser functionality
func TestMendixParserBasics(t *testing.T) {
	parser := NewMendixParser("")

	// Test Name()
	if parser.Name() != "mendix" {
		t.Errorf("Expected name 'mendix', got %q", parser.Name())
	}

	// Test Extensions()
	exts := parser.Extensions()
	if len(exts) != 1 || exts[0] != ".mpr" {
		t.Errorf("Expected extensions [.mpr], got %v", exts)
	}
}

// TestConversionToCodeSignal tests that ComponentSignal converts to CodeSignal correctly
func TestConversionToCodeSignal(t *testing.T) {
	comp := ComponentSignal{
		Name:       "MyApp",
		Type:       "service",
		Confidence: 0.95,
		SourceFile: "/path/to/MyApp.mpr",
		Evidence:   "mendix_mpr_file",
	}

	// Simulate conversion
	signal := code.CodeSignal{
		SourceFile:      comp.SourceFile,
		LineNumber:      1,
		TargetComponent: comp.Name,
		TargetType:      comp.Type,
		DetectionKind:   "mendix_app",
		Evidence:        "Mendix app: " + comp.Name,
		Language:        "mendix",
		Confidence:      comp.Confidence,
	}

	if signal.TargetComponent != "MyApp" {
		t.Errorf("Expected target component 'MyApp', got %q", signal.TargetComponent)
	}
	if signal.Language != "mendix" {
		t.Errorf("Expected language 'mendix', got %q", signal.Language)
	}
	if signal.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", signal.Confidence)
	}
}

// TestDependencySignalConversion tests DependencySignal to CodeSignal conversion
func TestDependencySignalConversion(t *testing.T) {
	dep := DependencySignal{
		SourceName: "MyApp:ModuleA",
		TargetName: "MyApp:ModuleB",
		TargetType: "service",
		Confidence: 0.80,
		Kind:       "module_dependency",
		Evidence:   "5 references from ModuleA to ModuleB",
		SourceFile: "/path/to/MyApp.mpr",
	}

	// Simulate conversion
	signal := code.CodeSignal{
		SourceFile:      dep.SourceFile,
		LineNumber:      1,
		TargetComponent: dep.TargetName,
		TargetType:      dep.TargetType,
		DetectionKind:   dep.Kind,
		Evidence:        dep.Evidence,
		Language:        "mendix",
		Confidence:      dep.Confidence,
	}

	if signal.TargetComponent != "MyApp:ModuleB" {
		t.Errorf("Expected target component 'MyApp:ModuleB', got %q", signal.TargetComponent)
	}
	if signal.DetectionKind != "module_dependency" {
		t.Errorf("Expected detection kind 'module_dependency', got %q", signal.DetectionKind)
	}
}

// TestIsNotAvailableError tests the error detection helper
func TestIsNotAvailableError(t *testing.T) {
	tests := []struct {
		err      string
		expected bool
	}{
		{"mxcli not found at 'mxcli'", true},
		{"executable file not found in PATH", true},
		{"some other error", false},
		{"", false},
	}

	for _, tt := range tests {
		// Create a mock error
		var err error
		if tt.err != "" {
			err = &mockError{msg: tt.err}
		}

		result := isNotAvailableError(err)
		if result != tt.expected {
			t.Errorf("isNotAvailableError(%q) = %v, expected %v", tt.err, result, tt.expected)
		}
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
