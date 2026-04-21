package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicOption "github.com/anthropics/anthropic-sdk-go/option"
)

// BedrockLLMConfig holds configuration for AWS Bedrock LLM access.
type BedrockLLMConfig struct {
	// Model is the Bedrock model ID (e.g., "us.anthropic.claude-sonnet-4-5-20250929-v1:0").
	Model string

	// MaxTokens is the maximum number of tokens for each response.
	MaxTokens int

	// Temperature controls randomness (0.0 = deterministic, 1.0 = creative).
	Temperature float64

	// MaxConcurrency limits parallel LLM requests (connection pool size).
	MaxConcurrency int

	// Timeout is the per-request timeout duration.
	Timeout time.Duration

	// RetryAttempts is the number of retry attempts on transient failures.
	RetryAttempts int

	// RetryDelay is the base delay between retries (uses exponential backoff).
	RetryDelay time.Duration

	// AWSRegion is the AWS region for Bedrock (e.g., "us-east-1").
	// If empty, uses the default region from AWS credentials.
	AWSRegion string

	// BedrockBaseURL is the Bedrock API endpoint override.
	// Defaults to the standard Bedrock endpoint if empty.
	BedrockBaseURL string
}

// DefaultBedrockLLMConfig returns a BedrockLLMConfig with sensible defaults.
func DefaultBedrockLLMConfig() BedrockLLMConfig {
	return BedrockLLMConfig{
		Model:          "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		MaxTokens:      4096,
		Temperature:    0.0, // Deterministic for consistent results
		MaxConcurrency: 4,
		Timeout:        60 * time.Second,
		RetryAttempts:  3,
		RetryDelay:     2 * time.Second,
		AWSRegion:      "us-east-1",
		BedrockBaseURL: "",
	}
}

// BedrockLLMClient is a connection-pooled, retry-enabled client for AWS Bedrock (Claude via Anthropic SDK).
type BedrockLLMClient struct {
	config    BedrockLLMConfig
	client    anthropic.Client
	semaphore chan struct{} // Connection pool semaphore
	mu        sync.Mutex    // Protects concurrent access to metrics
	metrics   LLMMetrics
}

// LLMMetrics tracks LLM call statistics.
type LLMMetrics struct {
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
	TotalTokens     int
	TotalLatencyMs  int64
}

// NewBedrockLLMClient creates a new AWS Bedrock LLM client with connection pooling.
func NewBedrockLLMClient(cfg BedrockLLMConfig) (*BedrockLLMClient, error) {
	// Build Bedrock endpoint URL.
	baseURL := cfg.BedrockBaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", cfg.AWSRegion)
	}

	// Initialize Anthropic client with Bedrock endpoint.
	// The SDK will use AWS credentials from the environment (IAM role, ~/.aws/credentials, etc.)
	client := anthropic.NewClient(
		anthropicOption.WithBaseURL(baseURL),
		anthropicOption.WithRequestTimeout(cfg.Timeout),
	)

	return &BedrockLLMClient{
		config:    cfg,
		client:    client,
		semaphore: make(chan struct{}, cfg.MaxConcurrency),
		metrics:   LLMMetrics{},
	}, nil
}

// CallLLM invokes the LLM with the given prompt and returns the response text.
// Uses connection pooling (semaphore) and retries on transient failures.
func (c *BedrockLLMClient) CallLLM(ctx context.Context, prompt string) (string, error) {
	// Acquire semaphore (connection pool slot).
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Track metrics.
	start := time.Now()
	c.recordRequest()
	defer func() {
		latency := time.Since(start).Milliseconds()
		c.recordLatency(latency)
	}()

	// Retry loop.
	var lastErr error
	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: delay * 2^attempt
			backoff := c.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		response, err := c.callLLMOnce(ctx, prompt)
		if err == nil {
			c.recordSuccess()
			return response, nil
		}

		lastErr = err

		// Check if error is retryable (transient network/rate limit errors).
		if !isRetryableError(err) {
			break
		}
	}

	c.recordFailure()
	return "", fmt.Errorf("llm call failed after %d attempts: %w", c.config.RetryAttempts+1, lastErr)
}

// callLLMOnce performs a single LLM API call without retries.
func (c *BedrockLLMClient) callLLMOnce(ctx context.Context, prompt string) (string, error) {
	// Build request.
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.config.Model),
		MaxTokens: int64(c.config.MaxTokens),
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(prompt),
			},
		}},
		Temperature: anthropic.Float(c.config.Temperature),
	})
	if err != nil {
		return "", fmt.Errorf("bedrock api call: %w", err)
	}

	// Extract text from content blocks.
	var responseText string
	for _, block := range msg.Content {
		responseText += block.Text
	}

	// Track token usage.
	c.recordTokens(int(msg.Usage.InputTokens) + int(msg.Usage.OutputTokens))

	return responseText, nil
}

// CallLLMBatch invokes the LLM for multiple prompts in parallel, respecting the concurrency limit.
// Returns results in the same order as prompts.
func (c *BedrockLLMClient) CallLLMBatch(ctx context.Context, prompts []string) ([]string, error) {
	if len(prompts) == 0 {
		return nil, nil
	}

	results := make([]string, len(prompts))
	errors := make([]error, len(prompts))
	var wg sync.WaitGroup

	for i, prompt := range prompts {
		wg.Add(1)
		go func(idx int, p string) {
			defer wg.Done()
			resp, err := c.CallLLM(ctx, p)
			results[idx] = resp
			errors[idx] = err
		}(i, prompt)
	}

	wg.Wait()

	// Check for errors.
	for i, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("batch call failed at index %d: %w", i, err)
		}
	}

	return results, nil
}

// CallLLMJSON invokes the LLM and parses the response as JSON into the provided target.
func (c *BedrockLLMClient) CallLLMJSON(ctx context.Context, prompt string, target interface{}) error {
	response, err := c.CallLLM(ctx, prompt)
	if err != nil {
		return err
	}

	// Parse JSON response.
	if err := json.Unmarshal([]byte(response), target); err != nil {
		return fmt.Errorf("parse llm json response: %w (response: %q)", err, response)
	}

	return nil
}

// GetMetrics returns a copy of the current LLM call metrics.
func (c *BedrockLLMClient) GetMetrics() LLMMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.metrics
}

// recordRequest increments the total request counter.
func (c *BedrockLLMClient) recordRequest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.TotalRequests++
}

// recordSuccess increments the success counter.
func (c *BedrockLLMClient) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.SuccessRequests++
}

// recordFailure increments the failure counter.
func (c *BedrockLLMClient) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.FailedRequests++
}

// recordTokens adds to the total token count.
func (c *BedrockLLMClient) recordTokens(tokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.TotalTokens += tokens
}

// recordLatency adds to the total latency.
func (c *BedrockLLMClient) recordLatency(ms int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.TotalLatencyMs += ms
}

// isRetryableError returns true if the error is transient and worth retrying.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Retry on rate limits, timeouts, and 5xx errors.
	retryablePatterns := []string{
		"rate limit",
		"timeout",
		"502",
		"503",
		"504",
		"connection reset",
		"connection refused",
		"throttl",
	}
	for _, pattern := range retryablePatterns {
		if containsIgnoreCaseLLM(errStr, pattern) {
			return true
		}
	}
	return false
}

// containsIgnoreCaseLLM checks if s contains substr (case-insensitive).
func containsIgnoreCaseLLM(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
