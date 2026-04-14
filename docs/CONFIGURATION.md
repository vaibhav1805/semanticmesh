# Configuration Guide

This guide covers all configuration mechanisms for semanticmesh: `.semanticmeshignore` for excluding files, `semanticmesh-aliases.yaml` for component name resolution, seed configuration for type customization, and named graph management.

---

## .semanticmeshignore

The `.semanticmeshignore` file controls which files and directories are excluded from scanning during `export` and `crawl`. Place it in the root of the directory being scanned.

### File Format

- One pattern per line
- Lines starting with `#` are comments
- Blank lines are skipped
- Patterns ending with `/` match directories only (trailing slash is stripped during matching)
- All other patterns match files
- Supports glob wildcards: `*`, `?`, `[abc]`

### Example

```gitignore
# .semanticmeshignore

# Directories
vendor/
node_modules/
.git/
dist/
build/
.bmd/

# Files
*.lock
temp_*
```

### Default Patterns

When no `.semanticmeshignore` file exists, semanticmesh applies these default directory exclusions:

```
vendor
node_modules
.git
__pycache__
.venv
dist
build
target
.gradle
.next
out
.cache
bin
obj
.bmd
.planning
```

When a `.semanticmeshignore` file is present, only its patterns are used (defaults are not merged).

---

## semanticmesh-aliases.yaml

The alias configuration file maps multiple names for the same component to a single canonical identity. This prevents duplicate nodes when documentation and code refer to the same component by different names.

Place `semanticmesh-aliases.yaml` in the root of the directory being scanned.

### File Format

```yaml
# semanticmesh-aliases.yaml
aliases:
  canonical-name:
    - alias-1
    - alias-2
    - alias-3
```

Each key is the canonical component name. Its values are alternate names that should resolve to the canonical name during graph building.

### Example

```yaml
# semanticmesh-aliases.yaml
aliases:
  postgres-primary:
    - pg-main
    - primary-db
    - pgdb
    - main-database

  redis-cache:
    - redis
    - cache-layer
    - session-store

  api-gateway:
    - gateway
    - api-gw
    - ingress
```

### How Aliases Are Applied

During `export` and `crawl`, semanticmesh loads the alias file and resolves node IDs and edge endpoints:

1. If a node ID matches an alias, the node is renamed to the canonical name
2. If an edge source or target matches an alias, the endpoint is updated to the canonical name
3. Matching is case-sensitive
4. If the canonical name already exists as a node, the alias node is not merged (to avoid data loss)

The number of aliases applied is reported in the export summary and stored in `metadata.json`.

### Use Cases

- **Documentation vs. code names:** Your docs call it "primary-db" but your code connects to "postgres-primary"
- **Abbreviations:** Teams use shorthand ("pg", "redis") while documentation uses full names
- **Legacy names:** Old documentation references outdated component names

---

## Seed Configuration

Seed config overrides auto-detected component types without modifying code. It allows you to define custom types, fix misclassifications, and apply tags.

### Key Principles

- **Always wins:** Seed config mappings have precedence over auto-detection (confidence 1.0)
- **Glob patterns:** Match components by name patterns, not exact matches
- **No code changes:** Customize your taxonomy entirely through YAML files

### File Format

```yaml
# custom_types.yaml
seed_mappings:
  - pattern: "COMPONENT_PATTERN"
    type: "TYPE"
    tags:
      - "tag1"
      - "tag2"
    confidence_override: 1.0  # optional; always 1.0 if omitted
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pattern` | string | Yes | Glob pattern matching component names. Examples: `postgres*`, `services/api-*`, `*-db` |
| `type` | string | Yes | Component type to assign. May be a core type or custom type. |
| `tags` | array | No | Tags to apply to matched components. |
| `confidence_override` | float | No | Confidence score (default: 1.0). |

### Pattern Matching

Patterns use standard glob syntax:

- `*` matches any sequence of characters (except `/`)
- `?` matches a single character
- `[abc]` matches one character in the set
- `/` is the path separator for folder-based matching

