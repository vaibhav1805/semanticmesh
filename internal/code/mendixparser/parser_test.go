package mendixparser

import (
	"testing"
)

func TestDetectMendixProject(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name:    "non-existent path",
			path:    "/nonexistent",
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := DetectMendixProject(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectMendixProject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectMendixProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractAppName(t *testing.T) {
	tests := []struct {
		name    string
		mprPath string
		want    string
	}{
		{
			name:    "simple path",
			mprPath: "/path/to/MyApp.mpr",
			want:    "MyApp",
		},
		{
			name:    "project name",
			mprPath: "/path/to/project.mpr",
			want:    "project",
		},
		{
			name:    "current directory",
			mprPath: "app.mpr",
			want:    "app",
		},
		{
			name:    "nested path",
			mprPath: "/Users/dev/Projects/Mendix/MyApplication.mpr",
			want:    "MyApplication",
		},
		{
			name:    "with spaces",
			mprPath: "/path/to/My App Name.mpr",
			want:    "My App Name",
		},
		{
			name:    "with hyphens",
			mprPath: "/path/my-mendix-app.mpr",
			want:    "my-mendix-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractAppName(tt.mprPath); got != tt.want {
				t.Errorf("ExtractAppName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := &Config{
		MxcliPath:                 "/custom/mxcli",
		RefreshCatalog:            false,
		IncludeInternalDeps:       true,
		DetectModulesAsComponents: true,
	}

	parser := NewWithConfig(cfg)

	if parser.config.MxcliPath != "/custom/mxcli" {
		t.Errorf("Expected MxcliPath to be /custom/mxcli, got %s", parser.config.MxcliPath)
	}

	if parser.config.IncludeInternalDeps != true {
		t.Errorf("Expected IncludeInternalDeps to be true")
	}

	if parser.config.DetectModulesAsComponents != true {
		t.Errorf("Expected DetectModulesAsComponents to be true")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MxcliPath != "mxcli" {
		t.Errorf("Expected default MxcliPath to be 'mxcli', got %s", cfg.MxcliPath)
	}

	if cfg.RefreshCatalog != true {
		t.Errorf("Expected default RefreshCatalog to be true")
	}

	if cfg.IncludeInternalDeps != false {
		t.Errorf("Expected default IncludeInternalDeps to be false")
	}

	if cfg.DetectModulesAsComponents != false {
		t.Errorf("Expected default DetectModulesAsComponents to be false")
	}
}

// TestExtractModules tests module extraction (requires mxcli and test data)
// This test is marked for integration testing as it requires actual Mendix project
func TestExtractModules_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would need a real .mpr file and mxcli installed
	// TODO: Add integration test with real data
	t.Skip("Integration test - requires mxcli and test .mpr file")
}

// TestExtractModuleDependencies tests module dependency extraction
func TestExtractModuleDependencies_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would need a real .mpr file and mxcli installed
	// TODO: Add integration test with real data
	t.Skip("Integration test - requires mxcli and test .mpr file")
}

// Integration tests for Phase 2 external dependency extraction

func TestExtractRESTAPIs_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would need a real .mpr file with REST clients configured
	// TODO: Add integration test with real data
	t.Skip("Integration test - requires mxcli and test .mpr file with REST clients")
}

func TestExtractDatabases_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would need a real .mpr file with external entities configured
	// TODO: Add integration test with real data
	t.Skip("Integration test - requires mxcli and test .mpr file with external entities")
}

func TestExtractConsumedServices_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This would need a real .mpr file with consumed web services
	// TODO: Add integration test with real data
	t.Skip("Integration test - requires mxcli and test .mpr file with consumed services")
}

// TestAnalyzeProject_WithExternalDeps verifies that AnalyzeProject calls Phase 2 methods
func TestAnalyzeProject_WithExternalDeps_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// This integration test would verify:
	// 1. REST APIs are extracted
	// 2. Databases are extracted
	// 3. Consumed services are extracted
	// 4. All dependencies are included in the analysis result
	t.Skip("Integration test - requires mxcli and comprehensive test .mpr file")
}

