package tfparser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vaibhav1805/semanticmesh/internal/code"
)

// helper finds the first signal matching kind and target.
func findSignal(signals []code.CodeSignal, kind, target string) *code.CodeSignal {
	for i := range signals {
		if signals[i].DetectionKind == kind && signals[i].TargetComponent == target {
			return &signals[i]
		}
	}
	return nil
}

func parseContent(t *testing.T, content string) []code.CodeSignal {
	t.Helper()
	p := NewTerraformParser()
	signals, err := p.ParseFile("main.tf", []byte(content))
	if err != nil {
		t.Fatalf("ParseFile error: %v", err)
	}
	return signals
}

// ── Resource declarations ──────────────────────────────────────────────────

func TestResourceDeclaration_KnownType(t *testing.T) {
	signals := parseContent(t, `
resource "aws_db_instance" "prod_db" {
  engine = "postgres"
}
`)
	sig := findSignal(signals, "terraform_resource", "aws_db_instance.prod_db")
	if sig == nil {
		t.Fatal("expected signal for aws_db_instance.prod_db, got none")
	}
	if sig.TargetType != "database" {
		t.Errorf("TargetType = %q, want %q", sig.TargetType, "database")
	}
	if sig.Confidence < 0.90 {
		t.Errorf("Confidence = %v, want >= 0.90", sig.Confidence)
	}
	if sig.Language != "terraform" {
		t.Errorf("Language = %q, want %q", sig.Language, "terraform")
	}
}

func TestResourceDeclaration_UnknownType(t *testing.T) {
	signals := parseContent(t, `
resource "random_id" "suffix" {
  byte_length = 4
}
`)
	if len(signals) != 0 {
		t.Errorf("expected no signals for unknown resource type, got %d", len(signals))
	}
}

func TestResourceDeclaration_Cache(t *testing.T) {
	signals := parseContent(t, `
resource "aws_elasticache_replication_group" "redis" {
  description = "session cache"
}
`)
	sig := findSignal(signals, "terraform_resource", "aws_elasticache_replication_group.redis")
	if sig == nil {
		t.Fatal("expected cache signal")
	}
	if sig.TargetType != "cache" {
		t.Errorf("TargetType = %q, want cache", sig.TargetType)
	}
}

func TestResourceDeclaration_Queue(t *testing.T) {
	signals := parseContent(t, `resource "aws_sqs_queue" "jobs" {}`)
	sig := findSignal(signals, "terraform_resource", "aws_sqs_queue.jobs")
	if sig == nil || sig.TargetType != "queue" {
		t.Errorf("expected queue signal, got %v", sig)
	}
}

func TestResourceDeclaration_Lambda(t *testing.T) {
	signals := parseContent(t, `resource "aws_lambda_function" "api" {}`)
	sig := findSignal(signals, "terraform_resource", "aws_lambda_function.api")
	if sig == nil || sig.TargetType != "service" {
		t.Errorf("expected service signal, got %v", sig)
	}
}

func TestResourceDeclaration_KubernetesDeployment(t *testing.T) {
	signals := parseContent(t, `resource "kubernetes_deployment" "web" {}`)
	sig := findSignal(signals, "terraform_resource", "kubernetes_deployment.web")
	if sig == nil || sig.TargetType != "service" {
		t.Errorf("expected service signal for kubernetes_deployment, got %v", sig)
	}
}

func TestResourceDeclaration_HelmRelease(t *testing.T) {
	signals := parseContent(t, `resource "helm_release" "nginx" {}`)
	sig := findSignal(signals, "terraform_resource", "helm_release.nginx")
	if sig == nil || sig.TargetType != "service" {
		t.Errorf("expected service signal for helm_release, got %v", sig)
	}
}

// ── Data sources ──────────────────────────────────────────────────────────

func TestDataSource_KnownType(t *testing.T) {
	signals := parseContent(t, `
data "aws_db_instance" "existing" {
  db_instance_identifier = "prod"
}
`)
	sig := findSignal(signals, "terraform_data_source", "aws_db_instance.existing")
	if sig == nil {
		t.Fatal("expected data source signal")
	}
	if sig.TargetType != "database" {
		t.Errorf("TargetType = %q, want database", sig.TargetType)
	}
	// Data sources should have confidence scaled down from the base pattern.
	if sig.Confidence >= 0.92 {
		t.Errorf("expected reduced confidence for data source, got %v", sig.Confidence)
	}
}

