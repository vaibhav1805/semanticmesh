package extractors

// getString safely extracts a string value from a map
func getString(row map[string]interface{}, key string) string {
	if v, ok := row[key]; ok && v != nil {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// getInt safely extracts an integer value from a map
func getInt(row map[string]interface{}, key string) int {
	if v, ok := row[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

// getBool safely extracts a boolean value from a map
func getBool(row map[string]interface{}, key string) bool {
	if v, ok := row[key]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
		// Handle numeric booleans (0/1)
		if i, ok := v.(int); ok {
			return i != 0
		}
		if i, ok := v.(int64); ok {
			return i != 0
		}
	}
	return false
}
