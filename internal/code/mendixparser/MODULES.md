# Mendix Module and Internal Dependency Mapping (Phase 3)

## Overview

Phase 3 adds support for extracting module structure and internal dependencies from Mendix applications. This enables deep analysis of application architecture and inter-module relationships.

## Features

### Module Extraction

The `ExtractModules()` method identifies all modules within a Mendix application:

```go
modules, err := parser.ExtractModules(mprPath)
// Returns: ["Administration", "MyFirstModule", "System", "Atlas_Core"]
```

Modules are extracted using the mxcli catalog query:
```
SHOW MODULES
```

### Module Dependencies

The `ExtractModuleDependencies()` method builds a dependency graph between modules by analyzing element references:

```go
deps, err := parser.ExtractModuleDependencies(mprPath, appName)
```

**How it works:**
1. Queries all references: `SELECT SourceName, TargetName FROM CATALOG.REFS`
2. Extracts module names from qualified element names (e.g., `MyModule.Page1` -> `MyModule`)
3. Aggregates references between modules (filters out self-references)
4. Returns dependency signals with reference counts

**Example output:**
```go
DependencySignal{
    SourceName: "MyApp:MyFirstModule",
    TargetName: "MyApp:Administration",
    TargetType: "service",
    Confidence: 0.80,
    Kind:       "module_dependency",
    Evidence:   "5 references from MyFirstModule to Administration",
}
```

### Microflow Call Graph (Optional)

The `ExtractMicroflowDependencies()` method extracts detailed microflow-to-microflow dependencies:

```go
deps, err := parser.ExtractMicroflowDependencies(mprPath, appName, includeInternal)
```

This provides fine-grained analysis of execution flow within the application.

## Configuration

Use the `Config` struct to control internal dependency extraction:

```go
cfg := &Config{
    MxcliPath:                 "mxcli",
    RefreshCatalog:            true,     // Rebuild catalog before analysis
    IncludeInternalDeps:       true,     // Extract module and microflow deps
    DetectModulesAsComponents: true,     // Create component nodes for each module
}

parser := NewWithConfig(cfg)
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `MxcliPath` | `"mxcli"` | Path to mxcli binary |
| `RefreshCatalog` | `true` | Rebuild catalog before analysis |
| `IncludeInternalDeps` | `false` | Extract module and microflow dependencies |
| `DetectModulesAsComponents` | `false` | Create separate component nodes for modules |

## Usage Example

```go
// Create parser with internal dependency extraction enabled
cfg := &Config{
    MxcliPath:           "mxcli",
    IncludeInternalDeps: true,
}
parser := NewWithConfig(cfg)

// Analyze project
analysis, err := parser.AnalyzeProject("/path/to/MyApp.mpr")
if err != nil {
    log.Fatal(err)
}

// Access results
fmt.Printf("App: %s\n", analysis.AppName)
fmt.Printf("Modules: %v\n", analysis.Modules)
fmt.Printf("Dependencies: %d\n", len(analysis.Dependencies))

// Iterate over module dependencies
for _, dep := range analysis.Dependencies {
    if dep.Kind == "module_dependency" {
        fmt.Printf("%s -> %s (%s)\n", dep.SourceName, dep.TargetName, dep.Evidence)
    }
}
```

## Signal Types

### Component Signals

When `DetectModulesAsComponents` is enabled, each module becomes a component:

```go
ComponentSignal{
    Name:       "MyApp:MyModule",
    Type:       "service",
    Confidence: 0.85,
    SourceFile: "/path/to/MyApp.mpr",
    Evidence:   "Module in MyApp",
}
```

### Dependency Signals

Module dependencies are represented as:

```go
DependencySignal{
    SourceName: "MyApp:SourceModule",
    TargetName: "MyApp:TargetModule",
    TargetType: "service",
    Confidence: 0.80,
    Kind:       "module_dependency",  // or "microflow_call"
    Evidence:   "N references from SourceModule to TargetModule",
    SourceFile: "/path/to/MyApp.mpr",
}
```

## Error Handling

The implementation gracefully handles missing catalog tables:

- If `CATALOG.REFS` doesn't exist, returns empty dependency list
- If `CATALOG.MICROFLOWS` doesn't exist, skips microflow analysis
- Catalog queries are wrapped in try-catch to prevent failures from breaking analysis

## Performance Considerations

- **Catalog refresh**: Set `RefreshCatalog: false` to skip rebuild if catalog is up-to-date
- **Internal dependencies**: Set `IncludeInternalDeps: false` to skip module/microflow analysis for faster scans
- **Aggregation**: Module dependencies are aggregated to avoid creating excessive edges

## Testing

Run tests with:

```bash
# Unit tests (mock data)
go test ./internal/code/mendixparser/... -v -short

# Integration tests (requires mxcli and .mpr file)
go test ./internal/code/mendixparser/... -v
```

## Requirements

- **mxcli**: Must be installed and available in PATH
- **Mendix catalog**: Project must have a catalog database (created via `mxcli app-queries refresh-catalog`)
- **Mendix version**: Compatible with Mendix 8.x and 9.x

## Limitations

1. **Cross-app dependencies**: Phase 3 focuses on intra-app dependencies; inter-app dependencies are future work
2. **Widget dependencies**: Currently not extracted (future enhancement)
3. **Java actions**: Not included in microflow call graph (future enhancement)

## Next Steps

- **Phase 4**: Multi-app workspace support for cross-application dependency analysis
- **External dependencies**: Phase 2 (REST/SOAP service calls, database connections)