func TestDataSource_UnknownType(t *testing.T) {
	signals := parseContent(t, `data "aws_caller_identity" "current" {}`)
	if len(signals) != 0 {
		t.Errorf("expected no signals for unknown data source type, got %d", len(signals))
	}
}

// ── Module calls ──────────────────────────────────────────────────────────

func TestModule_KnownRegistrySource(t *testing.T) {
	signals := parseContent(t, `
module "rds" {
  source = "terraform-aws-modules/rds/aws"
  version = "~> 5.0"
}
`)
	sig := findSignal(signals, "terraform_module", "rds")
	if sig == nil {
		t.Fatal("expected module signal")
	}
	if sig.TargetType != "database" {
		t.Errorf("TargetType = %q, want database", sig.TargetType)
	}
	if sig.Confidence != 0.80 {
		t.Errorf("Confidence = %v, want 0.80 for known module", sig.Confidence)
	}
}

func TestModule_UnknownLocalSource(t *testing.T) {
	signals := parseContent(t, `
module "vpc" {
  source = "./modules/vpc"
}
`)
	sig := findSignal(signals, "terraform_module", "vpc")
	if sig == nil {
		t.Fatal("expected module signal for local source")
	}
	if sig.TargetType != "service" {
		t.Errorf("TargetType = %q, want service for unknown module", sig.TargetType)
	}
	if sig.Confidence != 0.60 {
		t.Errorf("Confidence = %v, want 0.60 for unknown module", sig.Confidence)
	}
}

// ── depends_on ────────────────────────────────────────────────────────────

func TestDependsOn_SingleRef(t *testing.T) {
	signals := parseContent(t, `
resource "aws_lambda_function" "api" {
  depends_on = [aws_db_instance.prod_db]
}
`)
	sig := findSignal(signals, "terraform_dependency", "aws_db_instance.prod_db")
	if sig == nil {
		t.Fatal("expected terraform_dependency signal")
	}
	if sig.TargetType != "database" {
		t.Errorf("TargetType = %q, want database", sig.TargetType)
	}
	if sig.Confidence != 0.90 {
		t.Errorf("Confidence = %v, want 0.90", sig.Confidence)
	}
}

func TestDependsOn_MultipleRefs(t *testing.T) {
	signals := parseContent(t, `
resource "aws_lambda_function" "api" {
  depends_on = [aws_db_instance.prod_db, aws_elasticache_cluster.redis]
}
`)
	if findSignal(signals, "terraform_dependency", "aws_db_instance.prod_db") == nil {
		t.Error("expected dependency signal for aws_db_instance.prod_db")
	}
	if findSignal(signals, "terraform_dependency", "aws_elasticache_cluster.redis") == nil {
		t.Error("expected dependency signal for aws_elasticache_cluster.redis")
	}
}

func TestDependsOn_MultiLine(t *testing.T) {
	signals := parseContent(t, `
resource "aws_lambda_function" "api" {
  depends_on = [
    aws_db_instance.prod_db,
    aws_sqs_queue.jobs,
  ]
}
`)
	if findSignal(signals, "terraform_dependency", "aws_db_instance.prod_db") == nil {
		t.Error("expected dependency signal for aws_db_instance.prod_db")
	}
	if findSignal(signals, "terraform_dependency", "aws_sqs_queue.jobs") == nil {
		t.Error("expected dependency signal for aws_sqs_queue.jobs")
	}
}

// ── Attribute interpolations ──────────────────────────────────────────────

func TestAttributeInterpolation_BareRef(t *testing.T) {
	signals := parseContent(t, `
resource "aws_lambda_function" "api" {
  environment {
    variables = {
      DB_HOST = aws_db_instance.prod_db.address
    }
  }
}
`)
	sig := findSignal(signals, "terraform_ref", "aws_db_instance.prod_db")
	if sig == nil {
		t.Fatal("expected terraform_ref signal for bare attribute reference")
	}
	if sig.TargetType != "database" {
		t.Errorf("TargetType = %q, want database", sig.TargetType)
	}
	if sig.Confidence != 0.75 {
		t.Errorf("Confidence = %v, want 0.75", sig.Confidence)
	}
}

