# Mendix Parser Testing Guide

## Overview

The Mendix parser test suite is designed to validate all four phases of the implementation:
1. **Phase 1**: Basic Mendix app detection
2. **Phase 2**: External dependency extraction (REST APIs, databases, web services)
3. **Phase 3**: Module and internal dependencies
4. **Phase 4**: Multi-app workspace support (future)

## Test Structure

The test suite is organized into several test files:

### Unit Tests (No mxcli required)
- `parser_test.go` - Tests basic parser functionality, app name extraction, config
- `catalog_test.go` - Tests catalog helper functions (URL parsing, DB name extraction)
- `modules_test.go` - Tests module extraction logic and dependency structures
- `integration_test.go` - Tests interface implementation and signal conversion
- `confidence_test.go` - Validates confidence score ranges and consistency

### Integration Tests (Requires mxcli)
- `integration_runner_test.go` - Comprehensive integration tests with real Mendix projects

## Running Tests

### Unit Tests Only (Fast, No Dependencies)

```bash
# Run all unit tests (skips integration tests)
go test -short ./internal/code/mendixparser/

# Run with verbose output
go test -short -v ./internal/code/mendixparser/

# Run specific test file
go test -short ./internal/code/mendixparser/ -run TestExtractServiceNameFromURL
```

### All Tests Including Integration (Requires mxcli)

```bash
# Set path to test Mendix project
export MENDIX_TEST_PROJECT=/path/to/your/test/project.mpr

# Run all tests
go test -v ./internal/code/mendixparser/

# Run only integration tests
go test -v ./internal/code/mendixparser/ -run Integration
```

### With Coverage

```bash
# Generate coverage report
go test -cover ./internal/code/mendixparser/

# Generate detailed coverage HTML
go test -coverprofile=coverage.out ./internal/code/mendixparser/
go tool cover -html=coverage.out -o coverage.html
```

### Continuous Integration

```bash
# Fast unit tests for CI pipeline
go test -short -race ./internal/code/mendixparser/
```

## Test Coverage Goals

- **Unit Tests**: 80%+ coverage
- **Integration Tests**: Cover all major code paths (app analysis, module extraction, dependency detection)
- **Edge Cases**: Test error handling, missing mxcli, corrupt projects, empty results

## Prerequisites for Integration Tests

### 1. Install mxcli

```bash
# Clone mxcli repository
git clone https://github.com/mendixlabs/mxcli.git
cd mxcli

# Build and install
make install
```

### 2. Prepare Test Project

You need a Mendix project (.mpr file) for testing. Ideally, this project should have:
- Multiple modules
- REST API clients configured
- External database entities
- Consumed web services
- Inter-module dependencies

### 3. Set Environment Variable

```bash
export MENDIX_TEST_PROJECT=/path/to/TestApp.mpr
```

## Writing New Tests

### Adding Unit Tests

When adding new functionality:

1. **Write tests first** (TDD approach)
2. Place tests in the appropriate test file
3. Use descriptive test names
4. Include both happy path and error cases

Example:

```go
func TestNewFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "happy path",
            input: "valid input",
            want:  "expected output",
        },
        {
            name:    "error case",
            input:   "invalid",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewFeature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewFeature() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("NewFeature() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Adding Integration Tests

Add integration tests to `integration_runner_test.go`:

```go
func TestNewIntegrationFeature(t *testing.T) {
    testProject := os.Getenv("MENDIX_TEST_PROJECT")
    if testProject == "" {
        t.Skip("Set MENDIX_TEST_PROJECT to run integration tests")
    }

    parser := New("")
    if err := parser.CheckMxcliAvailable(); err != nil {
        t.Skipf("mxcli not available: %v", err)
    }

    t.Run("FeatureName", func(t *testing.T) {
        result, err := parser.NewFeature(testProject)
        if err != nil {
            t.Fatalf("NewFeature failed: %v", err)
        }

        // Validate result
        if result == nil {
            t.Error("Expected non-nil result")
        }

        t.Logf("Feature result: %v", result)
    })
}
```

## Test Data

Test data is stored in `testdata/`:

```
testdata/
├── README.md              # Documentation
└── expected_outputs/      # Expected analysis results
    ├── modules.json       # Expected module list
    ├── rest_clients.json  # Expected REST API clients
    └── refs.json          # Expected module references