```yaml
# Match by prefix
- pattern: "redis*"
  type: "cache"

# Match by suffix
- pattern: "*-db"
  type: "database"

# Match by folder
- pattern: "services/*"
  type: "service"

# Exact match
- pattern: "prometheus"
  type: "monitoring"
```

### Loading Seed Config

Pass the seed config file when indexing:

```bash
semanticmesh index --dir ./docs --seed-config ./custom_types.yaml
```

### Complete Example

```yaml
# organization_types.yaml
seed_mappings:
  # Override misclassification
  - pattern: "helper-service"
    type: "cache"
    tags: ["internal", "utility"]

  # Custom type for ML infrastructure
  - pattern: "ml-platform/*"
    type: "ml-service"
    tags: ["ml-team", "experimental"]

  # Tag production databases as critical
  - pattern: "databases/prod-*"
    type: "database"
    tags: ["production", "critical"]

  # Legacy components
  - pattern: "svc-*"
    type: "service"
    tags: ["legacy"]
```

### Verifying Seed Config

After indexing with seed config, components with `confidence = 1.0` were matched by seed config:

```bash
sqlite3 .bmd/knowledge.db "
  SELECT name, component_type, confidence
  FROM graph_nodes
  WHERE confidence = 1.0
  ORDER BY name;
"
```

---

## Named Graph Management

semanticmesh supports multiple named graphs in persistent storage. This allows you to import graphs from different environments or versions and query them independently.

### Storage Location

Graphs are stored under XDG-compliant paths:

- Linux/macOS: `$XDG_DATA_HOME/semanticmesh/` (default: `~/.local/share/semanticmesh/`)
- Each named graph gets its own subdirectory containing `graph.db` and `metadata.json`

### Importing Named Graphs

```bash
# Name derived from filename
semanticmesh import prod-graph.zip
# Stored as: ~/.local/share/semanticmesh/prod-graph/

# Explicit name
semanticmesh import graph.zip --name prod-infra
# Stored as: ~/.local/share/semanticmesh/prod-infra/

# Import a second graph
semanticmesh import graph.zip --name staging-infra
```

The most recently imported graph becomes the default for queries.

### Querying Named Graphs

Use `--graph` on any query command to select a specific graph:

```bash
# Query the default (most recent) graph
semanticmesh query impact --component primary-db

# Query a specific named graph
semanticmesh query impact --component primary-db --graph prod-infra
semanticmesh query list --type service --graph staging-infra
```

The `--graph` flag works on all query subcommands: `impact`, `dependencies`, `path`, and `list`.

### Replacing a Graph

Importing with the same name as an existing graph replaces it:

```bash
semanticmesh import updated-graph.zip --name prod-infra
# "Replacing existing graph "prod-infra""
```

---

## Mendix Analysis Configuration

The `semanticmesh.yaml` file controls Mendix-specific analysis settings when using the `--analyze-code` flag. Mendix extraction uses configurable profiles to balance speed and depth: Minimal (fast CI/CD scans), Standard (recommended for analysis), or Comprehensive (deep investigation).

Place `semanticmesh.yaml` in the root of your workspace or Mendix project directory.

**Note:** semanticmesh uses the mxcli Go library bundled as a module dependency—no external binary installation required.

### File Format

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extraction_profile: "standard"  # minimal, standard, or comprehensive
    extract_published_apis: true
    extract_domain_model: true
    extract_business_logic: true
    extract_ui_structure: true
    extract_configuration: true
    include_internal_deps: false
    detect_modules_as_components: false
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `extraction_profile` | string | `"standard"` | Profile: `minimal`, `standard`, or `comprehensive` |
| `extract_published_apis` | bool | `true` | Extract REST/OData APIs this app exposes |
| `extract_domain_model` | bool | `true` | Extract entities and attributes |
| `extract_business_logic` | bool | `true` | Extract microflows and Java actions |
| `extract_ui_structure` | bool | `true` | Extract pages and navigation |
| `extract_configuration` | bool | `true` | Extract constants and settings |
| `include_internal_deps` | bool | `false` | Include module and microflow dependencies |
| `detect_modules_as_components` | bool | `false` | Create component for each module |
| ~~`enabled`~~ | boolean | `true` | **Legacy:** Use `code_analysis.enabled` instead |
| ~~`catalog_refresh`~~ | boolean | `true` | **Legacy:** Always enabled in new profiles |
| ~~`mxcli_path`~~ | string | N/A | **Deprecated:** mxcli is now a Go module dependency |

