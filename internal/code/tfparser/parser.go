package tfparser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"github.com/vaibhav1805/semanticmesh/internal/code/connstring"
)

// Compile-time checks that TerraformParser implements both required interfaces.
var _ code.LanguageParser = (*TerraformParser)(nil)
var _ code.ManifestAnalyzer = (*TerraformParser)(nil)

// TerraformParser analyzes Terraform (.tf) source files for infrastructure
// dependency signals using regex-based pattern matching.
type TerraformParser struct{}

// NewTerraformParser creates a TerraformParser with the default detection patterns.
func NewTerraformParser() *TerraformParser { return &TerraformParser{} }

// Name returns "terraform".
func (p *TerraformParser) Name() string { return "terraform" }

// Extensions returns [".tf"].
func (p *TerraformParser) Extensions() []string { return []string{".tf"} }

// Block header regexes — match the opening line of HCL blocks.
var (
	// resourceBlockRe matches: resource "aws_db_instance" "prod_db" {
	resourceBlockRe = regexp.MustCompile(`^\s*resource\s+"([^"]+)"\s+"([^"]+)"`)

	// moduleBlockRe matches: module "rds" {
	moduleBlockRe = regexp.MustCompile(`^\s*module\s+"([^"]+)"`)

	// dataBlockRe matches: data "aws_db_instance" "existing" {
	dataBlockRe = regexp.MustCompile(`^\s*data\s+"([^"]+)"\s+"([^"]+)"`)
)

// Attribute regexes — used inside blocks.
var (
	// sourceAttrRe matches: source = "terraform-aws-modules/rds/aws"
	sourceAttrRe = regexp.MustCompile(`^\s*source\s*=\s*"([^"]+)"`)

	// dependsOnInlineRe matches single-line: depends_on = [aws_db_instance.main]
	dependsOnInlineRe = regexp.MustCompile(`^\s*depends_on\s*=\s*\[([^\]]+)\]`)

	// dependsOnOpenRe matches the start of a multi-line depends_on block.
	dependsOnOpenRe = regexp.MustCompile(`^\s*depends_on\s*=\s*\[`)

	// resourceRefRe matches resource address references: aws_db_instance.label
	// Captures: group1 = resource type, group2 = label
	resourceRefRe = regexp.MustCompile(`\b((?:aws|kubernetes|helm)_\w+)\.(\w+)\b`)

	// tfvarsValueRe extracts quoted values from .tfvars assignment lines.
	tfvarsValueRe = regexp.MustCompile(`=\s*"([^"]+)"`)
)

// resourceRef holds a parsed Terraform resource address (type + label).
type resourceRef struct {
	resType string
	label   string
}

