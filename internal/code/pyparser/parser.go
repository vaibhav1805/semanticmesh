package pyparser

import (
	"path/filepath"
	"strings"

	"github.com/vaibhav1805/semanticmesh/internal/code"
	"github.com/vaibhav1805/semanticmesh/internal/code/comments"
	"github.com/vaibhav1805/semanticmesh/internal/code/connstring"
)

// Compile-time check that PythonParser implements code.LanguageParser.
var _ code.LanguageParser = (*PythonParser)(nil)

// PythonParser analyzes Python source files for infrastructure dependency signals
// using regex-based pattern matching against known library calls.
type PythonParser struct {
	patterns map[string]PyDetectionPattern // "package.function" -> pattern
}

// NewPythonParser creates a PythonParser with the default detection patterns.
func NewPythonParser() *PythonParser {
	return &PythonParser{
		patterns: buildPatternIndex(DefaultPythonPatterns),
	}
}

// Name returns "python".
func (p *PythonParser) Name() string { return "python" }

// Extensions returns [".py"].
func (p *PythonParser) Extensions() []string { return []string{".py"} }

// importEntry tracks a resolved Python import.
type importEntry struct {
	// packageName is the original Python package (e.g., "requests", "redis", "kafka")
	packageName string
	// qualifiedName is the fully qualified name for from-imports (e.g., "redis.Redis", "kafka.KafkaProducer")
	// Empty for bare/aliased imports.
	qualifiedName string
}

// ParseFile analyzes a Python source file and returns detected infrastructure signals.
// Returns nil, nil for test files (*_test.py, test_*.py, conftest.py).
func (p *PythonParser) ParseFile(filePath string, content []byte) ([]code.CodeSignal, error) {
	// Skip test files
	base := filepath.Base(filePath)
	if isTestFile(base) {
		return nil, nil
	}

	lines := strings.Split(string(content), "\n")

	// First pass: build import map
	// importMap maps local name -> importEntry
	importMap := p.buildImportMap(lines)

	var signals []code.CodeSignal

	// Second pass: detect calls and comment hints
	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Skip pure comment lines (handled by shared comments.Analyze below)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Skip decorator lines
		if strings.HasPrefix(trimmed, "@") {
			continue
		}

		// Strip inline comments before matching
		codePart := stripInlineComment(trimmed)

		// Find all function calls on this line
		callMatches := callPatternRe.FindAllStringSubmatch(codePart, -1)
		for _, match := range callMatches {
			obj := match[1]  // object/module name (may be empty for bare calls)
			fn := match[2]   // function name
			args := match[3] // arguments string

			sig := p.matchCall(obj, fn, args, lineNum, lines, importMap)
			if sig != nil {
				signals = append(signals, *sig)
			}
		}

		// Also check for bare function calls that don't match the obj.fn pattern
		// (from-imports like Redis(...), KafkaProducer(...), etc.)
		if !strings.Contains(codePart, ".") || true {
			// Check for bare calls (no dot prefix) that match from-imports
			p.matchBareCalls(codePart, lineNum, lines, importMap, &signals)
		}
	}

	// Scan comments using shared comment analyzer
	commentSignals := comments.Analyze(lines, comments.SyntaxPython, nil)
	for i := range commentSignals {
		commentSignals[i].Language = "python"
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
				Language:        "python",
				Confidence:      0.7,
			})
		}
	}

	return signals, nil
}

// buildImportMap scans lines for Python import statements and builds a mapping
// from local name to import metadata.
func (p *PythonParser) buildImportMap(lines []string) map[string]importEntry {
	importMap := make(map[string]importEntry)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// import X as Y
		if matches := aliasedImportRe.FindStringSubmatch(trimmed); len(matches) >= 3 {
			pkg := matches[1]
			alias := matches[2]
			importMap[alias] = importEntry{packageName: pkg}
			continue
		}

		// import X
		if matches := bareImportRe.FindStringSubmatch(trimmed); len(matches) >= 2 {
			pkg := matches[1]
			importMap[pkg] = importEntry{packageName: pkg}
			continue
		}

		// from X import Y [as Z]
		if matches := fromImportRe.FindStringSubmatch(trimmed); len(matches) >= 3 {
			pkg := matches[1]
			name := matches[2]
			alias := matches[3] // may be empty

			localName := name
			if alias != "" {
				localName = alias
			}

			importMap[localName] = importEntry{
				packageName:   pkg,
				qualifiedName: pkg + "." + name,
			}
			continue
		}
	}

	return importMap
}

