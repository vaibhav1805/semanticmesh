package jsparser

import (
	"strings"

	"github.com/semanticmesh/semanticmesh/internal/code"
	"github.com/semanticmesh/semanticmesh/internal/code/comments"
	"github.com/semanticmesh/semanticmesh/internal/code/connstring"
)

// Compile-time check that JSParser implements code.LanguageParser.
var _ code.LanguageParser = (*JSParser)(nil)

// JSParser analyzes JavaScript and TypeScript source files for infrastructure
// dependency signals using regex-based pattern matching.
type JSParser struct {
	// packagePatterns maps "package.function" -> pattern for method calls
	packagePatterns map[string]JSDetectionPattern

	// constructorPatterns maps "package.Constructor" -> pattern for new X() calls
	constructorPatterns map[string]JSDetectionPattern

	// bareCallPatterns maps "function" -> pattern for calls with no package prefix
	bareCallPatterns map[string]JSDetectionPattern

	// globalBarePatterns are patterns that fire without any import (e.g., fetch)
	globalBarePatterns []JSDetectionPattern
}

// NewJSParser creates a JSParser with the default detection patterns.
func NewJSParser() *JSParser {
	p := &JSParser{
		packagePatterns:     make(map[string]JSDetectionPattern),
		constructorPatterns: make(map[string]JSDetectionPattern),
		bareCallPatterns:    make(map[string]JSDetectionPattern),
	}

	for _, pat := range DefaultJSPatterns {
		if pat.IsBareCall && pat.Package == "" {
			// Global bare call (like fetch) — always active
			p.globalBarePatterns = append(p.globalBarePatterns, pat)
		} else if pat.IsConstructor {
			key := pat.Package + "." + pat.Function
			p.constructorPatterns[key] = pat
		} else {
			key := pat.Package + "." + pat.Function
			p.packagePatterns[key] = pat
		}
	}

	return p
}

// Name returns "javascript".
func (p *JSParser) Name() string { return "javascript" }

// Extensions returns JS/TS file extensions.
func (p *JSParser) Extensions() []string {
	return []string{".js", ".ts", ".jsx", ".tsx"}
}

// isTestFile returns true if the filename matches JS/TS test file patterns.
func isTestFile(filePath string) bool {
	// Check common test patterns: *.test.*, *.spec.*
	base := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		base = filePath[idx+1:]
	}
	if idx := strings.LastIndex(base, "\\"); idx >= 0 {
		base = base[idx+1:]
	}

	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
}