func TestAttributeInterpolation_StringInterpolation(t *testing.T) {
	signals := parseContent(t, `
resource "aws_lambda_function" "api" {
  environment {
    variables = {
      REDIS_URL = "redis://${aws_elasticache_cluster.main.cache_nodes.0.address}:6379"
    }
  }
}
`)
	sig := findSignal(signals, "terraform_ref", "aws_elasticache_cluster.main")
	if sig == nil {
		t.Fatal("expected terraform_ref signal inside string interpolation")
	}
}

func TestAttributeInterpolation_NoSelfReference(t *testing.T) {
	signals := parseContent(t, `
resource "aws_db_instance" "prod_db" {
  identifier = aws_db_instance.prod_db.id
}
`)
	// Self-reference should be suppressed
	selfSig := findSignal(signals, "terraform_ref", "aws_db_instance.prod_db")
	if selfSig != nil {
		t.Error("self-reference should not produce a terraform_ref signal")
	}
}

// ── Deduplication ─────────────────────────────────────────────────────────

func TestDeduplication_SameTargetMultipleLines(t *testing.T) {
	signals := parseContent(t, `
resource "aws_lambda_function" "api" {
  environment {
    variables = {
      DB_HOST  = aws_db_instance.prod_db.address
      DB_PORT  = aws_db_instance.prod_db.port
    }
  }
}
`)
	count := 0
	for _, s := range signals {
		if s.DetectionKind == "terraform_ref" && s.TargetComponent == "aws_db_instance.prod_db" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 deduplicated terraform_ref signal, got %d", count)
	}
}

// ── Brace on next line ────────────────────────────────────────────────────

func TestBraceOnNextLine(t *testing.T) {
	// Some editors / formatters place the opening brace on its own line.
	signals := parseContent(t, `
resource "aws_s3_bucket" "assets"
{
  bucket = "my-assets"
}
`)
	sig := findSignal(signals, "terraform_resource", "aws_s3_bucket.assets")
	if sig == nil {
		t.Fatal("expected signal when opening brace is on next line")
	}
}

// ── .terraform/ skip ──────────────────────────────────────────────────────

func TestSkipTerraformVendorDir(t *testing.T) {
	p := NewTerraformParser()
	signals, err := p.ParseFile(".terraform/modules/rds/main.tf", []byte(`
resource "aws_db_instance" "main" {}
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected no signals from .terraform/ vendor path, got %d", len(signals))
	}
}

// ── AnalyzeManifests (.tfvars) ────────────────────────────────────────────

func TestAnalyzeManifests_DSNValue(t *testing.T) {
	dir := t.TempDir()
	tfvarsContent := `
db_url = "postgres://user:pass@prod-postgres.internal:5432/mydb"
region = "us-east-1"
`
	if err := os.WriteFile(filepath.Join(dir, "terraform.tfvars"), []byte(tfvarsContent), 0644); err != nil {
		t.Fatal(err)
	}

	p := NewTerraformParser()
	signals, err := p.AnalyzeManifests(dir)
	if err != nil {
		t.Fatalf("AnalyzeManifests error: %v", err)
	}

	var found bool
	for _, s := range signals {
		if s.DetectionKind == "env_var_ref" && s.TargetComponent == "prod-postgres.internal" {
			found = true
			if s.TargetType != "database" {
				t.Errorf("TargetType = %q, want database", s.TargetType)
			}
			if s.Confidence != 0.65 {
				t.Errorf("Confidence = %v, want 0.65", s.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("expected env_var_ref signal for prod-postgres.internal, signals: %+v", signals)
	}
}

func TestAnalyzeManifests_NoTfvarsFiles(t *testing.T) {
	dir := t.TempDir()
	p := NewTerraformParser()
	signals, err := p.AnalyzeManifests(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected no signals from empty dir, got %d", len(signals))
	}
}
