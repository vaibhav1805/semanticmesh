# Mendix Extraction Guide

**Last updated:** 2026-04-14

## Overview

semanticmesh extracts comprehensive architectural metadata from Mendix applications using the mxcli Go library. The extraction pipeline analyzes the application catalog and extracts 14+ tables covering published APIs, domain models, business logic, UI structure, and configuration—all in ~1.5-2.3 seconds.

**Key benefits:**
- **No external dependencies:** mxcli bundled as Go module (no binary installation)
- **Fast extraction:** Hundreds of items in ~1-2 seconds
- **Configurable profiles:** Balance speed and depth for different use cases
- **Typed components:** Published APIs, entities, microflows, pages with confidence scores
- **AI agent ready:** Structured JSON output for MCP server integration

---

## What Gets Extracted

### Tier 1: External Dependencies & Published Services

**Purpose:** Understand what the app depends on and what it exposes to the outside world.

#### Published Services

**What:** REST and OData APIs exposed by this application

**Examples from a typical Mendix app:**
- `PRS_OrderAPI` (REST) - Order management API with multiple operations
- `PRS_CustomerAPI` (REST) - Customer data integration API
- `PRS_Webhooks` (REST) - Webhook receiver endpoints

**Data extracted:**
```json
{
  "name": "PRS_OrderAPI",
  "type": "rest",
  "path": "rest/orders",
  "module_name": "OrderModule",
  "operations": [
    {
      "resource_name": "OrderResource",
      "http_method": "POST",
      "path": "orders/{orderId}",
      "microflow": "OrderModule.ProcessOrder"
    }
  ]
}
```

**Use cases:**
- "What APIs does this app expose?"
- "Which microflow handles the POST /orders endpoint?"
- "What's the complete API surface for this application?"

#### Domain Model

**What:** Entities, attributes, and external entities (consumed OData/REST)

**Examples from a typical Mendix app:**
- 100+ entities total
- `Customer`, `Order`, `Invoice`, `Payment`, `User` (business entities)
- `LogEntry`, `SystemMetrics`, `Configuration` (system entities)

**Data extracted:**
```json
{
  "name": "Customer",
  "qualified_name": "CustomerModule.Customer",
  "module_name": "CustomerModule",
  "entity_type": "Entity",
  "attribute_count": 12,
  "is_external": false
}
```

**Use cases:**
- "Which entities are in the CustomerModule?"
- "How many attributes does the Order entity have?"
- "Are there any external entities (consumed from other systems)?"

#### External Dependencies

**What:** REST clients, OData clients, consumed services

**Examples from a typical Mendix app:**
- REST clients to external payment APIs
- OData feeds from legacy systems
- SOAP services (if any)

**Use cases:**
- "What external services does this app depend on?"
- "If the payment API goes down, what breaks?"

---

### Tier 2: Internal Structure

**Purpose:** Understand the internal architecture for impact analysis and refactoring.

#### Business Logic

**What:** Microflows and Java Actions

**Examples from a typical Mendix app:**
- 200+ microflows (e.g., `ACT_ProcessOrder`, `ACT_ValidateCustomer`, `SUB_CalculateTotal`)
- 100+ Java actions (e.g., `ExportToExcel`, `SendEmail`, `EncryptPassword`)

**Data extracted:**
```json
{
  "name": "ACT_ProcessOrder",
  "qualified_name": "OrderModule.ACT_ProcessOrder",
  "module_name": "OrderModule",
  "type": "Microflow",
  "activity_count": 15,
  "complexity": 8,
  "is_scheduled": false
}
```

**Use cases:**
- "Which microflows are in the OrderModule?"
- "What's the complexity of ACT_ProcessOrder?"
- "Which microflows are scheduled tasks?"

#### UI Structure

**What:** Pages and navigation entry points

**Examples from a typical Mendix app:**
- 100+ pages (e.g., `Login`, `Dashboard`, `CustomerDetail`, `OrderList`)
- Navigation layouts and templates

**Data extracted:**
```json
{
  "name": "CustomerDetail",
  "qualified_name": "CustomerModule.CustomerDetail",
  "module_name": "CustomerModule"
}
```