// TestNew tests parser creation
func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		mxcliPath string
		want      string
	}{
		{
			name:      "empty path defaults to mxcli",
			mxcliPath: "",
			want:      "mxcli",
		},
		{
			name:      "custom path",
			mxcliPath: "/usr/local/bin/mxcli",
			want:      "/usr/local/bin/mxcli",
		},
		{
			name:      "relative path",
			mxcliPath: "./bin/mxcli",
			want:      "./bin/mxcli",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New(tt.mxcliPath)
			if parser.config.MxcliPath != tt.want {
				t.Errorf("New() MxcliPath = %v, want %v", parser.config.MxcliPath, tt.want)
			}
			if parser.config == nil {
				t.Error("Expected non-nil config")
			}
		})
	}
}

// TestConfigValidation tests that config values are used correctly
func TestConfigValidation(t *testing.T) {
	t.Run("default config has expected values", func(t *testing.T) {
		cfg := DefaultConfig()

		if cfg.MxcliPath != "mxcli" {
			t.Errorf("Default MxcliPath = %s, want 'mxcli'", cfg.MxcliPath)
		}
		if !cfg.RefreshCatalog {
			t.Error("Default RefreshCatalog should be true")
		}
		if cfg.IncludeInternalDeps {
			t.Error("Default IncludeInternalDeps should be false")
		}
		if cfg.DetectModulesAsComponents {
			t.Error("Default DetectModulesAsComponents should be false")
		}
	})

	t.Run("custom config overrides defaults", func(t *testing.T) {
		cfg := &Config{
			MxcliPath:                 "/custom/path",
			RefreshCatalog:            false,
			IncludeInternalDeps:       true,
			DetectModulesAsComponents: true,
		}

		parser := NewWithConfig(cfg)

		if parser.config.MxcliPath != "/custom/path" {
			t.Errorf("MxcliPath = %s, want '/custom/path'", parser.config.MxcliPath)
		}
		if parser.config.RefreshCatalog {
			t.Error("RefreshCatalog should be false")
		}
		if !parser.config.IncludeInternalDeps {
			t.Error("IncludeInternalDeps should be true")
		}
		if !parser.config.DetectModulesAsComponents {
			t.Error("DetectModulesAsComponents should be true")
		}
	})
}

// TestProjectAnalysisStructure validates ProjectAnalysis structure
func TestProjectAnalysisStructure(t *testing.T) {
	analysis := &ProjectAnalysis{
		AppName: "TestApp",
		MprPath: "/path/to/TestApp.mpr",
		Components: []ComponentSignal{
			{
				Name:       "TestApp",
				Type:       "service",
				Confidence: 0.95,
				SourceFile: "/path/to/TestApp.mpr",
				Evidence:   "mendix_mpr_file",
			},
		},
		Dependencies: []DependencySignal{
			{
				SourceName: "TestApp",
				TargetName: "ExternalService",
				TargetType: "service",
				Confidence: 0.90,
				Kind:       "rest_api_call",
				Evidence:   "REST client",
				SourceFile: "/path/to/TestApp.mpr",
			},
		},
		Modules: []string{"ModuleA", "ModuleB"},
	}

	// Validate structure
	if analysis.AppName == "" {
		t.Error("AppName should not be empty")
	}
	if analysis.MprPath == "" {
		t.Error("MprPath should not be empty")
	}
	if len(analysis.Components) == 0 {
		t.Error("Should have at least one component")
	}
	if len(analysis.Dependencies) == 0 {
		t.Error("Should have at least one dependency")
	}
	if len(analysis.Modules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(analysis.Modules))
	}

	// Validate component structure
	comp := analysis.Components[0]
	if comp.Name == "" {
		t.Error("Component Name should not be empty")
	}
	if comp.Type == "" {
		t.Error("Component Type should not be empty")
	}
	if comp.Confidence < 0.4 || comp.Confidence > 1.0 {
		t.Errorf("Component Confidence out of range: %f", comp.Confidence)
	}

	// Validate dependency structure
	dep := analysis.Dependencies[0]
	if dep.SourceName == "" {
		t.Error("Dependency SourceName should not be empty")
	}
	if dep.TargetName == "" {
		t.Error("Dependency TargetName should not be empty")
	}
	if dep.Kind == "" {
		t.Error("Dependency Kind should not be empty")
	}
}