// ParseFile analyzes a .tf file and returns detected infrastructure signals.
// Returns nil, nil for files inside the .terraform/ vendor directory.
func (p *TerraformParser) ParseFile(filePath string, content []byte) ([]code.CodeSignal, error) {
	// Skip vendored provider downloads (.terraform/ directory).
	// Match both absolute paths (/.terraform/) and relative paths (.terraform/).
	if strings.Contains(filePath, "/.terraform/") ||
		strings.HasPrefix(filePath, ".terraform/") ||
		strings.Contains(filePath, `\.terraform\`) {
		return nil, nil
	}

	lines := strings.Split(string(content), "\n")
	var signals []code.CodeSignal

	// seen deduplicates signals by "kind:target" to avoid emitting the same
	// interpolation reference multiple times within one file.
	seen := make(map[string]bool)

	emit := func(target, targetType, kind string, lineNum int, confidence float64) {
		key := kind + ":" + target
		if seen[key] {
			return
		}
		seen[key] = true
		signals = append(signals, code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: target,
			TargetType:      targetType,
			DetectionKind:   kind,
			Evidence:        evidenceSnippet(lines, lineNum),
			Language:        "terraform",
			Confidence:      confidence,
		})
	}

	// blockCtx holds state for the currently active HCL block.
	type blockCtx struct {
		kind    string // "resource", "module", or "data"
		resType string // e.g., "aws_db_instance"
		label   string // e.g., "prod_db"
	}

	var (
		pending      *blockCtx // header seen, waiting for opening brace
		current      *blockCtx // inside a block
		braceDepth   int
		inDependsOn  bool // inside a multi-line depends_on = [...] list
	)

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}

		opens, closes := countBraces(trimmed)
		prevDepth := braceDepth
		braceDepth += opens - closes

		// ── Handle continuation of a multi-line depends_on = [...] ──────────
		if inDependsOn {
			if strings.Contains(trimmed, "]") {
				inDependsOn = false
			}
			for _, ref := range extractResourceRefs(trimmed) {
				if pat, ok := lookupResourcePattern(ref.resType); ok {
					emit(ref.resType+"."+ref.label, pat.TargetType, "terraform_dependency", lineNum, 0.90)
				}
			}
			continue
		}

		// ── Activate a pending block when its opening brace arrives ─────────
		// This handles the (rare) case where `{` is on its own line after the header.
		if pending != nil && prevDepth == 0 && braceDepth > 0 {
			current = pending
			pending = nil
		}

		// ── Detect top-level block headers ───────────────────────────────────
		if prevDepth == 0 {
			if m := resourceBlockRe.FindStringSubmatch(trimmed); m != nil {
				resType, label := m[1], m[2]
				if pat, ok := lookupResourcePattern(resType); ok {
					emit(resType+"."+label, pat.TargetType, "terraform_resource", lineNum, pat.Confidence)
				}
				ctx := &blockCtx{kind: "resource", resType: resType, label: label}
				if opens > 0 {
					current = ctx
				} else {
					pending = ctx
				}
				continue
			}

			if m := moduleBlockRe.FindStringSubmatch(trimmed); m != nil {
				ctx := &blockCtx{kind: "module", label: m[1]}
				if opens > 0 {
					current = ctx
				} else {
					pending = ctx
				}
				continue
			}

			if m := dataBlockRe.FindStringSubmatch(trimmed); m != nil {
				resType, label := m[1], m[2]
				if pat, ok := lookupResourcePattern(resType); ok {
					// Data sources reference existing infrastructure — lower confidence.
					emit(resType+"."+label, pat.TargetType, "terraform_data_source", lineNum, pat.Confidence*0.85)
				}
				ctx := &blockCtx{kind: "data", resType: resType, label: label}
				if opens > 0 {
					current = ctx
				} else {
					pending = ctx
				}
				continue
			}
		}

		// ── Process lines inside the current top-level block ────────────────────
		if current != nil && braceDepth >= 1 {
			switch current.kind {
			case "module":
				// source attribute is always at depth 1 (direct child of module block)
				if braceDepth == 1 {
					if m := sourceAttrRe.FindStringSubmatch(trimmed); m != nil {
						source := m[1]
						targetType, confidence := "service", 0.60
						if pat, ok := lookupModulePattern(source); ok {
							targetType, confidence = pat.TargetType, 0.80
						}
						emit(current.label, targetType, "terraform_module", lineNum, confidence)
					}
				}

			case "resource", "data":
				// depends_on is a direct attribute (depth 1 only)
				if braceDepth == 1 {
					if m := dependsOnInlineRe.FindStringSubmatch(trimmed); m != nil {
						for _, ref := range extractResourceRefs(m[1]) {
							if pat, ok := lookupResourcePattern(ref.resType); ok {
								emit(ref.resType+"."+ref.label, pat.TargetType, "terraform_dependency", lineNum, 0.90)
							}
						}
						continue
					}
					if dependsOnOpenRe.MatchString(trimmed) && !strings.Contains(trimmed, "]") {
						inDependsOn = true
						continue
					}
				}

				// Attribute interpolations can appear at any nesting depth
				// (e.g., inside environment { variables = { ... } } blocks).
				selfTarget := current.resType + "." + current.label
				for _, ref := range extractResourceRefs(trimmed) {
					target := ref.resType + "." + ref.label
					if target == selfTarget {
						continue // skip self-reference
					}
					if pat, ok := lookupResourcePattern(ref.resType); ok {
						emit(target, pat.TargetType, "terraform_ref", lineNum, 0.75)
					}
				}
			}
		}

		// ── Exit block when brace depth returns to zero ───────────────────────
		if braceDepth == 0 && current != nil {
			current = nil
		}
	}

	return signals, nil
}

// AnalyzeManifests implements code.ManifestAnalyzer by scanning *.tfvars files
// for connection string values (database URLs, hostnames, etc.).
func (p *TerraformParser) AnalyzeManifests(dir string) ([]code.CodeSignal, error) {
	var signals []code.CodeSignal

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".tfvars") && !strings.HasSuffix(path, ".tfvars.json") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files
		}

		for lineIdx, line := range strings.Split(string(data), "\n") {
			lineNum := lineIdx + 1
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			m := tfvarsValueRe.FindStringSubmatch(trimmed)
			if m == nil {
				continue
			}

			result, ok := connstring.Parse(m[1])
			if !ok || result.Host == "" {
				continue
			}

			signals = append(signals, code.CodeSignal{
				SourceFile:      path,
				LineNumber:      lineNum,
				TargetComponent: result.Host,
				TargetType:      result.TargetType,
				DetectionKind:   "env_var_ref",
				Evidence:        trimmed,
				Language:        "terraform",
				Confidence:      0.65,
			})
		}
		return nil
	})

	return signals, err
}

// countBraces counts unquoted, uncommented `{` and `}` characters on a single line.
// It skips characters inside double-quoted strings and stops at `#` or `//` comments.
func countBraces(line string) (opens, closes int) {
	inStr := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inStr {
			if c == '"' && (i == 0 || line[i-1] != '\\') {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			opens++
		case '}':
			closes++
		case '#':
			return opens, closes
		case '/':
			if i+1 < len(line) && line[i+1] == '/' {
				return opens, closes
			}
		}
	}
	return opens, closes
}

// extractResourceRefs finds all Terraform resource address references (type.label)
// in a string. Matches aws_*, kubernetes_*, and helm_* prefixes.
func extractResourceRefs(s string) []resourceRef {
	matches := resourceRefRe.FindAllStringSubmatch(s, -1)
	refs := make([]resourceRef, 0, len(matches))
	for _, m := range matches {
		refs = append(refs, resourceRef{resType: m[1], label: m[2]})
	}
	return refs
}

// evidenceSnippet returns the source line at lineNum (1-based), trimmed to 200 chars.
func evidenceSnippet(lines []string, lineNum int) string {
	if lineNum < 1 || lineNum > len(lines) {
		return ""
	}
	line := strings.TrimSpace(lines[lineNum-1])
	if len(line) > 200 {
		line = line[:200]
	}
	return line
}