**Use cases:**
- "Which pages use the Customer entity?"
- "What's the UI structure of the CustomerModule?"

#### Configuration

**What:** Constants and environment settings

**Examples from a typical Mendix app:**
- Dozens of constants (e.g., `API_Timeout`, `MaxRetries`, `EnableDebugLogging`)

**Data extracted:**
```json
{
  "name": "API_Timeout",
  "qualified_name": "ConfigModule.API_Timeout",
  "module_name": "ConfigModule",
  "value": "30000"
}
```

**Use cases:**
- "What configuration constants exist?"
- "Which module owns the API_Timeout constant?"

---

## Extraction Profiles

### Profile Comparison

| Feature | Minimal | Standard | Comprehensive |
|---------|---------|----------|---------------|
| **Time** | ~1-2s | ~2-3s | ~1-2s |
| **Items** | 100-200 | 500-1000 | 500-1000+ |
| **Tables** | 4 | 8 | 10+ |
| **Modules** | ✓ | ✓ | ✓ |
| **Published APIs** | ✓ | ✓ | ✓ |
| **Entities** | ✓ | ✓ | ✓ |
| **Microflows** | ✗ | ✓ | ✓ |
| **Java Actions** | ✗ | ✓ | ✓ |
| **Pages** | ✗ | ✓ | ✓ |
| **Constants** | ✗ | ✓ | ✓ |
| **Module Deps** | ✗ | ✗ | ✓ |
| **Microflow Calls** | ✗ | ✗ | ✓ |

---

### When to Use Each Profile

#### Use Minimal Profile When:

- **Running in CI/CD pipelines** - Fast scans for automated checks
- **Checking external dependencies** - "What does this app depend on?"
- **Discovering API surface** - "What APIs does this app expose?"
- **High-frequency scans** - Hourly/daily automated analysis

**Configuration:**
```yaml
code_analysis:
  mendix:
    extraction_profile: "minimal"
```

**Example workflow:**
```bash
# CI/CD pipeline: Check if new dependencies were added
semanticmesh export --input ./app --output app.zip --analyze-code
semanticmesh import app.zip --name ci-scan
semanticmesh query dependencies --component MyApp --format json > deps.json
diff deps.json previous-deps.json  # Alert if dependencies changed
```

---

#### Use Standard Profile When:

- **Performing impact analysis** - "If X fails, what breaks?"
- **Generating architecture documentation** - Complete app structure
- **Change impact assessment** - "What microflows use the Customer entity?"
- **Service dependency mapping** - Understanding cross-service calls

**Configuration:**
```yaml
code_analysis:
  mendix:
    extraction_profile: "standard"  # This is the default
```

**Example workflow:**
```bash
# Impact analysis before making a change
semanticmesh export --input ./app --output app.zip --analyze-code
semanticmesh import app.zip --name prod-analysis
semanticmesh query impact --component Customer --depth all --format json

# Result: "Customer entity is used by 15 microflows, 8 pages, 3 APIs"
```

---

#### Use Comprehensive Profile When:

- **Planning refactoring** - Understanding module coupling
- **Deep architectural investigation** - Complete internal structure
- **Module boundary analysis** - "Which modules depend on CustomerModule?"
- **Technical debt assessment** - Finding circular dependencies

**Configuration:**
```yaml
code_analysis:
  mendix:
    extraction_profile: "comprehensive"
    include_internal_deps: true
```

**Example workflow:**
```bash
# Refactoring analysis: Find module coupling
semanticmesh export --input ./app --output app.zip --analyze-code
semanticmesh import app.zip --name refactor-analysis
semanticmesh query dependencies --component CustomerModule --include-internal --format json

# Result: "CustomerModule has 12 incoming dependencies from 5 other modules"
```

---

## Real-World Example: Mid-Sized Mendix Application

**Example:** A typical mid-sized Mendix application

**Size:**
- 20-30 modules
- 100-200 entities
- 200-300 microflows
- 100-150 Java actions
- 100+ pages
- Dozens of constants
- Multiple published REST APIs

### Extraction Results

#### Minimal Profile

