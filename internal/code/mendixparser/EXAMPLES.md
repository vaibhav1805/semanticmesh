# Mendix Parser Usage Examples

## Basic Usage

### 1. Detect Mendix Projects

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    projectDir := "/path/to/workspace"
    
    // Check if directory contains a Mendix project
    isMendix, mprPath, err := mendixparser.DetectMendixProject(projectDir)
    if err != nil {
        log.Fatal(err)
    }
    
    if isMendix {
        fmt.Printf("Found Mendix project: %s\n", mprPath)
        appName := mendixparser.ExtractAppName(mprPath)
        fmt.Printf("App name: %s\n", appName)
    }
}
```

### 2. Analyze Project with Default Config

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    // Create parser with default configuration
    parser := mendixparser.New("mxcli")
    
    // Analyze project
    analysis, err := parser.AnalyzeProject("/path/to/MyApp.mpr")
    if err != nil {
        log.Fatal(err)
    }
    
    // Print results
    fmt.Printf("App: %s\n", analysis.AppName)
    fmt.Printf("Components: %d\n", len(analysis.Components))
    fmt.Printf("Dependencies: %d\n", len(analysis.Dependencies))
    
    // List modules (if extracted)
    fmt.Printf("Modules: %v\n", analysis.Modules)
}
```

### 3. Extract Module Dependencies

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    // Create parser with internal dependencies enabled
    cfg := &mendixparser.Config{
        MxcliPath:           "mxcli",
        RefreshCatalog:      true,
        IncludeInternalDeps: true,
    }
    parser := mendixparser.NewWithConfig(cfg)
    
    // Analyze project
    analysis, err := parser.AnalyzeProject("/path/to/MyApp.mpr")
    if err != nil {
        log.Fatal(err)
    }
    
    // Print module list
    fmt.Println("Modules:")
    for _, module := range analysis.Modules {
        fmt.Printf("  - %s\n", module)
    }
    
    // Print module dependencies
    fmt.Println("\nModule Dependencies:")
    for _, dep := range analysis.Dependencies {
        if dep.Kind == "module_dependency" {
            fmt.Printf("  %s -> %s\n", dep.SourceName, dep.TargetName)
            fmt.Printf("    Evidence: %s\n", dep.Evidence)
        }
    }
}
```

### 4. Create Module Components

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    // Create parser with module components enabled
    cfg := &mendixparser.Config{
        MxcliPath:                 "mxcli",
        IncludeInternalDeps:       true,
        DetectModulesAsComponents: true,  // Creates component for each module
    }
    parser := mendixparser.NewWithConfig(cfg)
    
    // Analyze project
    analysis, err := parser.AnalyzeProject("/path/to/MyApp.mpr")
    if err != nil {
        log.Fatal(err)
    }
    
    // Print all components (app + modules)
    fmt.Println("Components:")
    for _, comp := range analysis.Components {
        fmt.Printf("  - Name: %s\n", comp.Name)
        fmt.Printf("    Type: %s\n", comp.Type)
        fmt.Printf("    Confidence: %.2f\n", comp.Confidence)
    }
}
```

### 5. Extract Microflow Dependencies

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    // Create parser with full internal analysis
    cfg := &mendixparser.Config{
        MxcliPath:           "mxcli",
        IncludeInternalDeps: true,
    }
    parser := mendixparser.NewWithConfig(cfg)
    
    // Analyze project
    analysis, err := parser.AnalyzeProject("/path/to/MyApp.mpr")
    if err != nil {
        log.Fatal(err)
    }
    
    // Print microflow dependencies
    fmt.Println("Microflow Dependencies:")
    for _, dep := range analysis.Dependencies {
        if dep.Kind == "microflow_call" {
            fmt.Printf("  %s -> %s\n", dep.SourceName, dep.TargetName)
        }
    }
}
```

### 6. Scan Directory for Mendix Projects

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    // Scan entire directory tree for Mendix projects
    signals, err := mendixparser.AnalyzeMendixProjects("/path/to/workspace")
    if err != nil {
        log.Fatal(err)
    }
    
    // Group by project
    projects := make(map[string]bool)
    for _, signal := range signals {
        if signal.DetectionKind == "mendix_app" {
            projects[signal.TargetComponent] = true
        }
    }
    
    fmt.Printf("Found %d Mendix projects:\n", len(projects))
    for project := range projects {
        fmt.Printf("  - %s\n", project)
    }
}
```

