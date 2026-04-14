package mendixparser

import (
	"os"
	"testing"
)

// TestIntegrationRunner runs comprehensive integration tests if MENDIX_TEST_PROJECT is set
func TestIntegrationRunner(t *testing.T) {
	testProject := os.Getenv("MENDIX_TEST_PROJECT")
	if testProject == "" {
		t.Skip("Set MENDIX_TEST_PROJECT environment variable to run integration tests")
	}

	parser := New("")

	t.Run("AnalyzeProject", func(t *testing.T) {
		analysis, err := parser.AnalyzeProject(testProject)
		if err != nil {
			t.Fatalf("AnalyzeProject failed: %v", err)
		}

		if analysis.AppName == "" {
			t.Error("Expected non-empty app name")
		}

		if len(analysis.Components) == 0 {
			t.Error("Expected at least one component (the app itself)")
		}

		// Validate component structure
		appComponent := analysis.Components[0]
		if appComponent.Type != "service" {
			t.Errorf("Expected app type 'service', got %s", appComponent.Type)
		}
		if appComponent.Confidence < 0.9 {
			t.Errorf("Expected high confidence (>= 0.9) for app detection, got %f", appComponent.Confidence)
		}
		if appComponent.SourceFile == "" {
			t.Error("Expected non-empty source file")
		}
		if appComponent.Evidence == "" {
			t.Error("Expected non-empty evidence")
		}

		t.Logf("Successfully analyzed app: %s with %d components and %d dependencies",
			analysis.AppName, len(analysis.Components), len(analysis.Dependencies))
	})

	t.Run("ExtractModules", func(t *testing.T) {
		cm, err := NewCatalogManager(testProject)
		if err != nil {
			t.Fatalf("Failed to create catalog manager: %v", err)
		}
		defer cm.Close()

		if err := cm.BuildFull(); err != nil {
			t.Fatalf("Failed to build catalog: %v", err)
		}

		modules, err := parser.ExtractModules(cm)
		if err != nil {
			t.Fatalf("ExtractModules failed: %v", err)
		}

		if len(modules) == 0 {
			t.Error("Expected at least one module")
		}

		// Validate module names
		for _, module := range modules {
			if module == "" {
				t.Error("Module name should not be empty")
			}
		}

		t.Logf("Found %d modules: %v", len(modules), modules)
	})

	t.Run("ExtractRESTAPIs", func(t *testing.T) {
		cm, err := NewCatalogManager(testProject)
		if err != nil {
			t.Fatalf("Failed to create catalog manager: %v", err)
		}
		defer cm.Close()

		if err := cm.BuildFull(); err != nil {
			t.Fatalf("Failed to build catalog: %v", err)
		}

		appName := ExtractAppName(testProject)
		signals, err := parser.ExtractRESTAPIs(cm, appName, testProject)
		if err != nil {
			t.Fatalf("ExtractRESTAPIs failed: %v", err)
		}

		// Validate signal structure
		for i, signal := range signals {
			if signal.SourceName != appName {
				t.Errorf("Signal[%d] SourceName should be %s, got %s", i, appName, signal.SourceName)
			}
			if signal.TargetName == "" {
				t.Errorf("Signal[%d] TargetName should not be empty", i)
			}
			if signal.TargetType != "service" {
				t.Errorf("Signal[%d] TargetType should be 'service', got %s", i, signal.TargetType)
			}
			if signal.Confidence < 0.4 || signal.Confidence > 1.0 {
				t.Errorf("Signal[%d] Confidence out of range: %f", i, signal.Confidence)
			}
			if signal.Kind != "rest_api_call" {
				t.Errorf("Signal[%d] Kind should be 'rest_api_call', got %s", i, signal.Kind)
			}
		}

		t.Logf("Found %d REST API dependencies", len(signals))
	})

	t.Run("ExtractDatabases", func(t *testing.T) {
		cm, err := NewCatalogManager(testProject)
		if err != nil {
			t.Fatalf("Failed to create catalog manager: %v", err)
		}
		defer cm.Close()

		if err := cm.BuildFull(); err != nil {
			t.Fatalf("Failed to build catalog: %v", err)
		}

		appName := ExtractAppName(testProject)
		signals, err := parser.ExtractDatabases(cm, appName, testProject)
		if err != nil {
			t.Fatalf("ExtractDatabases failed: %v", err)
		}

		// Validate signal structure
		for i, signal := range signals {
			if signal.SourceName != appName {
				t.Errorf("Signal[%d] SourceName should be %s, got %s", i, appName, signal.SourceName)
			}
			if signal.TargetType != "database" {
				t.Errorf("Signal[%d] TargetType should be 'database', got %s", i, signal.TargetType)
			}
			if signal.Kind != "db_connection" {
				t.Errorf("Signal[%d] Kind should be 'db_connection', got %s", i, signal.Kind)
			}
		}

		t.Logf("Found %d database dependencies", len(signals))
	})

	t.Run("ExtractModuleDependencies", func(t *testing.T) {
		cm, err := NewCatalogManager(testProject)
		if err != nil {
			t.Fatalf("Failed to create catalog manager: %v", err)
		}
		defer cm.Close()

		if err := cm.BuildFull(); err != nil {
			t.Fatalf("Failed to build catalog: %v", err)
		}

		appName := ExtractAppName(testProject)
		signals, err := parser.ExtractModuleDependencies(cm, appName, testProject)
		if err != nil {
			t.Fatalf("ExtractModuleDependencies failed: %v", err)
		}

		// Validate signal structure
		for i, signal := range signals {
			if signal.Kind != "module_dependency" {
				t.Errorf("Signal[%d] Kind should be 'module_dependency', got %s", i, signal.Kind)
			}
			if signal.TargetType != "service" {
				t.Errorf("Signal[%d] TargetType should be 'service', got %s", i, signal.TargetType)
			}
			// Check that source and target are different modules
			if signal.SourceName == signal.TargetName {
				t.Errorf("Signal[%d] should not be a self-reference: %s", i, signal.SourceName)
			}
		}

		t.Logf("Found %d module dependencies", len(signals))
	})

	t.Run("ExtractConsumedServices", func(t *testing.T) {
		cm, err := NewCatalogManager(testProject)
		if err != nil {
			t.Fatalf("Failed to create catalog manager: %v", err)
		}
		defer cm.Close()

		if err := cm.BuildFull(); err != nil {
			t.Fatalf("Failed to build catalog: %v", err)
		}

		appName := ExtractAppName(testProject)
		signals, err := parser.ExtractConsumedServices(cm, appName, testProject)
		if err != nil {
			t.Fatalf("ExtractConsumedServices failed: %v", err)
		}

		// Validate signal structure
		for i, signal := range signals {
			if signal.Kind != "webservice_call" {
				t.Errorf("Signal[%d] Kind should be 'webservice_call', got %s", i, signal.Kind)
			}
		}

		t.Logf("Found %d consumed web services", len(signals))
	})
}