```bash
$ time semanticmesh export --input ./your-mendix-app --output minimal.zip --analyze-code

Extraction results:
- Profile: minimal
- Time: ~1.6s
- Items: 150-200 (modules + APIs + entities)
- Tables: modules, published_rest_services, published_rest_operations, entities
```

**Use case:** "Quick scan to see what external services this app depends on"

---

#### Standard Profile (Default)

```bash
$ time semanticmesh export --input ./your-mendix-app --output standard.zip --analyze-code

Extraction results:
- Profile: standard
- Time: ~2.3s
- Items: 500-1000 (modules + APIs + entities + microflows + Java actions + pages + constants)
- Tables: modules, published_rest_services, published_rest_operations, entities,
          microflows, java_actions, pages, constants
```

**Use case:** "Complete architectural analysis for documentation and impact assessment"

---

#### Comprehensive Profile

```bash
$ time semanticmesh export --input ./your-mendix-app --output comprehensive.zip --analyze-code

Extraction results:
- Profile: comprehensive
- Time: ~1.5s
- Items: 500-1000+ (includes internal dependencies)
- Tables: All Standard tables + module_dependencies, microflow_calls
```

**Use case:** "Refactoring planning—understanding module coupling and call graphs"

**Note:** Comprehensive profile is faster than Standard because it uses optimized batch queries.

---

## Use Cases for AI Agents

### 1. Impact Analysis

**Question:** "If the Customer entity schema changes, what breaks?"

**Query:**
```bash
semanticmesh query impact --component Customer --depth all --format json
```

**Result:**
```json
{
  "affected_components": [
    {"name": "ACT_GetCustomer", "type": "microflow", "distance": 1},
    {"name": "CustomerDetail", "type": "page", "distance": 1},
    {"name": "PRS_CustomerAPI", "type": "rest-api", "distance": 2}
  ]
}
```

**Agent interpretation:** "The Customer entity change affects 15 microflows (direct), 8 pages (direct), and 3 REST APIs (indirect). High risk—requires thorough testing."

---

### 2. API Surface Discovery

**Question:** "What REST APIs does this application expose?"

**Query:**
```bash
semanticmesh query list --type rest-api --format json
```

**Result:**
```json
{
  "components": [
    {
      "name": "PRS_OrderAPI",
      "type": "rest-api",
      "confidence": 0.95,
      "metadata": {
        "path": "rest/orders",
        "operations": 5
      }
    }
  ]
}
```

**Agent interpretation:** "This app exposes multiple REST APIs for order management, customer data, and webhooks."

---

### 3. Dependency Tracing

**Question:** "What does the OrderModule depend on?"

**Query:**
```bash
semanticmesh query dependencies --component OrderModule --format json
```

**Result:**
```json
{
  "dependencies": [
    {"name": "CustomerModule", "type": "module", "relationship": "module_reference"},
    {"name": "PaymentAPI", "type": "rest-api", "relationship": "rest_client"},
    {"name": "OrderDatabase", "type": "database", "relationship": "external_entity"}
  ]
}
```

**Agent interpretation:** "OrderModule depends on CustomerModule (internal), PaymentAPI (external REST), and OrderDatabase (external entity). If any of these fail, OrderModule breaks."

---

### 4. Refactoring Safety Check

**Question:** "Can I safely delete the LegacyModule?"

**Query:**
```bash
semanticmesh query impact --component LegacyModule --depth all --format json
```

**Result:**
```json
{
  "affected_components": []
}
```

**Agent interpretation:** "LegacyModule has zero incoming dependencies. Safe to delete."

---

## Performance Considerations

### Timing Breakdown (Typical Mendix App)

| Operation | Time | Percentage |
|-----------|------|------------|
| Catalog build | ~1.0-1.5s | 50-60% |
| Table extraction | ~0.5-1.0s | 25-35% |
| Component generation | ~0.2-0.5s | 10-20% |
| **Total** | **~2-3s** | **100%** |

**Optimization notes:**
- Catalog build is one-time cost (can be cached with `catalog_refresh: false`)
- Comprehensive profile is faster due to optimized batch queries
- Minimal profile skips Tier 2 extraction entirely

