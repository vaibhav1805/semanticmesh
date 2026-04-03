package comments

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/semanticmesh/semanticmesh/internal/code"
)

// CommentSyntax identifies which comment syntaxes to recognize.
type CommentSyntax int

const (
	SyntaxGo         CommentSyntax = iota // // and /* */
	SyntaxPython                          // # and """ and '''
	SyntaxJavaScript                      // //, /* */, and /** */
)

// explicitPattern matches explicit dependency verbs followed by a component-like name.
// Case-insensitive. Applied to comment text (after stripping comment prefix).
var explicitPattern = regexp.MustCompile(
	`(?i)(?:calls|depends on|uses|connects to|talks to|sends to|reads from|writes to)\s+([\w][\w-]*(?:\.[\w][\w-]*)*)`,
)

// todoPattern matches TODO/FIXME/HACK/XXX annotations referencing component-like names
// with infrastructure suffixes to reduce false positives.
var todoPattern = regexp.MustCompile(
	`(?i)(?:TODO|FIXME|HACK|XXX)[:\s]+.*?([\w][\w.-]*(?:-(?:service|api|db|cache|queue|broker|cluster))[\w.-]*)`,
)

// urlPattern matches URLs in comment text.
var urlPattern = regexp.MustCompile(`https?://[^\s,;)"']+|[a-z][a-z0-9+.-]*://[^\s,;)"']+`)

// schemeTypes maps URL schemes to target types for inference.
var schemeTypes = map[string]string{
	"postgres":    "database",
	"postgresql":  "database",
	"mysql":       "database",
	"mongodb":     "database",
	"mongodb+srv": "database",
	"redis":       "cache",
	"rediss":      "cache",
	"amqp":        "message-broker",
	"amqps":       "message-broker",
	"nats":        "message-broker",
	"http":        "service",
	"https":       "service",
}

// filteredHosts are documentation/example domains that should not produce signals.
var filteredHosts = map[string]bool{
	"example.com":      true,
	"example.org":      true,
	"example.net":      true,
	"localhost":        true,
	"127.0.0.1":        true,
	"0.0.0.0":          true,
	"docs.python.org":  true,
	"docs.go.dev":      true,
	"developer.mozilla.org": true,
	"github.com":       true,
	"stackoverflow.com": true,
	"en.wikipedia.org": true,
	"www.example.com":  true,
}

// Analyze scans lines for comment-based dependency signals.
// knownComponents is used for confidence boosting (0.5 for known, 0.4 for new explicit, 0.3 for ambiguous).
// SourceFile and Language are left empty -- the caller sets them.
func Analyze(lines []string, syntax CommentSyntax, knownComponents map[string]bool) []code.CodeSignal {
	if len(lines) == 0 {
		return nil
	}

	var signals []code.CodeSignal
	inBlock := false
	var blockDelim string // tracks which delimiter opened the block ("/*", `"""`, "'''")

	for lineIdx, line := range lines {
		lineNum := lineIdx + 1
		trimmed := strings.TrimSpace(line)

		// Track block comment / docstring state
		if inBlock {
			closed, commentText := handleBlockClose(trimmed, blockDelim)
			if closed {
				inBlock = false
				// Also scan the text before the closing delimiter
				if commentText != "" {
					signals = appendPatternMatches(signals, commentText, lineNum, lines, knownComponents)
				}
			} else {
				// Entire line is inside a block comment/docstring
				text := stripBlockPrefix(trimmed)
				if text != "" {
					signals = appendPatternMatches(signals, text, lineNum, lines, knownComponents)
				}
			}
			continue
		}

		// Check for block comment / docstring open
		if opened, delim, textBefore, textInside := checkBlockOpen(trimmed, syntax); opened {
			// Text before the block open is regular code -- skip it
			_ = textBefore
			inBlock = true
			blockDelim = delim
			// If the block also closes on the same line, handle it
			if closed, commentText := handleBlockClose(textInside, blockDelim); closed {
				inBlock = false
				if commentText != "" {
					signals = appendPatternMatches(signals, commentText, lineNum, lines, knownComponents)
				}
			} else if textInside != "" {
				signals = appendPatternMatches(signals, textInside, lineNum, lines, knownComponents)
			}
			continue
		}

		// Check for single-line comments
		commentText := extractSingleLineComment(trimmed, syntax)
		if commentText != "" {
			signals = appendPatternMatches(signals, commentText, lineNum, lines, knownComponents)
		}
	}

	return signals
}

// extractSingleLineComment extracts the text content from a single-line comment.
// Returns empty string if the line is not a single-line comment for the given syntax.
func extractSingleLineComment(trimmed string, syntax CommentSyntax) string {
	switch syntax {
	case SyntaxGo, SyntaxJavaScript:
		// Check for // comment (can be standalone or inline after code)
		if idx := findSingleLineCommentStart(trimmed, "//"); idx >= 0 {
			return strings.TrimSpace(trimmed[idx+2:])
		}
	case SyntaxPython:
		// Check for # comment (can be standalone or inline after code)
		if idx := findSingleLineCommentStart(trimmed, "#"); idx >= 0 {
			return strings.TrimSpace(trimmed[idx+1:])
		}
	}
	return ""
}

