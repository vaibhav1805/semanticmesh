package connstring

import (
	"testing"
)

// --- Parse tests ---

func TestParse_URLSchemes(t *testing.T) {
	tests := []struct {
		input      string
		wantHost   string
		wantType   string
		wantOK     bool
		wantConf   float64
	}{
		{"postgres://db-host:5432/myapp", "db-host", "database", true, 0.85},
		{"postgresql://pg-server/production", "pg-server", "database", true, 0.85},
		{"mysql://mysql-host:3306/app", "mysql-host", "database", true, 0.85},
		{"mongodb://mongo-cluster:27017/data", "mongo-cluster", "database", true, 0.85},
		{"mongodb+srv://atlas-host/db", "atlas-host", "database", true, 0.85},
		{"redis://cache-server:6379", "cache-server", "cache", true, 0.85},
		{"rediss://secure-cache:6380/0", "secure-cache", "cache", true, 0.85},
		{"amqp://rabbit-host:5672/vhost", "rabbit-host", "message-broker", true, 0.85},
		{"amqps://secure-rabbit:5671/", "secure-rabbit", "message-broker", true, 0.85},
		{"nats://nats-server:4222", "nats-server", "message-broker", true, 0.85},
		{"http://api-gateway:8080/v1", "api-gateway", "service", true, 0.85},
		{"https://payments.internal:443/charge", "payments.internal", "service", true, 0.85},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, ok := Parse(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if result.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", result.Host, tt.wantHost)
			}
			if result.TargetType != tt.wantType {
				t.Errorf("TargetType = %q, want %q", result.TargetType, tt.wantType)
			}
			if result.Confidence != tt.wantConf {
				t.Errorf("Confidence = %v, want %v", result.Confidence, tt.wantConf)
			}
			if result.Kind != "conn_string" {
				t.Errorf("Kind = %q, want %q", result.Kind, "conn_string")
			}
		})
	}
}

func TestParse_URLWithUserinfo(t *testing.T) {
	result, ok := Parse("postgres://user:pass@db-host:5432/mydb")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if result.Host != "db-host" {
		t.Errorf("Host = %q, want %q", result.Host, "db-host")
	}
	if result.TargetType != "database" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "database")
	}
}

func TestParse_UnknownScheme(t *testing.T) {
	result, ok := Parse("custom://some-host:9999/path")
	if !ok {
		t.Fatal("expected ok=true for unknown scheme with hostname")
	}
	if result.Host != "some-host" {
		t.Errorf("Host = %q, want %q", result.Host, "some-host")
	}
	if result.TargetType != "unknown" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "unknown")
	}
}

func TestParse_FilteredDomains(t *testing.T) {
	filtered := []string{
		"http://example.com/api",
		"http://example.org/test",
		"http://localhost:8080/health",
		"http://127.0.0.1:3000/api",
		"http://0.0.0.0:5000/",
		"https://docs.python.org/3/library/",
		"https://pkg.go.dev/net/http",
		"https://developer.mozilla.org/en-US/",
	}
	for _, raw := range filtered {
		t.Run(raw, func(t *testing.T) {
			_, ok := Parse(raw)
			if ok {
				t.Errorf("Parse(%q) should return ok=false (filtered domain)", raw)
			}
		})
	}
}

func TestParse_DSN_MySQL(t *testing.T) {
	result, ok := Parse("user:password@tcp(mysql-host:3306)/dbname")
	if !ok {
		t.Fatal("expected ok=true for MySQL DSN")
	}
	if result.Host != "mysql-host" {
		t.Errorf("Host = %q, want %q", result.Host, "mysql-host")
	}
	if result.TargetType != "database" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "database")
	}
}

func TestParse_DSN_PostgresKeyValue(t *testing.T) {
	result, ok := Parse("host=pg-server port=5432 dbname=mydb user=admin")
	if !ok {
		t.Fatal("expected ok=true for PostgreSQL key-value DSN")
	}
	if result.Host != "pg-server" {
		t.Errorf("Host = %q, want %q", result.Host, "pg-server")
	}
	if result.TargetType != "database" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "database")
	}
}

func TestParse_HostPort(t *testing.T) {
	result, ok := Parse("kafka-broker:9092")
	if !ok {
		t.Fatal("expected ok=true for host:port")
	}
	if result.Host != "kafka-broker" {
		t.Errorf("Host = %q, want %q", result.Host, "kafka-broker")
	}
	if result.TargetType != "unknown" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "unknown")
	}
}

func TestParse_EdgeCases(t *testing.T) {
	edgeCases := []string{
		"",
		"   ",
		"not-a-url",
		"just-a-word",
		"/relative/path",
	}
	for _, raw := range edgeCases {
		t.Run(raw, func(t *testing.T) {
			_, ok := Parse(raw)
			if ok {
				t.Errorf("Parse(%q) should return ok=false", raw)
			}
		})
	}
}