---

### Memory Usage

| Profile | Peak Memory | Catalog Size |
|---------|-------------|--------------|
| Minimal | ~80 MB | ~40 MB |
| Standard | ~120 MB | ~40 MB |
| Comprehensive | ~150 MB | ~40 MB |

**Note:** Catalog build memory is constant across profiles; extraction memory scales with items extracted.

---

### Catalog Build Time

| App Size | Modules | Catalog Build Time |
|----------|---------|-------------------|
| Small | <10 | ~0.5s |
| Medium | 10-30 | ~1.2s |
| Large | 30-50 | ~3-5s |
| Very Large | 50+ | ~5-10s |

**Optimization:** Set `catalog_refresh: false` for subsequent analyses if catalog hasn't changed.

---

## Troubleshooting

### Issue: "Cannot open MPR file"

**Cause:** MPR file is locked by Mendix Studio Pro or another process

**Solution:**
```bash
# Close Mendix Studio Pro first
# Then run analysis
semanticmesh export --input ./app --output app.zip --analyze-code
```

---

### Issue: "Extraction takes 30-60 seconds"

**Cause:** Large app (50+ modules) requires longer catalog build time

**Solution:**
```yaml
# Use Minimal profile for faster scans
code_analysis:
  mendix:
    extraction_profile: "minimal"
```

```bash
# Or cache the catalog
# First run: ~30s (builds catalog)
semanticmesh export --input ./app --output app.zip --analyze-code

# Subsequent runs: ~5s (catalog cached)
# Set catalog_refresh: false in config
```

---

### Issue: "Missing expected components"

**Cause 1:** Using Minimal profile (only extracts Tier 1)

**Solution:**
```yaml
code_analysis:
  mendix:
    extraction_profile: "standard"  # Or "comprehensive"
```

**Cause 2:** Catalog not refreshed after app changes

**Solution:**
```yaml
code_analysis:
  mendix:
    catalog_refresh: true  # Always refresh catalog
```

---

### Issue: "JSON output is too large"

**Cause:** Standard/Comprehensive profiles extract hundreds of items

**Solution:**
```yaml
# Use Minimal profile for smaller output
code_analysis:
  mendix:
    extraction_profile: "minimal"

# Or filter results after extraction
semanticmesh query list --type microflow --filter "module:OrderModule"
```

---

## Performance Tuning Tips

### 1. Profile Selection

**Fast scans (CI/CD):** Use Minimal profile
```yaml
code_analysis:
  mendix:
    extraction_profile: "minimal"
```

**Balanced analysis:** Use Standard profile (default)
```yaml
code_analysis:
  mendix:
    extraction_profile: "standard"
```

**Deep investigation:** Use Comprehensive profile
```yaml
code_analysis:
  mendix:
    extraction_profile: "comprehensive"
```

---

### 2. Catalog Caching

**First analysis:**
```yaml
code_analysis:
  mendix:
    catalog_refresh: true  # Build catalog (slower)
```

**Subsequent analyses (no app changes):**
```yaml
code_analysis:
  mendix:
    catalog_refresh: false  # Use cached catalog (faster)
```

**Speedup:** 50-70% reduction in analysis time

---

### 3. Selective Extraction

**Extract only what you need:**
```yaml
code_analysis:
  mendix:
    extract_published_apis: true
    extract_domain_model: true
    extract_business_logic: false  # Skip microflows/Java actions
    extract_ui_structure: false    # Skip pages
    extract_configuration: false   # Skip constants
```

**Result:** Faster extraction, smaller output

---

### 4. Parallel Analysis

**Analyze multiple apps in parallel:**
```bash
#!/bin/bash
# Analyze 3 Mendix apps in parallel
semanticmesh export --input ./app1 --output app1.zip --analyze-code &
semanticmesh export --input ./app2 --output app2.zip --analyze-code &
semanticmesh export --input ./app3 --output app3.zip --analyze-code &
wait

# Import all graphs
semanticmesh import app1.zip --name app1
semanticmesh import app2.zip --name app2
semanticmesh import app3.zip --name app3
```

