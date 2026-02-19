package sidecar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alexkarsten/dehydrate/pkg/models"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://localhost:%d", port),
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

type APIResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error,omitempty"`
}

type AnalyzeRequest struct {
	Path   string            `json:"path"`
	Files  []models.FileInfo `json:"files,omitempty"`
	Config map[string]any    `json:"config,omitempty"`
}

type AnalyzeResult struct {
	TotalFiles   int                     `json:"total_files"`
	TotalSymbols int                     `json:"total_symbols"`
	Hotspots     []ComplexityHotspot     `json:"hotspots"`
	Duplicates   []models.DuplicateGroup `json:"duplicates"`
	Symbols      []models.Symbol         `json:"symbols"`
}

type ComplexityHotspot struct {
	File                 string `json:"file"`
	Line                 int    `json:"line"`
	Symbol               string `json:"symbol"`
	CyclomaticComplexity int    `json:"cyclomatic_complexity"`
	CognitiveComplexity  int    `json:"cognitive_complexity"`
}

type DeduplicateRequest struct {
	Path                string            `json:"path"`
	Files               []models.FileInfo `json:"files"`
	SimilarityThreshold float64           `json:"similarity_threshold"`
}

type IdiomatizeRequest struct {
	Path     string            `json:"path"`
	Files    []models.FileInfo `json:"files"`
	Language models.Language   `json:"language"`
}

type PatternRequest struct {
	Pattern string            `json:"pattern"`
	Path    string            `json:"path"`
	Files   []models.FileInfo `json:"files"`
}

type ApplyPlanRequest struct {
	SessionID string `json:"session_id"`
}

func (c *Client) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) Analyze(path string) (*AnalyzeResult, error) {
	req := AnalyzeRequest{
		Path: path,
	}

	var result AnalyzeResult
	if err := c.doRequest("POST", "/analyze", req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Deduplicate(path string) (*models.RefactorPlan, error) {
	req := DeduplicateRequest{
		Path:                path,
		SimilarityThreshold: 0.85,
	}

	var result models.RefactorPlan
	if err := c.doRequest("POST", "/deduplicate", req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Idiomatize(path string) (*models.RefactorPlan, error) {
	req := IdiomatizeRequest{
		Path: path,
	}

	var result models.RefactorPlan
	if err := c.doRequest("POST", "/idiomatize", req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) ApplyPattern(pattern, path string) (*models.RefactorPlan, error) {
	req := PatternRequest{
		Pattern: pattern,
		Path:    path,
	}

	var result models.RefactorPlan
	if err := c.doRequest("POST", "/pattern", req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) ApplyPlan(sessionID string) (*models.RefactorResult, error) {
	req := ApplyPlanRequest{
		SessionID: sessionID,
	}

	var result models.RefactorResult
	if err := c.doRequest("POST", "/apply", req, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Embed(files []models.FileInfo) (map[string][]float32, error) {
	req := map[string]any{
		"files": files,
	}

	var result map[string][]float32
	if err := c.doRequest("POST", "/embed", req, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) Shutdown() error {
	resp, err := c.httpClient.Post(c.baseURL+"/shutdown", "application/json", nil)
	if err != nil {
		return fmt.Errorf("shutdown request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) doRequest(method, endpoint string, reqBody any, result any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		body, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiResp APIResponse
		if err := json.Unmarshal(respBody, &apiResp); err == nil && apiResp.Error != "" {
			return fmt.Errorf("API error: %s", apiResp.Error)
		}
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Status == "error" {
		return fmt.Errorf("API error: %s", apiResp.Error)
	}

	if err := json.Unmarshal(apiResp.Data, result); err != nil {
		return fmt.Errorf("failed to parse response data: %w", err)
	}

	return nil
}
