package code

// CodeSignal represents a detected infrastructure dependency from source code analysis.
// Each signal captures one call site where the source component communicates with
// an external infrastructure component (HTTP service, database, cache, queue, etc.).
type CodeSignal struct {
	// SourceFile is the path of the file where the signal was detected.
	SourceFile string `json:"source_file"`

	// LineNumber is the line in SourceFile where the call/reference occurs.
	LineNumber int `json:"line_number"`

	// TargetComponent is the inferred name of the dependency (e.g., hostname, service name).
	TargetComponent string `json:"target_component"`

	// TargetType classifies the dependency: service, database, cache, message-broker, queue, unknown.
	TargetType string `json:"target_type"`

	// DetectionKind describes what was detected: http_call, db_connection, cache_client,
	// queue_producer, queue_consumer, comment_hint.
	DetectionKind string `json:"detection_kind"`

	// Evidence is a snippet of the source line that triggered detection (trimmed, max 200 chars).
	Evidence string `json:"evidence"`

	// Language is the programming language of the source file (e.g., "go", "python", "javascript").
	Language string `json:"language"`

	// Confidence is the detection certainty in [0.4, 1.0].
	Confidence float64 `json:"confidence"`
}