**Speedup:** 3x faster for multi-app workspaces (assuming sufficient CPU cores)

---

## Integration Examples

### MCP Server Integration

**Start MCP server:**
```bash
semanticmesh mcp
```

**Query via MCP tools:**
```json
{
  "tool": "list_components",
  "arguments": {
    "component_type": "rest-api"
  }
}
```

**Response:**
```json
{
  "components": [
    {"name": "PRS_OrderAPI", "type": "rest-api", "confidence": 0.95},
    {"name": "PRS_CustomerAPI", "type": "rest-api", "confidence": 0.95},
    {"name": "PRS_Webhooks", "type": "rest-api", "confidence": 0.95}
  ]
}
```

---

### CI/CD Pipeline

```yaml
# .github/workflows/analyze-mendix.yml
name: Analyze Mendix Architecture

on: [push]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Install semanticmesh
        run: curl -fsSL https://raw.githubusercontent.com/vaibhav1805/semanticmesh/main/install.sh | bash
      
      - name: Analyze Mendix app
        run: |
          semanticmesh export --input . --output app.zip --analyze-code
          semanticmesh import app.zip --name ci-scan
          semanticmesh query list --format json > components.json
      
      - name: Check for new dependencies
        run: |
          diff components.json previous-components.json || exit 1
```

---

## Best Practices

### 1. Choose the Right Profile

- **Minimal:** CI/CD, quick scans, API surface discovery
- **Standard:** Architecture docs, impact analysis, change assessment
- **Comprehensive:** Refactoring, module coupling, technical debt

---

### 2. Cache Catalogs When Possible

- Set `catalog_refresh: false` for repeated analyses of unchanged apps
- Save 50-70% extraction time

---

### 3. Filter Results

- Use `semanticmesh query list --type microflow` to filter by component type
- Use `--filter "module:OrderModule"` to scope to a single module
- Reduce output size and improve query speed

---

### 4. Version Your Exports

```bash
# Tag exports with version
semanticmesh export --input . --output app-v1.0.zip --analyze-code --version "1.0"
semanticmesh import app-v1.0.zip --name app-v1.0

# Compare versions later
semanticmesh query list --graph app-v1.0 > v1.0-components.txt
semanticmesh query list --graph app-v2.0 > v2.0-components.txt
diff v1.0-components.txt v2.0-components.txt
```

---

### 5. Use Named Graphs for Environments

```bash
# Import production graph
semanticmesh import prod-app.zip --name prod

# Import staging graph
semanticmesh import staging-app.zip --name staging

# Compare environments
semanticmesh query list --graph prod --format json > prod.json
semanticmesh query list --graph staging --format json > staging.json
diff prod.json staging.json  # Find environment drift
```

---

## Appendix: Complete Configuration Reference

```yaml
# semanticmesh.yaml - Complete Mendix configuration

code_analysis:
  mendix:
    # Profile selection (recommended)
    extraction_profile: "standard"  # "minimal", "standard", or "comprehensive"
    
    # Fine-grained control (overrides profile)
    extract_published_apis: true     # Extract REST/OData APIs
    extract_domain_model: true       # Extract entities
    extract_business_logic: true     # Extract microflows/Java actions
    extract_ui_structure: true       # Extract pages
    extract_configuration: true      # Extract constants
    
    # Advanced options
    include_internal_deps: false     # Include module/microflow dependencies
    detect_modules_as_components: false  # Create component per module
    
    # Performance tuning
    catalog_refresh: true            # Build catalog before analysis

# Ignore patterns
ignore_patterns:
  - "*/deployment/"
  - "*/resources/"
  - "*/.mendix-cache/"
  - "*.mpr.lock"
  - "*.mpr.bak"

# Aliases (normalize component names)
aliases:
  YourMendixApp:
    - Your-App
    - your-app
    - YourApp
```

---

**Last updated:** 2026-04-14

**See also:**
- [CODE_ANALYSIS.md](CODE_ANALYSIS.md) - Code analysis patterns for all languages
- [CONFIGURATION.md](CONFIGURATION.md) - Configuration guide
- [CLI_REFERENCE.md](CLI_REFERENCE.md) - Command reference