// findSingleLineCommentStart finds the position of a comment prefix, accounting for
// string literals. Returns -1 if not found or if it's inside a string.
func findSingleLineCommentStart(line, prefix string) int {
	inSingle := false
	inDouble := false
	inBacktick := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		// Track string state
		if ch == '\'' && !inDouble && !inBacktick {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle && !inBacktick {
			inDouble = !inDouble
			continue
		}
		if ch == '`' && !inSingle && !inDouble {
			inBacktick = !inBacktick
			continue
		}

		// Only look for prefix outside strings
		if !inSingle && !inDouble && !inBacktick {
			if strings.HasPrefix(line[i:], prefix) {
				return i
			}
		}
	}
	return -1
}

// checkBlockOpen checks if a line opens a block comment or docstring.
// Returns: opened, delimiter, textBefore, textAfterOpen
func checkBlockOpen(trimmed string, syntax CommentSyntax) (bool, string, string, string) {
	switch syntax {
	case SyntaxGo, SyntaxJavaScript:
		if idx := strings.Index(trimmed, "/*"); idx >= 0 {
			before := trimmed[:idx]
			after := trimmed[idx+2:]
			return true, "/*", before, after
		}
	case SyntaxPython:
		// Check for triple-quote docstrings (""" or ''')
		for _, delim := range []string{`"""`, `'''`} {
			if idx := strings.Index(trimmed, delim); idx >= 0 {
				before := trimmed[:idx]
				after := trimmed[idx+3:]
				return true, delim, before, after
			}
		}
	}
	return false, "", "", ""
}

// handleBlockClose checks if a line closes a block comment/docstring.
// Returns: closed, textBeforeClose
func handleBlockClose(text, delim string) (bool, string) {
	var closeMarker string
	switch delim {
	case "/*":
		closeMarker = "*/"
	case `"""`:
		closeMarker = `"""`
	case `'''`:
		closeMarker = `'''`
	default:
		return false, ""
	}

	if idx := strings.Index(text, closeMarker); idx >= 0 {
		before := text[:idx]
		return true, strings.TrimSpace(before)
	}
	return false, ""
}

// stripBlockPrefix removes common block comment prefixes like " * " from JSDoc.
func stripBlockPrefix(text string) string {
	text = strings.TrimSpace(text)
	// Strip leading "* " or "*" from JSDoc-style lines
	if strings.HasPrefix(text, "* ") {
		text = text[2:]
	} else if text == "*" {
		return ""
	}
	return strings.TrimSpace(text)
}

// appendPatternMatches applies all detection patterns to comment text and appends matches.
func appendPatternMatches(signals []code.CodeSignal, commentText string, lineNum int, lines []string, known map[string]bool) []code.CodeSignal {
	// Track what we've already matched on this line to avoid duplicates
	seen := map[string]bool{}

	// 1. Explicit dependency patterns
	matches := explicitPattern.FindAllStringSubmatch(commentText, -1)
	for _, m := range matches {
		target := m[1]
		if seen[target] {
			continue
		}
		seen[target] = true
		conf := 0.4
		if known[target] {
			conf = 0.5
		}
		signals = append(signals, code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: target,
			TargetType:      "unknown",
			DetectionKind:   "comment_hint",
			Evidence:        evidenceSnippet(lines, lineNum),
			Confidence:      conf,
		})
	}

	// 2. TODO/FIXME patterns (only if not already matched by explicit pattern)
	todoMatches := todoPattern.FindAllStringSubmatch(commentText, -1)
	for _, m := range todoMatches {
		target := m[1]
		if seen[target] {
			continue
		}
		seen[target] = true
		signals = append(signals, code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: target,
			TargetType:      "unknown",
			DetectionKind:   "comment_hint",
			Evidence:        evidenceSnippet(lines, lineNum),
			Confidence:      0.3,
		})
	}

	// 3. URL references in comments
	urlMatches := urlPattern.FindAllString(commentText, -1)
	for _, raw := range urlMatches {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		host := u.Hostname()
		if host == "" {
			continue
		}
		if filteredHosts[host] {
			continue
		}
		if seen[host] {
			continue
		}
		seen[host] = true

		targetType := schemeTypes[u.Scheme]
		if targetType == "" {
			targetType = "unknown"
		}

		conf := 0.4
		if known[host] {
			conf = 0.5
		}

		signals = append(signals, code.CodeSignal{
			LineNumber:      lineNum,
			TargetComponent: host,
			TargetType:      targetType,
			DetectionKind:   "comment_hint",
			Evidence:        evidenceSnippet(lines, lineNum),
			Confidence:      conf,
		})
	}

	return signals
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