// matchCall tries to match an object.function() call against the pattern table.
func (p *PythonParser) matchCall(obj, fn, args string, lineNum int, lines []string, importMap map[string]importEntry) *code.CodeSignal {
	if obj == "" {
		return nil
	}

	entry, ok := importMap[obj]
	if !ok {
		return nil
	}

	// Build lookup key: package.function
	key := entry.packageName + "." + fn
	pattern, ok := p.patterns[key]
	if !ok {
		return nil
	}

	// Special case: boto3.client requires "sqs" argument
	if entry.packageName == "boto3" && fn == "client" {
		if !boto3SQSArgRe.MatchString(args) {
			return nil
		}
		return &code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: "sqs",
			TargetType:      pattern.TargetType,
			DetectionKind:   pattern.Kind,
			Evidence:        evidenceSnippet(lines, lineNum),
			Language:        "python",
			Confidence:      pattern.Confidence,
		}
	}

	target := p.extractTarget(args, pattern)

	return &code.CodeSignal{
		LineNumber:      lineNum,
		TargetComponent: target,
		TargetType:      pattern.TargetType,
		DetectionKind:   pattern.Kind,
		Evidence:        evidenceSnippet(lines, lineNum),
		Language:        "python",
		Confidence:      pattern.Confidence,
	}
}

// matchBareCalls handles from-imported bare function calls like Redis(...), KafkaProducer(...).
// These don't have a dot-qualified object prefix.
func (p *PythonParser) matchBareCalls(codePart string, lineNum int, lines []string, importMap map[string]importEntry, signals *[]code.CodeSignal) {
	for localName, entry := range importMap {
		if entry.qualifiedName == "" {
			continue // not a from-import
		}

		// Check if this local name is called as a bare function: Name(...)
		// Must not be preceded by a dot (to avoid matching obj.Name(...) twice)
		callIdx := strings.Index(codePart, localName+"(")
		if callIdx < 0 {
			continue
		}
		// Make sure it's not part of obj.Name (preceded by a dot)
		if callIdx > 0 && codePart[callIdx-1] == '.' {
			continue
		}

		// Look up in pattern table using qualified name
		pattern, ok := p.patterns[entry.qualifiedName]
		if !ok {
			continue
		}

		// Already matched by matchCall? Skip to avoid duplicates.
		// matchCall handles obj.fn patterns; bare calls have no obj prefix.
		// We check by looking for a dot before the function name.
		// If there's no dot, this is a genuine bare call.

		// Extract args from the call
		startParen := callIdx + len(localName)
		args := extractParenContent(codePart, startParen)

		target := p.extractTarget(args, pattern)

		*signals = append(*signals, code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: target,
			TargetType:      pattern.TargetType,
			DetectionKind:   pattern.Kind,
			Evidence:        evidenceSnippet(lines, lineNum),
			Language:        "python",
			Confidence:      pattern.Confidence,
		})
	}
}

// extractParenContent extracts content between parentheses starting at the given index.
func extractParenContent(s string, startIdx int) string {
	if startIdx >= len(s) || s[startIdx] != '(' {
		return ""
	}
	depth := 0
	for i := startIdx; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[startIdx+1 : i]
			}
		}
	}
	return s[startIdx+1:]
}

// extractTarget determines the target component name from the call arguments.
func (p *PythonParser) extractTarget(args string, pattern PyDetectionPattern) string {
	// For pika.BlockingConnection, look inside for ConnectionParameters
	if pattern.Package == "pika" {
		if matches := pikaParamsRe.FindStringSubmatch(args); len(matches) >= 2 {
			return matches[1]
		}
	}

	if pattern.ArgIndex >= 0 {
		// Extract positional argument (the first string)
		if matches := stringArgRe.FindStringSubmatch(args); len(matches) >= 2 {
			return extractURLHost(matches[1])
		}
	}

	// Try keyword arguments: host=, bootstrap_servers=
	if matches := kwargHostRe.FindStringSubmatch(args); len(matches) >= 2 {
		return matches[1]
	}
	if matches := kwargBootstrapRe.FindStringSubmatch(args); len(matches) >= 2 {
		return extractURLHost(matches[1])
	}

	// Fallback: try first string arg even for ArgIndex=-1
	if matches := stringArgRe.FindStringSubmatch(args); len(matches) >= 2 {
		host := extractURLHost(matches[1])
		if host != "" {
			return host
		}
		return matches[1]
	}

	// Final fallback: derive from package name
	return pattern.Package
}

// extractURLHost extracts the hostname from a URL, connection string, or host:port pair.
// Uses the shared connstring package. Returns the original string if no hostname can be extracted.
func extractURLHost(raw string) string {
	if result, ok := connstring.Parse(raw); ok {
		return result.Host
	}
	return raw
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

// isTestFile returns true if the filename matches Python test file patterns.
func isTestFile(filename string) bool {
	if strings.HasPrefix(filename, "test_") && strings.HasSuffix(filename, ".py") {
		return true
	}
	if strings.HasSuffix(filename, "_test.py") {
		return true
	}
	if filename == "conftest.py" {
		return true
	}
	return false
}

// stripInlineComment removes inline comments from a Python code line.
// Handles the case where # appears inside strings by being conservative:
// only strips if # is preceded by whitespace.
func stripInlineComment(line string) string {
	// Simple approach: find # that's not inside a string
	inSingle := false
	inDouble := false
	for i, c := range line {
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
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