// TestIntegrationWithConfig tests parser with custom configuration
func TestIntegrationWithConfig(t *testing.T) {
	testProject := os.Getenv("MENDIX_TEST_PROJECT")
	if testProject == "" {
		t.Skip("Set MENDIX_TEST_PROJECT environment variable to run integration tests")
	}

	t.Run("WithModulesAsComponents", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.DetectModulesAsComponents = true
		cfg.IncludeInternalDeps = false

		parser := NewWithConfig(cfg)
		if err := parser.CheckMxcliAvailable(); err != nil {
			t.Skipf("mxcli not available: %v", err)
		}

		analysis, err := parser.AnalyzeProject(testProject)
		if err != nil {
			t.Fatalf("AnalyzeProject failed: %v", err)
		}

		// With DetectModulesAsComponents=true, we should have:
		// 1 app component + N module components
		if len(analysis.Components) < 2 {
			t.Errorf("Expected at least 2 components (app + modules), got %d", len(analysis.Components))
		}

		t.Logf("Created %d components with DetectModulesAsComponents=true", len(analysis.Components))
	})

	t.Run("WithInternalDeps", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.IncludeInternalDeps = true
		cfg.DetectModulesAsComponents = false

		parser := NewWithConfig(cfg)
		if err := parser.CheckMxcliAvailable(); err != nil {
			t.Skipf("mxcli not available: %v", err)
		}

		analysis, err := parser.AnalyzeProject(testProject)
		if err != nil {
			t.Fatalf("AnalyzeProject failed: %v", err)
		}

		// Check that module dependencies are included
		hasModuleDeps := false
		for _, dep := range analysis.Dependencies {
			if dep.Kind == "module_dependency" {
				hasModuleDeps = true
				break
			}
		}

		if !hasModuleDeps {
			t.Log("No module dependencies found (this is OK if project has no inter-module refs)")
		}

		t.Logf("Found %d total dependencies with IncludeInternalDeps=true", len(analysis.Dependencies))
	})

	t.Run("WithoutCatalogRefresh", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.RefreshCatalog = false

		parser := NewWithConfig(cfg)
		if err := parser.CheckMxcliAvailable(); err != nil {
			t.Skipf("mxcli not available: %v", err)
		}

		// This should work if catalog was already built
		analysis, err := parser.AnalyzeProject(testProject)
		if err != nil {
			t.Logf("Analysis without refresh failed (expected if catalog doesn't exist): %v", err)
		} else {
			t.Logf("Analysis succeeded without catalog refresh: %s", analysis.AppName)
		}
	})
}

// TestDetectMendixProjectIntegration tests project detection with real directory
func TestDetectMendixProjectIntegration(t *testing.T) {
	testProject := os.Getenv("MENDIX_TEST_PROJECT")
	if testProject == "" {
		t.Skip("Set MENDIX_TEST_PROJECT environment variable to run integration tests")
	}

	// Get parent directory
	projectDir := testProject
	if stat, err := os.Stat(testProject); err == nil && !stat.IsDir() {
		// testProject is a file, get its directory
		projectDir = testProject[:len(testProject)-len(testProject[len(testProject)-1:])]
		for projectDir[len(projectDir)-1] != '/' {
			projectDir = projectDir[:len(projectDir)-1]
		}
	}

	isMendix, mprPath, err := DetectMendixProject(projectDir)
	if err != nil {
		t.Fatalf("DetectMendixProject failed: %v", err)
	}

	if !isMendix {
		t.Error("Expected Mendix project to be detected")
	}

	if mprPath == "" {
		t.Error("Expected non-empty .mpr path")
	}

	t.Logf("Detected Mendix project at: %s", mprPath)
}
