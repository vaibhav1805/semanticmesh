# Mendix Parser Test Data

This directory contains test fixtures for the mendixparser package.

## Structure

- `expected_outputs/`: JSON files with expected analysis results for validation

## Running Integration Tests

Integration tests require a real Mendix project and mxcli:

```bash
# Set path to test Mendix project
export MENDIX_TEST_PROJECT=/path/to/test/project.mpr

# Run all tests including integration
go test -v ./internal/code/mendixparser/...

# Run only unit tests (fast, no mxcli required)
go test -short ./internal/code/mendixparser/...
```

## Creating Test Data

To create test data for a new Mendix project:

1. Analyze the project:
   ```bash
   mxcli -p TestApp.mpr -c "REFRESH CATALOG FULL"
   mxcli -p TestApp.mpr -c "SHOW MODULES" --format json > expected_outputs/modules.json
   ```

2. Export expected catalog data:
   ```bash
   mxcli -p TestApp.mpr -c "SELECT * FROM CATALOG.REST_CLIENTS" --format json > expected_outputs/rest_clients.json
   mxcli -p TestApp.mpr -c "SELECT * FROM CATALOG.EXTERNAL_ENTITIES" --format json > expected_outputs/external_entities.json
   mxcli -p TestApp.mpr -c "SELECT SourceName, TargetName FROM CATALOG.REFS" --format json > expected_outputs/refs.json
   ```

3. Document expected signals:
   ```bash
   # Manually create expected_signals.json with expected ComponentSignals and DependencySignals
   ```

## Test Coverage

The test suite includes:

- **Unit Tests**: Test individual functions without requiring mxcli
- **Integration Tests**: Test with real Mendix projects (requires MENDIX_TEST_PROJECT env var)
- **Validation Tests**: Verify confidence scores, signal structure, and data integrity
- **Error Handling Tests**: Test behavior when mxcli is unavailable or projects are invalid

## Notes

- Integration tests are automatically skipped when `MENDIX_TEST_PROJECT` is not set
- Unit tests should not require mxcli or real .mpr files
- Mock data should represent realistic Mendix project structures