```

### Creating Test Data

To create test data for a new Mendix project:

```bash
# Refresh catalog
mxcli -p TestApp.mpr -c "REFRESH CATALOG FULL"

# Export modules
mxcli -p TestApp.mpr -c "SHOW MODULES" --format json > testdata/expected_outputs/modules.json

# Export REST clients
mxcli -p TestApp.mpr -c "SELECT * FROM CATALOG.REST_CLIENTS" --format json > testdata/expected_outputs/rest_clients.json

# Export external entities
mxcli -p TestApp.mpr -c "SELECT * FROM CATALOG.EXTERNAL_ENTITIES" --format json > testdata/expected_outputs/external_entities.json

# Export module references
mxcli -p TestApp.mpr -c "SELECT SourceName, TargetName FROM CATALOG.REFS" --format json > testdata/expected_outputs/refs.json
```

## CI/CD Integration

Integration tests are automatically skipped in CI unless `MENDIX_TEST_PROJECT` is set.

### GitHub Actions Example

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      
      - name: Run unit tests
        run: go test -short -race -coverprofile=coverage.out ./internal/code/mendixparser/
      
      - name: Upload coverage
        uses: codecov/codecov-action@v2
        with:
          file: ./coverage.out
      
      # Optional: Run integration tests if test project is available
      - name: Run integration tests
        if: env.MENDIX_TEST_PROJECT != ''
        run: go test -v ./internal/code/mendixparser/...
        env:
          MENDIX_TEST_PROJECT: ${{ secrets.MENDIX_TEST_PROJECT }}
```

## Troubleshooting

### "mxcli not found"

Integration tests are automatically skipped if mxcli is not installed. To fix:
1. Install mxcli from https://github.com/mendixlabs/mxcli
2. Ensure it's in your PATH
3. Verify with `mxcli --version`

### "MENDIX_TEST_PROJECT not set"

Integration tests need a real Mendix project:
```bash
export MENDIX_TEST_PROJECT=/absolute/path/to/TestApp.mpr
```

### "Catalog table not found"

The catalog needs to be built first:
```bash
mxcli -p TestApp.mpr -c "REFRESH CATALOG FULL"
```

Or set `RefreshCatalog: true` in the parser config (default).

### Test Coverage Too Low

To identify uncovered code:
```bash
go test -coverprofile=coverage.out ./internal/code/mendixparser/
go tool cover -func=coverage.out | grep -v 100.0%
```

## Best Practices

1. **Keep unit tests fast** - Mock external dependencies
2. **Use table-driven tests** - Easy to add new test cases
3. **Test error paths** - Not just happy paths
4. **Validate confidence scores** - Ensure [0.4, 1.0] range
5. **Check signal structure** - Required fields should not be empty
6. **Document test data** - Explain what each test case validates
7. **Use descriptive names** - Test name should explain what's being tested
8. **Fail fast** - Use t.Fatalf() for setup failures, t.Errorf() for validation failures

## Validation Checklist

When adding new detection logic, ensure tests validate:

- [ ] Component signals have correct structure
- [ ] Dependency signals have correct structure
- [ ] Confidence scores are in valid range [0.4, 1.0]
- [ ] Confidence scores follow expected ordering
- [ ] Required fields are not empty
- [ ] Evidence strings are meaningful
- [ ] Detection kind is set correctly
- [ ] Target types match expected values
- [ ] Self-references are filtered out
- [ ] Error cases are handled gracefully
- [ ] mxcli unavailable scenario works correctly

## Future Enhancements

- [ ] Add benchmark tests for performance
- [ ] Create mock mxcli for fully isolated testing
- [ ] Add property-based testing for edge cases
- [ ] Generate test Mendix projects programmatically
- [ ] Add mutation testing for robustness
- [ ] Test concurrent project analysis (Phase 4)
