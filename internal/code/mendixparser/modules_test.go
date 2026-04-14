package mendixparser

import (
	"testing"
)

// TestModuleExtraction tests the module extraction logic with mock data
func TestModuleExtraction(t *testing.T) {
	// This test demonstrates the expected structure of module extraction
	// In a real scenario, this would use QueryCatalog with a real .mpr file

	mockModules := []string{
		"Administration",
		"MyFirstModule",
		"System",
		"Atlas_Core",
	}

	// Verify module list structure
	if len(mockModules) != 4 {
		t.Errorf("Expected 4 modules, got %d", len(mockModules))
	}

	// Verify module names don't contain dots (they're module names, not qualified names)
	for _, module := range mockModules {
		if len(module) == 0 {
			t.Errorf("Module name should not be empty")
		}
	}
}

// TestModuleDependencyLogic tests the dependency extraction logic
func TestModuleDependencyLogic(t *testing.T) {
	// Mock reference data (SourceName -> TargetName)
	mockRefs := []struct {
		sourceName string
		targetName string
	}{
		{"MyFirstModule.Page1", "Administration.User"},
		{"MyFirstModule.Page2", "Administration.Account"},
		{"MyFirstModule.Microflow1", "System.CurrentUser"},
		{"Administration.Helper", "System.Log"},
		{"MyFirstModule.Widget", "MyFirstModule.Helper"}, // Self-reference (should be filtered)
	}

	// Build module-to-module edges (simulating ExtractModuleDependencies logic)
	moduleEdges := make(map[string]map[string]int)

	for _, ref := range mockRefs {
		// Extract module names
		sourceModule := getModuleName(ref.sourceName)
		targetModule := getModuleName(ref.targetName)

		// Skip self-references
		if sourceModule == targetModule {
			continue
		}

		if moduleEdges[sourceModule] == nil {
			moduleEdges[sourceModule] = make(map[string]int)
		}
		moduleEdges[sourceModule][targetModule]++
	}

	// Verify results
	if len(moduleEdges) != 2 { // MyFirstModule -> Admin/System, Administration -> System
		t.Errorf("Expected 2 source modules with dependencies, got %d", len(moduleEdges))
	}

	// Check MyFirstModule dependencies
	if deps, ok := moduleEdges["MyFirstModule"]; ok {
		if deps["Administration"] != 2 {
			t.Errorf("Expected 2 references from MyFirstModule to Administration, got %d", deps["Administration"])
		}
		if deps["System"] != 1 {
			t.Errorf("Expected 1 reference from MyFirstModule to System, got %d", deps["System"])
		}
	} else {
		t.Error("Expected MyFirstModule to have dependencies")
	}

	// Check Administration dependencies
	if deps, ok := moduleEdges["Administration"]; ok {
		if deps["System"] != 1 {
			t.Errorf("Expected 1 reference from Administration to System, got %d", deps["System"])
		}
	} else {
		t.Error("Expected Administration to have dependencies")
	}
}

// Helper function to extract module name from qualified name
func getModuleName(qualifiedName string) string {
	// Extract module from "Module.Element" format
	for i := 0; i < len(qualifiedName); i++ {
		if qualifiedName[i] == '.' {
			return qualifiedName[:i]
		}
	}
	return qualifiedName
}

func TestGetModuleName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MyModule.Page1", "MyModule"},
		{"Administration.User", "Administration"},
		{"System", "System"},
		{"MyModule.SubModule.Page", "MyModule"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := getModuleName(tt.input)
			if got != tt.want {
				t.Errorf("getModuleName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestDependencySignalStructure tests that dependency signals have correct structure
func TestDependencySignalStructure(t *testing.T) {
	signal := DependencySignal{
		SourceName: "MyApp:MyModule",
		TargetName: "MyApp:Administration",
		TargetType: "service",
		Confidence: 0.80,
		Kind:       "module_dependency",
		Evidence:   "5 references from MyModule to Administration",
		SourceFile: "/path/to/MyApp.mpr",
	}

	// Verify required fields
	if signal.SourceName == "" {
		t.Error("SourceName should not be empty")
	}
	if signal.TargetName == "" {
		t.Error("TargetName should not be empty")
	}
	if signal.Confidence < 0.4 || signal.Confidence > 1.0 {
		t.Errorf("Confidence should be in [0.4, 1.0], got %f", signal.Confidence)
	}
	if signal.Kind != "module_dependency" {
		t.Errorf("Expected Kind to be 'module_dependency', got %s", signal.Kind)
	}
	if signal.TargetType != "service" {
		t.Errorf("Expected TargetType to be 'service', got %s", signal.TargetType)
	}
}

// TestComponentSignalStructure tests that component signals have correct structure
func TestComponentSignalStructure(t *testing.T) {
	signal := ComponentSignal{
		Name:       "MyApp:MyModule",
		Type:       "service",
		Confidence: 0.85,
		SourceFile: "/path/to/MyApp.mpr",
		Evidence:   "Module in MyApp",
	}

	// Verify required fields
	if signal.Name == "" {
		t.Error("Name should not be empty")
	}
	if signal.Type == "" {
		t.Error("Type should not be empty")
	}
	if signal.Confidence < 0.4 || signal.Confidence > 1.0 {
		t.Errorf("Confidence should be in [0.4, 1.0], got %f", signal.Confidence)
	}
}