### Profile Presets

#### Minimal Profile (~1.6s)

Extracts only Tier 1: external dependencies and published APIs.

**What gets extracted:**
- Modules (20-30 in typical apps)
- Published REST services (multiple per app)
- Published REST operations
- Entities (100-200 in typical apps)

**Tables:** `modules`, `published_rest_services`, `published_rest_operations`, `entities`

**Use for:**
- CI/CD pipelines requiring fast scans
- Quick dependency checks ("what does this app depend on?")
- Initial architecture discovery
- High-frequency automated scans

**Performance:** ~1-2 seconds for 150-200 items

#### Standard Profile (~2.3s, default)

Extracts Tier 1 + Tier 2: business logic and UI structure.

**What gets extracted:**
- All Minimal profile items
- Microflows (200-500 in typical apps)
- Java Actions (100-200 in typical apps)
- Pages (100+ in typical apps)
- Constants (dozens to hundreds)

**Tables:** All Minimal tables + `microflows`, `java_actions`, `pages`, `constants`

**Use for:**
- Impact analysis ("if X fails, what breaks?")
- Architecture documentation
- Service dependency mapping
- Change impact assessment
- Most day-to-day analysis tasks

**Performance:** ~2-3 seconds for 500-1000 items

#### Comprehensive Profile (~1.5s)

Extracts all tiers + internal dependencies (module-to-module, microflow calls).

**What gets extracted:**
- All Standard profile items
- Module dependencies (cross-module references)
- Microflow call graphs (caller/callee relationships)

**Tables:** All Standard tables + `module_dependencies`, `microflow_calls`

**Use for:**
- Complete architectural analysis
- Refactoring planning (understanding module coupling)
- Deep investigation of internal structure
- Module boundary analysis
- Technical debt assessment

**Performance:** ~1-2 seconds for 500-1000+ items (optimized catalog queries)

### Configuration Examples

#### Minimal Profile - Fast CI/CD Scans

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extraction_profile: "minimal"
```

**Result:** Extracts 150-200 items (modules, published APIs, entities) in ~1-2s

**Use case:** "Does this app depend on any external services? What APIs does it expose?"

#### Standard Profile - Recommended for Analysis

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extraction_profile: "standard"
```

**Result:** Extracts 500-1000 items (modules, APIs, entities, microflows, Java actions, pages, constants) in ~2-3s

**Use case:** "If the Customer entity changes, which microflows and pages are affected?"

#### Comprehensive Profile - Deep Investigation

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extraction_profile: "comprehensive"
    include_internal_deps: true
```

**Result:** Extracts 500-1000+ items with internal dependencies in ~1-2s

**Use case:** "Which modules depend on the CustomerModule? What's the call graph for ProcessOrder microflow?"

#### Custom Profile - Fine-Grained Control

```yaml
# semanticmesh.yaml
code_analysis:
  mendix:
    extract_published_apis: true    # Include APIs
    extract_domain_model: true      # Include entities
    extract_business_logic: false   # Skip microflows/Java actions
    extract_ui_structure: false     # Skip pages
    extract_configuration: false    # Skip constants
```

**Result:** Custom extraction (only APIs + entities)

**Use case:** "I only care about the data model and API surface—skip the rest"

#### Legacy Configuration (Still Supported)

```yaml
# semanticmesh.yaml (legacy format)
mendix:
  enabled: true
  catalog_refresh: true
  include_internal_deps: false
  detect_modules_as_components: false