// ParseFile analyzes a JS/TS source file and returns detected infrastructure signals.
// Returns nil, nil for test files.
func (p *JSParser) ParseFile(filePath string, content []byte) ([]code.CodeSignal, error) {
	if isTestFile(filePath) {
		return nil, nil
	}

	lines := strings.Split(string(content), "\n")

	// First pass: build importMap from import/require statements.
	// Maps local name -> package name.
	importMap := buildJSImportMap(lines)

	var signals []code.CodeSignal
	inBlockComment := false

	for lineIdx, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := lineIdx + 1

		// Handle block comments
		if inBlockComment {
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				inBlockComment = false
				// Process the rest of the line after */
				trimmed = strings.TrimSpace(trimmed[idx+2:])
				if trimmed == "" {
					continue
				}
			} else {
				continue
			}
		}

		// Check for block comment start
		if idx := strings.Index(trimmed, "/*"); idx >= 0 {
			// Check if block comment closes on same line
			closeIdx := strings.Index(trimmed[idx+2:], "*/")
			if closeIdx >= 0 {
				// Remove the block comment from the line
				before := trimmed[:idx]
				after := trimmed[idx+2+closeIdx+2:]
				trimmed = strings.TrimSpace(before + " " + after)
				if trimmed == "" {
					continue
				}
			} else {
				inBlockComment = true
				// Process only the part before /*
				trimmed = strings.TrimSpace(trimmed[:idx])
				if trimmed == "" {
					continue
				}
			}
		}

		// Skip single-line comments (handled by shared comments.Analyze below)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Skip import/require lines (no signals from imports alone)
		if isImportLine(trimmed) {
			continue
		}

		// Match global bare calls (fetch)
		for _, pat := range p.globalBarePatterns {
			if matches := bareCallRe.FindAllStringSubmatch(trimmed, -1); matches != nil {
				for _, m := range matches {
					if m[1] == pat.Function {
						target := extractTargetFromLine(trimmed, pat.ArgIndex)
						signals = append(signals, code.CodeSignal{
							LineNumber:      lineNum,
							TargetComponent: target,
							TargetType:      pat.TargetType,
							DetectionKind:   pat.Kind,
							Evidence:        evidenceSnippet(lines, lineNum),
							Language:        "javascript",
							Confidence:      pat.Confidence,
						})
					}
				}
			}
		}

		// Match constructor calls: new Constructor(...)
		if ctorMatches := constructorCallRe.FindAllStringSubmatch(trimmed, -1); ctorMatches != nil {
			for _, m := range ctorMatches {
				ctorName := m[1]
				// Look up the constructor in importMap
				if pkg, ok := importMap[ctorName]; ok {
					key := pkg + "." + ctorName
					if pat, ok := p.constructorPatterns[key]; ok {
						target := extractTargetFromLine(trimmed, pat.ArgIndex)
						if target == "" {
							target = pkg
						}
						signals = append(signals, code.CodeSignal{
							LineNumber:      lineNum,
							TargetComponent: target,
							TargetType:      pat.TargetType,
							DetectionKind:   pat.Kind,
							Evidence:        evidenceSnippet(lines, lineNum),
							Language:        "javascript",
							Confidence:      pat.Confidence,
						})
					}
				}
			}
		}

		// Match package.method() calls
		if methodMatches := packageMethodCallRe.FindAllStringSubmatch(trimmed, -1); methodMatches != nil {
			for _, m := range methodMatches {
				pkgAlias := m[1]
				methodName := m[2]

				// Resolve alias to package
				if pkg, ok := importMap[pkgAlias]; ok {
					key := pkg + "." + methodName
					if pat, ok := p.packagePatterns[key]; ok {
						target := extractTargetFromLine(trimmed, pat.ArgIndex)
						if target == "" {
							target = pkg
						}
						signals = append(signals, code.CodeSignal{
							LineNumber:      lineNum,
							TargetComponent: target,
							TargetType:      pat.TargetType,
							DetectionKind:   pat.Kind,
							Evidence:        evidenceSnippet(lines, lineNum),
							Language:        "javascript",
							Confidence:      pat.Confidence,
						})
					}
				}
			}
		}

		// Match bare function calls from destructured imports (e.g., get() from axios)
		if bareMatches := bareCallRe.FindAllStringSubmatch(trimmed, -1); bareMatches != nil {
			for _, m := range bareMatches {
				funcName := m[1]
				// Skip already matched constructors (new X()) and global patterns
				if constructorCallRe.MatchString(trimmed) && funcName == constructorCallRe.FindStringSubmatch(trimmed)[1] {
					continue
				}
				isGlobal := false
				for _, gp := range p.globalBarePatterns {
					if funcName == gp.Function {
						isGlobal = true
						break
					}
				}
				if isGlobal {
					continue
				}

				// Check if this bare function came from a destructured import
				if pkg, ok := importMap[funcName]; ok {
					// Try as package method pattern (destructured method used as bare call)
					key := pkg + "." + funcName
					if pat, ok := p.packagePatterns[key]; ok {
						target := extractTargetFromLine(trimmed, pat.ArgIndex)
						if target == "" {
							target = pkg
						}
						signals = append(signals, code.CodeSignal{
							LineNumber:      lineNum,
							TargetComponent: target,
							TargetType:      pat.TargetType,
							DetectionKind:   pat.Kind,
							Evidence:        evidenceSnippet(lines, lineNum),
							Language:        "javascript",
							Confidence:      pat.Confidence,
						})
					}
				}
			}
		}
	}

	// Scan comments using shared comment analyzer
	commentSignals := comments.Analyze(lines, comments.SyntaxJavaScript, nil)
	for i := range commentSignals {
		commentSignals[i].Language = "javascript"
		commentSignals[i].SourceFile = filePath
	}
	signals = append(signals, commentSignals...)

	// Scan for env var references
	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		refs := connstring.ParseEnvVarRef(line)
		for _, ref := range refs {
			if !connstring.IsConnectionEnvVar(ref.Name) {
				continue
			}
			targetType := inferEnvVarTargetType(ref.Name)
			signals = append(signals, code.CodeSignal{
				LineNumber:      lineNum,
				TargetComponent: ref.Name,
				TargetType:      targetType,
				DetectionKind:   "env_var_ref",
				Evidence:        evidenceSnippet(lines, lineNum),
				Language:        "javascript",
				Confidence:      0.7,
			})
		}
	}

	return signals, nil
}