### 7. Integrate with Code Analyzer

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    // Create code analyzer with Mendix support
    analyzer := code.NewAnalyzer()
    
    // Register Mendix parser
    mendixParser := mendixparser.NewMendixParser("mxcli")
    analyzer.RegisterParser(mendixParser)
    
    // Analyze directory
    signals, err := analyzer.AnalyzeDirectory("/path/to/workspace")
    if err != nil {
        log.Fatal(err)
    }
    
    // Filter Mendix signals
    var mendixSignals []code.CodeSignal
    for _, sig := range signals {
        if sig.Language == "mendix" {
            mendixSignals = append(mendixSignals, sig)
        }
    }
    
    fmt.Printf("Found %d Mendix signals\n", len(mendixSignals))
}
```

## Configuration Examples

### Production Configuration (Fast Scan)

```go
cfg := &mendixparser.Config{
    MxcliPath:                 "mxcli",
    RefreshCatalog:            false,  // Skip rebuild if catalog exists
    IncludeInternalDeps:       false,  // Skip internal analysis
    DetectModulesAsComponents: false,
}
```

### Development Configuration (Deep Analysis)

```go
cfg := &mendixparser.Config{
    MxcliPath:                 "mxcli",
    RefreshCatalog:            true,   // Always rebuild for accuracy
    IncludeInternalDeps:       true,   // Extract all dependencies
    DetectModulesAsComponents: true,   // Create module components
}
```

### CI/CD Configuration

```go
cfg := &mendixparser.Config{
    MxcliPath:                 "/opt/mxcli/bin/mxcli",
    RefreshCatalog:            true,
    IncludeInternalDeps:       true,
    DetectModulesAsComponents: false,
}
```

## Error Handling

```go
package main

import (
    "fmt"
    "log"
    "github.com/vaibhav1805/semanticmesh/internal/code/mendixparser"
)

func main() {
    parser := mendixparser.New("mxcli")
    
    // Check if mxcli is available
    if err := parser.CheckMxcliAvailable(); err != nil {
        log.Printf("Warning: mxcli not available: %v", err)
        log.Printf("Skipping Mendix analysis")
        return
    }
    
    // Analyze with error handling
    analysis, err := parser.AnalyzeProject("/path/to/MyApp.mpr")
    if err != nil {
        // Check if it's a "not available" error (gracefully skip)
        if mendixparser.IsNotAvailableError(err) {
            log.Printf("Skipping project: %v", err)
            return
        }
        // Real error - fail
        log.Fatal(err)
    }
    
    fmt.Printf("Analysis complete: %s\n", analysis.AppName)
}
```

## Testing Examples

### Mock Module Extraction

```go
func TestModuleExtraction(t *testing.T) {
    // Simulate module extraction result
    modules := []string{
        "Administration",
        "MyFirstModule",
        "System",
    }
    
    if len(modules) != 3 {
        t.Errorf("Expected 3 modules, got %d", len(modules))
    }
}
```

### Mock Dependency Extraction

```go
func TestDependencyExtraction(t *testing.T) {
    // Simulate dependency signal
    dep := mendixparser.DependencySignal{
        SourceName: "MyApp:MyModule",
        TargetName: "MyApp:Administration",
        TargetType: "service",
        Confidence: 0.80,
        Kind:       "module_dependency",
        Evidence:   "5 references",
    }
    
    if dep.Confidence < 0.4 || dep.Confidence > 1.0 {
        t.Errorf("Invalid confidence: %f", dep.Confidence)
    }
}
```

## Command-Line Integration

```bash
#!/bin/bash
# Analyze Mendix project and export to JSON

go run cmd/analyze/main.go \
    --parser mendix \
    --workspace /path/to/workspace \
    --output mendix-analysis.json \
    --config '{"IncludeInternalDeps": true}'
```

## See Also

- [MODULES.md](./MODULES.md) - Detailed Phase 3 documentation
- [../../README.md](../../README.md) - Main semanticmesh documentation
- [mxcli documentation](https://github.com/mendixlabs/mxcli) - Mendix CLI tool
