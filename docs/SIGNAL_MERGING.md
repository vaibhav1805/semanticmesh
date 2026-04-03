# Signal Merging

How semanticmesh combines detection signals from multiple sources to produce confidence scores on relationships.

---

## Source Types

Every relationship in the graph carries a `source_type` field indicating how it was discovered. This field is always present (never omitted) and takes one of three values:

| Value | Meaning |
|-------|---------|
| `markdown` | Detected from markdown document analysis only |
| `code` | Detected from source code analysis only |
| `both` | Detected independently by both markdown and code analysis |

The `source_type` field defaults to `"markdown"` for graphs created before code analysis was introduced.

---

## Detection Sources

### Markdown Analysis

Relationships are extracted from markdown documentation using multiple algorithms, each contributing signals at different confidence levels:

| Algorithm | Weight | Typical Confidence | What it detects |
|-----------|--------|--------------------|-----------------|
| `co-occurrence` | 0.3 | 0.4--0.6 | Component names appearing near each other in text |
| `NER` | 0.5 | 0.5--0.7 | Named Entity Recognition with Subject-Verb-Object extraction |
| `structural` | 0.6 | 0.6--0.8 | Heading hierarchies, dependency lists, tables |
| `semantic` | 0.7 | 0.5--0.75 | TF-IDF vector similarity between documents |
| `LLM` | 1.0 | 0.65 | LLM-inferred semantic relationships |
| `explicit-link` | -- | 1.0 | Direct markdown hyperlinks between documents |

Algorithm weights are used during signal aggregation when multiple markdown algorithms detect the same relationship.

### Code Analysis

Source code is parsed for dependency patterns:

- Import statements and package references
- Function calls to external services
- Connection strings and configuration references

All code-detected relationships use the `code-analysis` extraction method with algorithm weight `0.85`. Code signals carry the source file path, line number, and programming language for provenance.

---

## Multi-Source Confidence Merging

When the **same relationship** (same source component, target component, and edge type) is detected by both markdown analysis and code analysis, their confidence scores are merged using the **probabilistic OR formula**:

```
merged = 1.0 - (1.0 - markdown_confidence) * (1.0 - code_confidence)
```

This formula models the two sources as independent evidence. The merged confidence is always higher than either individual score, reflecting the increased certainty from corroboration.

### When merging applies

- The same source-target-type triple must be detected by **both** markdown and code signals
- If only one source type detected the relationship, the confidence is left unchanged
- The merged confidence is clamped to a maximum of 1.0

### Examples

| Markdown Confidence | Code Confidence | Merged Confidence | Explanation |
|--------------------|-----------------|-------------------|-------------|
| 0.6 | 0.7 | 0.88 | `1 - (0.4 * 0.3) = 0.88` |
| 0.5 | 0.5 | 0.75 | `1 - (0.5 * 0.5) = 0.75` |
| 0.8 | 0.9 | 0.98 | `1 - (0.2 * 0.1) = 0.98` |
| 0.99 | 0.99 | 0.9999 | Near-certain from both sources |
| 0.4 | 0.0 | 0.4 | Only markdown detected it; no merge |

### Effect on confidence tiers

Merging frequently promotes relationships to higher confidence tiers:

- A `moderate` (0.6) markdown edge corroborated by `moderate` (0.6) code evidence merges to `strong-inference` (0.84)
- A `weak` (0.5) + `weak` (0.5) merges to `strong-inference` (0.75)

---

## Integration Pipeline

During `semanticmesh export` and `semanticmesh crawl`, signal merging follows this sequence:

1. **Convert** code signals to graph edges, grouped by (source, target, type). For each group, the highest-confidence signal sets the edge confidence.

2. **Create stub nodes** for code-detected targets not already in the graph (prevents dangling edge references).

3. **Merge** code-discovered edges with markdown-discovered edges. Edges with matching source-target-type keys have their signal lists combined.

4. **Apply probabilistic OR** to merged edges that have signals from both markdown and code sources.

5. **Set source_type** on each edge based on which signal sources are present:
   - Only markdown signals: `"markdown"`
   - Only code signals: `"code"`
   - Both: `"both"`

6. **Add edges to graph** -- code-originated and dual-source edges are added to the graph structure.

---

## The --source-type Filter

Query commands accept `--source-type` to filter results by detection source. The filter semantics differ from exact matching:

| Filter Value | Matches edges with source_type |
|-------------|-------------------------------|
| `markdown` | `"markdown"` or `"both"` |
| `code` | `"code"` or `"both"` |
| `both` | `"both"` only |
| *(omitted)* | All edges |

The key insight: `--source-type markdown` includes `"both"` edges because markdown analysis was involved. Similarly, `--source-type code` includes `"both"` edges because code analysis was involved. Use `--source-type both` to find only relationships corroborated by both methods.

```bash
# All relationships where code analysis contributed
semanticmesh query impact --component payment-api --source-type code

# Only relationships confirmed by both sources
semanticmesh query dependencies --component auth-service --source-type both

# Markdown-detected relationships (including those also found in code)
semanticmesh query list --source-type markdown
```

---

## code_signals Provenance Table

Raw code analysis signals are persisted in the `code_signals` table for full provenance:

| Column | Type | Description |
|--------|------|-------------|
| `source_component` | TEXT | Component where the signal was detected |
| `target_component` | TEXT | Target component referenced in code |
| `signal_type` | TEXT | Detection kind (import, call, connection, etc.) |
| `confidence` | REAL | Signal confidence score |
| `evidence` | TEXT | Human-readable description of what was found |
| `file_path` | TEXT | Source file where the signal was detected |
| `line_number` | INTEGER | Line number in the source file |
| `language` | TEXT | Programming language of the source file |
| `created_at` | TEXT | Timestamp of detection |

This table preserves fine-grained signal details that are aggregated into the coarser graph edge representation. It enables "where exactly in the code was this detected?" queries.

---

## Edge Drop Warnings

During graph export, edges referencing components not present as graph nodes are dropped. When this occurs, a warning is printed to stderr:

```
  Warning: 3 edge(s) dropped (missing endpoint nodes)
  service-a -> unknown-target
  missing-source -> database-b
  ...
```

This can happen when:
- Code analysis detects a dependency on a component not found in documentation and stub node creation fails
- An edge references a component that was removed between detection and export

The warning is informational -- the graph is saved successfully with the remaining valid edges.

---

**Last updated:** 2026-04-03