// buildJSImportMap parses import/require statements and returns a map of
// local name -> package name.
func buildJSImportMap(lines []string) map[string]string {
	m := make(map[string]string)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// ESM default: import axios from 'axios'
		if matches := esmDefaultRe.FindStringSubmatch(trimmed); matches != nil {
			localName := matches[1]
			pkg := matches[2]
			m[localName] = pkg
			continue
		}

		// ESM named: import { Pool, Client } from 'pg'
		if matches := esmNamedRe.FindStringSubmatch(trimmed); matches != nil {
			names := matches[1]
			pkg := matches[2]
			for _, name := range strings.Split(names, ",") {
				name = strings.TrimSpace(name)
				// Handle "as" aliases: import { Pool as MyPool } from 'pg'
				if idx := strings.Index(name, " as "); idx >= 0 {
					alias := strings.TrimSpace(name[idx+4:])
					m[alias] = pkg
				} else if name != "" {
					m[name] = pkg
				}
			}
			continue
		}

		// CJS destructured: const { Pool } = require('pg')
		// Must check before CJS default since both start with const/let/var
		if matches := cjsDestructuredRe.FindStringSubmatch(trimmed); matches != nil {
			names := matches[1]
			pkg := matches[2]
			for _, name := range strings.Split(names, ",") {
				name = strings.TrimSpace(name)
				if name != "" {
					m[name] = pkg
				}
			}
			continue
		}

		// CJS default: const axios = require('axios')
		if matches := cjsDefaultRe.FindStringSubmatch(trimmed); matches != nil {
			localName := matches[1]
			pkg := matches[2]
			m[localName] = pkg
			continue
		}
	}

	return m
}

// isImportLine returns true if the line is an import or require statement.
func isImportLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "import ") {
		return true
	}
	if cjsDefaultRe.MatchString(trimmed) || cjsDestructuredRe.MatchString(trimmed) {
		return true
	}
	return false
}

// extractTargetFromLine extracts a URL target from the first string argument on the line.
// If argIndex is -1 or no URL is found, returns "".
func extractTargetFromLine(line string, argIndex int) string {
	if argIndex < 0 {
		return ""
	}

	// Find string arguments in the line
	matches := stringArgRe.FindAllStringSubmatch(line, -1)
	if matches == nil {
		return ""
	}

	// Use the first string argument (argIndex 0 in most cases)
	// For JS patterns, argIndex is almost always 0 (first arg).
	idx := argIndex
	if idx >= len(matches) {
		return ""
	}

	raw := matches[idx][1]
	return extractURLHost(raw)
}

// extractURLHost extracts the hostname from a URL string using the shared connstring package.
func extractURLHost(raw string) string {
	if result, ok := connstring.Parse(raw); ok {
		return result.Host
	}
	return ""
}

// inferEnvVarTargetType infers a target type from an environment variable name.
func inferEnvVarTargetType(name string) string {
	upper := strings.ToUpper(name)
	switch {
	case strings.HasPrefix(upper, "DATABASE_") || strings.HasPrefix(upper, "DB_") || strings.HasPrefix(upper, "MONGO_"):
		return "database"
	case strings.HasPrefix(upper, "REDIS_"):
		return "cache"
	case strings.HasPrefix(upper, "KAFKA_") || strings.HasPrefix(upper, "RABBIT_") || strings.HasPrefix(upper, "AMQP_") || strings.HasPrefix(upper, "NATS_"):
		return "message-broker"
	default:
		return "unknown"
	}
}

// evidenceSnippet returns the source line at lineNum (1-based), trimmed, max 200 chars.
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