// --- ParseEnvVarRef tests ---

func TestParseEnvVarRef_Go(t *testing.T) {
	tests := []struct {
		line     string
		wantVars []string
	}{
		{`url := os.Getenv("DATABASE_URL")`, []string{"DATABASE_URL"}},
		{`val, ok := os.LookupEnv("REDIS_HOST")`, []string{"REDIS_HOST"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			refs := ParseEnvVarRef(tt.line)
			if len(refs) != len(tt.wantVars) {
				t.Fatalf("got %d refs, want %d", len(refs), len(tt.wantVars))
			}
			for i, ref := range refs {
				if ref.Name != tt.wantVars[i] {
					t.Errorf("ref[%d].Name = %q, want %q", i, ref.Name, tt.wantVars[i])
				}
			}
		})
	}
}

func TestParseEnvVarRef_Python(t *testing.T) {
	tests := []struct {
		line     string
		wantVars []string
	}{
		{`url = os.environ["DATABASE_URL"]`, []string{"DATABASE_URL"}},
		{`host = os.environ.get("REDIS_HOST")`, []string{"REDIS_HOST"}},
		{`dsn = os.getenv("MONGO_DSN")`, []string{"MONGO_DSN"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			refs := ParseEnvVarRef(tt.line)
			if len(refs) != len(tt.wantVars) {
				t.Fatalf("got %d refs, want %d", len(refs), len(tt.wantVars))
			}
			for i, ref := range refs {
				if ref.Name != tt.wantVars[i] {
					t.Errorf("ref[%d].Name = %q, want %q", i, ref.Name, tt.wantVars[i])
				}
			}
		})
	}
}

func TestParseEnvVarRef_JS(t *testing.T) {
	tests := []struct {
		line     string
		wantVars []string
	}{
		{`const url = process.env.DATABASE_URL;`, []string{"DATABASE_URL"}},
		{`const host = process.env["REDIS_HOST"];`, []string{"REDIS_HOST"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			refs := ParseEnvVarRef(tt.line)
			if len(refs) != len(tt.wantVars) {
				t.Fatalf("got %d refs, want %d", len(refs), len(tt.wantVars))
			}
			for i, ref := range refs {
				if ref.Name != tt.wantVars[i] {
					t.Errorf("ref[%d].Name = %q, want %q", i, ref.Name, tt.wantVars[i])
				}
			}
		})
	}
}

func TestParseEnvVarRef_Shell(t *testing.T) {
	tests := []struct {
		line     string
		wantVars []string
	}{
		{`echo "connecting to ${DATABASE_URL}"`, []string{"DATABASE_URL"}},
		{`redis-cli -u $REDIS_URL`, []string{"REDIS_URL"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			refs := ParseEnvVarRef(tt.line)
			if len(refs) != len(tt.wantVars) {
				t.Fatalf("got %d refs, want %d", len(refs), len(tt.wantVars))
			}
			for i, ref := range refs {
				if ref.Name != tt.wantVars[i] {
					t.Errorf("ref[%d].Name = %q, want %q", i, ref.Name, tt.wantVars[i])
				}
			}
		})
	}
}

func TestParseEnvVarRef_NoMatch(t *testing.T) {
	refs := ParseEnvVarRef(`fmt.Println("hello world")`)
	if len(refs) != 0 {
		t.Errorf("expected no refs, got %d", len(refs))
	}
}

// --- IsConnectionEnvVar tests ---

func TestIsConnectionEnvVar_Positive(t *testing.T) {
	positives := []string{
		"DATABASE_URL",
		"DB_HOST",
		"REDIS_URL",
		"MONGO_URI",
		"RABBIT_ADDR",
		"KAFKA_ENDPOINT",
		"NATS_CONNECTION",
		"AMQP_DSN",
		"SERVICE_URL",
		"API_HOST",
		"CACHE_ADDR",
	}
	for _, name := range positives {
		t.Run(name, func(t *testing.T) {
			if !IsConnectionEnvVar(name) {
				t.Errorf("IsConnectionEnvVar(%q) = false, want true", name)
			}
		})
	}
}

func TestIsConnectionEnvVar_Negative(t *testing.T) {
	negatives := []string{
		"LOG_LEVEL",
		"DEBUG",
		"APP_NAME",
		"VERSION",
		"NODE_ENV",
		"PATH",
		"HOME",
	}
	for _, name := range negatives {
		t.Run(name, func(t *testing.T) {
			if IsConnectionEnvVar(name) {
				t.Errorf("IsConnectionEnvVar(%q) = true, want false", name)
			}
		})
	}
}