```

**Migration:** Use `extraction_profile: "standard"` instead for better control

### Performance Impact

| Profile | Time | Items | Tables | Memory | Use Case |
|---------|------|-------|--------|--------|----------|
| Minimal | ~1-2s | 150-200 | 4 | Low | CI/CD, quick scans |
| Standard | ~2-3s | 500-1000 | 8 | Medium | Impact analysis, docs |
| Comprehensive | ~1-2s | 500-1000+ | 10+ | Medium | Refactoring, deep investigation |

**Example:** Typical mid-sized Mendix app (20-30 modules, 100-200 entities, 200-300 microflows, multiple REST APIs)

**Note:** Comprehensive profile is faster due to optimized catalog queries—it performs fewer total queries by batch-fetching related data.

### Combining with Other Configuration

```yaml
# Complete workspace configuration
code_analysis:
  mendix:
    extraction_profile: "standard"
    detect_modules_as_components: false
  
  enabled: true
  skip_test_files: true

ignore_patterns:
  - "*/deployment/"
  - "*/resources/"
  - "*/.mendix-cache/"
```

### Mendix-Specific Aliases

When analyzing Mendix apps alongside other services, use aliases to normalize component names:

```yaml
# semanticmesh-aliases.yaml
aliases:
  backend-service:
    - MendixBackend
    - Backend
    - backend-api
  
  frontend-app:
    - MendixFrontend
    - Frontend
    - frontend-web
  
  # Normalize module names (when detect_modules_as_components: true)
  customer-module:
    - CustomerManagement
    - CustomerModule
    - Customers
```

### Mendix-Specific Ignore Patterns

Add Mendix-specific directories to `.semanticmeshignore`:

```gitignore
# .semanticmeshignore

# Mendix deployment artifacts
deployment/
javasource/
resources/
userlib/
.mendix-cache/

# Mendix version control
.svn/

# Mendix temporary files
*.mpr.lock
*.mpr.bak
```

### Environment-Specific Configuration

For CI/CD pipelines or different environments:

```yaml
# production.yaml
mendix:
  enabled: true
  catalog_refresh: true
  include_internal_deps: false

# development.yaml
mendix:
  enabled: true
  catalog_refresh: false  # Faster during development
  include_internal_deps: true  # More detail for debugging
```

Use with:

```bash
# Production export
semanticmesh export --input . --output prod.zip --analyze-code --config production.yaml

# Development export
semanticmesh export --input . --output dev.zip --analyze-code --config development.yaml
```

---

## Best Practices

### 1. Start with Auto-Detection

Run semanticmesh without seed config first to see what auto-detection achieves:

```bash
semanticmesh crawl --input ./docs --format json
```

Then create seed config for the gaps.

### 2. Use Aliases for Name Consistency

If your docs and code use different names for the same component, define aliases rather than renaming things in your documentation:

```yaml
aliases:
  postgres-primary:
    - pg-main
    - primary-db
```

### 3. Keep .semanticmeshignore Updated

Exclude directories that generate noise (build output, dependencies, caches) to keep the graph focused on real infrastructure documentation.

### 4. Version Your Exports

Use `--git-version` or `--version` to tag exports with meaningful versions:

```bash
semanticmesh export --input ./docs --output graph.zip --git-version
```

### 5. Use Named Graphs for Environments

Import different environments as separate named graphs:

```bash
semanticmesh import prod-graph.zip --name prod
semanticmesh import staging-graph.zip --name staging
semanticmesh query list --graph prod --type database
```

---

## Troubleshooting

### Pattern Not Matching in .semanticmeshignore

- Patterns are case-sensitive
- Directory patterns must end with `/`
- File glob patterns (e.g., `*.lock`) do not cross directory boundaries

### Aliases Not Being Applied

- Check that `semanticmesh-aliases.yaml` is in the root of the scanned directory
- Alias matching is case-sensitive
- If the canonical name already exists as a node, the alias node is not renamed

### Seed Config Pattern Not Matching

- Patterns are case-sensitive
- `*` does not cross `/` boundaries — use folder prefixes for nested components
- Check the confidence column: `1.0` confirms seed config matched

### No Graph Found When Querying

If queries return `NO_GRAPH`, import a graph first:

```bash
semanticmesh export --input ./docs --output graph.zip
semanticmesh import graph.zip --name my-graph
semanticmesh query list
```

---

**Last updated:** 2026-04-03
